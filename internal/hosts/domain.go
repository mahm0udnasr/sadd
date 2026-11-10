package hosts

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrInvalidDomain is returned (wrapped) for anything Normalize rejects.
var ErrInvalidDomain = errors.New("invalid domain")

// Normalize turns user input into a bare, lowercase domain:
//
//	"https://www.YouTube.com/watch?v=x" -> "youtube.com"
//	"m.youtube.com:443"                 -> "m.youtube.com"
//
// Only a leading "www." is removed; other subdomains are kept on purpose,
// because /etc/hosts has no wildcards and the user must be explicit.
func Normalize(input string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", fmt.Errorf("%w: empty input", ErrInvalidDomain)
	}

	// Let net/url do the hard work (scheme, user@, port, path, query).
	// It needs a scheme to recognise the host, so add one if missing.
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrInvalidDomain, input, err)
	}
	host := u.Hostname() // drops the port and IPv6 brackets

	host = strings.TrimSuffix(host, ".") // "youtube.com." is valid DNS, same site
	host = strings.TrimPrefix(host, "www.")

	if err := validate(host); err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrInvalidDomain, input, err)
	}
	return host, nil
}

// validate checks the basic DNS rules for a hostname.
func validate(host string) error {
	switch {
	case host == "":
		return errors.New("no host found")
	case net.ParseIP(host) != nil:
		return errors.New("IP addresses cannot be blocked via /etc/hosts")
	case host == "localhost":
		return errors.New("refusing to block localhost")
	case !strings.Contains(host, "."):
		return errors.New("domain needs a dot, e.g. example.com")
	case len(host) > 253:
		return errors.New("domain longer than 253 characters")
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("bad label %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("label %q starts or ends with '-'", label)
		}
		for _, r := range label {
			isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			if !isAlnum && r != '-' {
				return fmt.Errorf("character %q not allowed (non-English domains must be in punycode, xn--...)", r)
			}
		}
	}
	return nil
}

// HostLines returns the four hosts lines that block one domain.
// IPv6 (::) is needed too, otherwise browsers may simply use IPv6.
func HostLines(domain string) []string {
	return []string{
		"0.0.0.0 " + domain,
		":: " + domain,
		"0.0.0.0 www." + domain,
		":: www." + domain,
	}
}
