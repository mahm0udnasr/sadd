package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/mahm0udnasr/sadd/internal/dns"
	"github.com/mahm0udnasr/sadd/internal/groups"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// normalizeAll normalizes every argument. If any is invalid it returns all
// the problems together and nothing gets written (all or nothing).
func normalizeAll(args []string) ([]string, error) {
	var (
		domains []string
		errs    []error
	)
	for _, a := range args {
		d, err := hosts.Normalize(a)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		domains = append(domains, d)
	}
	return domains, errors.Join(errs...)
}

// collectDomains merges domain arguments and --group names into one
// normalized list without duplicates.
func collectDomains(args, groupNames []string) ([]string, error) {
	if len(args) == 0 && len(groupNames) == 0 {
		return nil, errors.New("give at least one domain or --group (see 'sadd groups')")
	}

	domains, err := normalizeAll(args)
	if err != nil {
		return nil, err
	}
	for _, g := range groupNames {
		list, err := groups.Get(g)
		if err != nil {
			return nil, err
		}
		domains = append(domains, list...)
	}

	seen := make(map[string]bool, len(domains))
	unique := domains[:0] // filter in place, reusing the same array
	for _, d := range domains {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}
	return unique, nil
}

// result is what update did. Warning is set when the file was saved but
// something non-fatal failed (e.g. the DNS flush).
type result struct {
	Changed []string
	Expired []string
	Warning error
}

// update is the one place that changes the hosts file:
// load -> drop expired timed blocks -> apply change -> save (only if needed).
// Both the CLI commands and the TUI use it.
func update(change func(f *hosts.File) []string) (result, error) {
	path := hosts.Path()
	f, err := hosts.Load(path)
	if err != nil {
		return result{}, err
	}

	var r result
	r.Expired = f.Expire(time.Now())
	if change != nil {
		r.Changed = change(&f)
	}
	if len(r.Changed) == 0 && len(r.Expired) == 0 {
		return r, nil // nothing to write: no backup, no root needed
	}

	if err := requireRoot(path, os.Geteuid()); err != nil {
		return result{}, err
	}
	if err := hosts.Save(path, f); err != nil {
		return result{}, friendlyPermError(path, err)
	}
	r.Warning = flushDNS(path)
	return r, nil
}

// requireRoot returns a friendly error when the real /etc/hosts would be
// changed without root. For a HOSTS_FILE test file root is not needed;
// normal file permissions decide (see friendlyPermError).
// euid is a parameter (not os.Geteuid() inside) so tests can fake it.
func requireRoot(path string, euid int) error {
	if euid == 0 || path != hosts.DefaultPath {
		return nil
	}
	return fmt.Errorf("changing %s needs root. Run it with sudo:\n  sudo %s",
		path, commandLine())
}

// friendlyPermError adds a hint to "permission denied" errors.
func friendlyPermError(path string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w\nhint: you need write access to %s and its folder", err, path)
	}
	return err
}

// commandLine rebuilds what the user typed, for copy-paste hints.
func commandLine() string {
	s := "sadd"
	for _, a := range os.Args[1:] {
		s += " " + shellQuote(a)
	}
	return s
}

// shellQuote wraps an argument in single quotes if it has special characters
// (URLs often contain ? and &, which the shell would interpret).
func shellQuote(s string) string {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' || r == '/' || r == ':') {
			return "'" + s + "'"
		}
	}
	return s
}

// flushDNS clears the system DNS cache, but only for the real hosts file:
// for a HOSTS_FILE test file the system cache is irrelevant.
// It returns the error instead of printing it, so the TUI can show it
// without breaking its screen.
func flushDNS(path string) error {
	if path != hosts.DefaultPath {
		return nil
	}
	if err := dns.Flush(); err != nil {
		return fmt.Errorf("could not flush DNS cache: %w", err)
	}
	return nil
}

func printWarning(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
}

func printList(label string, items []string) {
	for _, s := range items {
		fmt.Printf("%s %s\n", label, s)
	}
}
