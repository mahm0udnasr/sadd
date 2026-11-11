// Package blocklist downloads big ready-made lists of domains (like the
// StevenBlack porn list) and turns them into a clean list of domains.
//
// SAFETY: the downloaded file comes from the internet but ends up in a
// root-owned /etc/hosts. A hosts file can REDIRECT a name, not only block
// it ("6.6.6.6 mybank.com" would send your bank to someone else). So we
// never copy lines from the download. We take only the names, check each
// one, and write the 0.0.0.0 / :: lines ourselves (hosts.SetList).
package blocklist

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// Source is one list we know how to download.
type Source struct {
	Name  string // used on the command line: "sadd blocklist on porn"
	Title string // shown in the TUI
	URL   string
	About string // one line for help and dialogs
}

// sources is the menu. To add a list, add a line here (StevenBlack also
// has gambling, fakenews and social lists in the same format).
// It is a variable so tests can point a source at a local test server.
var sources = []Source{
	{
		Name:  "porn",
		Title: "Porn blocker",
		URL:   "https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/porn-only/hosts",
		About: "about 77,000 adult sites, from github.com/StevenBlack/hosts",
	},
}

// All returns every known source, in menu order.
func All() []Source { return slices.Clone(sources) }

// Get finds a source by name.
func Get(name string) (Source, error) {
	for _, s := range sources {
		if s.Name == name {
			return s, nil
		}
	}
	var names []string
	for _, s := range sources {
		names = append(names, s.Name)
	}
	return Source{}, fmt.Errorf("unknown blocklist %q (available: %s)", name, strings.Join(names, ", "))
}

// SetURLForTest points a source at another URL and returns a function
// that undoes it. Only tests should call it.
func SetURLForTest(name, url string) (restore func()) {
	old := slices.Clone(sources)
	for i := range sources {
		if sources[i].Name == name {
			sources[i].URL = url
		}
	}
	return func() { sources = old }
}

const (
	maxBytes   = 64 << 20 // 64 MB: far above the real size (2 MB)
	minDomains = 100      // fewer means we got an error page, not a list
	timeout    = 2 * time.Minute
)

// Download fetches a source and returns its checked domains.
func Download(ctx context.Context, src Source) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sadd")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download the %s blocklist: %w", src.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download the %s blocklist: server answered %s", src.Name, resp.Status)
	}

	domains, err := Parse(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("the %s blocklist: %w", src.Name, err)
	}
	return domains, nil
}

// notSites are names hosts files list for the machine itself.
var notSites = map[string]bool{
	"localhost": true, "localhost.localdomain": true, "local": true,
	"broadcasthost": true, "ip6-localhost": true, "ip6-loopback": true,
	"0.0.0.0": true,
}

// blockingIPs are the addresses a BLOCKING line may use. A line with any
// other address is a redirect, and is skipped completely.
var blockingIPs = map[string]bool{"0.0.0.0": true, "127.0.0.1": true, "::": true, "::1": true}

// Parse reads a hosts-format list and returns its domains: lowercase,
// valid, without duplicates, in file order.
func Parse(r io.Reader) ([]string, error) {
	var out []string
	seen := map[string]bool{}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#") // drop comments
		fields := strings.Fields(line)
		if len(fields) < 2 || !blockingIPs[fields[0]] {
			continue
		}
		for _, name := range fields[1:] {
			name = strings.TrimSuffix(strings.ToLower(name), ".")
			if notSites[name] || seen[name] || hosts.ValidDomain(name) != nil {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) < minDomains {
		return nil, errors.New("the download does not look like a blocklist (too few domains)")
	}
	return out, nil
}
