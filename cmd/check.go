package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/mahm0udnasr/sadd/internal/dns"
	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check [domain]...",
	Short: "Check that sites are really blocked on this computer",
	Long: `Check asks the system resolver (the same one your browser uses) what
each site resolves to. A blocked site must resolve to 0.0.0.0 or ::, for
both the domain and www.domain. With no arguments, every site in sadd's
list is checked. The exit code is 1 if any site is not blocked.`,
	Example: `  sadd check
  sadd check facebook.com https://www.youtube.com/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := hosts.Path()
		return runCheck(os.Stdout, path, path == hosts.DefaultPath, args)
	},
}

// lookupHost is a variable so tests can fake the system resolver.
var lookupHost = dns.Lookup

// checkResult is what we found for one domain.
type checkResult struct {
	Domain string
	InList bool              // in sadd's section of the hosts file
	Leaks  map[string]string // name -> real addresses, for names NOT blocked
	Err    error             // the lookup itself failed
}

func (r checkResult) Blocked() bool { return r.Err == nil && len(r.Leaks) == 0 }

// runCheck checks domains (or the whole list). system says whether path is
// the hosts file the system really uses; only then is the resolver asked.
func runCheck(w io.Writer, path string, system bool, args []string) error {
	f, err := hosts.Load(path)
	if err != nil {
		return err
	}
	domains := f.Domains()
	inList := make(map[string]bool, len(domains))
	for _, d := range domains {
		inList[d] = true
	}

	if len(args) > 0 {
		if domains, err = normalizeAll(args); err != nil {
			return err
		}
	}
	if len(domains) == 0 {
		fmt.Fprintln(w, "No websites are blocked, so there is nothing to check.")
		return nil
	}

	// A HOSTS_FILE test file is not used by the system, so asking the
	// resolver would only be confusing. Report what the file says.
	if !system {
		fmt.Fprintf(w, "%s is not the system hosts file, so only the file is checked.\n", path)
		missing := 0
		for _, d := range domains {
			if inList[d] {
				fmt.Fprintf(w, "✓ %s  in the list\n", d)
			} else {
				fmt.Fprintf(w, "✗ %s  not in the list\n", d)
				missing++
			}
		}
		if missing > 0 {
			return fmt.Errorf("%s not in the list", plural(missing, "site"))
		}
		return nil
	}

	var failed []string
	width := 0
	for _, d := range domains {
		width = max(width, len(d))
	}
	for _, d := range domains {
		r := checkDomain(d, inList[d])
		fmt.Fprintf(w, "%s %-*s  %s\n", mark(r.Blocked()), width, d, explain(r))
		if !r.Blocked() {
			failed = append(failed, d)
		}
	}

	if len(failed) > 0 {
		fmt.Fprintln(w)
		printCheckHints(w, failed, inList)
		return fmt.Errorf("%s of %d not blocked", plural(len(failed), "site"), len(domains))
	}
	fmt.Fprintln(w, "\nAll blocked. If a site still loads in your browser:")
	fmt.Fprint(w, browserTips)
	return nil
}

// checkDomain looks up domain and www.domain. Both must resolve to a
// "nowhere" address (or not resolve at all) for the site to count as blocked.
func checkDomain(domain string, inList bool) checkResult {
	r := checkResult{Domain: domain, InList: inList, Leaks: map[string]string{}}
	for _, name := range []string{domain, "www." + domain} {
		ips, err := lookupHost(name)
		if err != nil {
			r.Err = err
			return r
		}
		var real []string
		for _, ip := range ips {
			if !isNowhere(ip) {
				real = append(real, ip.String())
			}
		}
		if len(real) > 0 {
			r.Leaks[name] = shortList(real, 2)
		}
	}
	return r
}

// shortList shows the first n items and how many more there are.
func shortList(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(items[:n], ", "), len(items)-n)
}

// isNowhere reports whether connecting to ip cannot reach the real site:
// 0.0.0.0, ::, or this computer (127.x, ::1).
func isNowhere(ip net.IP) bool { return ip.IsUnspecified() || ip.IsLoopback() }

func mark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func explain(r checkResult) string {
	switch {
	case r.Err != nil:
		return "could not check: " + r.Err.Error()
	case r.Blocked() && r.InList:
		return "blocked"
	case r.Blocked():
		return "blocked, but not by sadd (another hosts line or DNS filter)"
	}
	var parts []string
	for _, name := range []string{r.Domain, "www." + r.Domain} {
		if a, ok := r.Leaks[name]; ok {
			parts = append(parts, name+" → "+a)
		}
	}
	leaks := strings.Join(parts, "; ")
	if r.InList {
		return "in sadd's list, but the system still resolves " + leaks
	}
	return "not blocked (" + leaks + ")"
}

func printCheckHints(w io.Writer, failed []string, inList map[string]bool) {
	var notListed []string
	listedButLeaking := false
	for _, d := range failed {
		if inList[d] {
			listedButLeaking = true
		} else {
			notListed = append(notListed, d)
		}
	}
	if len(notListed) > 0 {
		fmt.Fprintf(w, "To block: sudo sadd block %s\n", strings.Join(notListed, " "))
	}
	if listedButLeaking {
		fmt.Fprint(w, `Listed but not working? Check:
  - an earlier line in /etc/hosts for the same name (the first match wins)
  - the "hosts:" line in /etc/nsswitch.conf must contain "files"
  - flush the cache: sudo resolvectl flush-caches
`)
	}
}

// browserTips explains why a browser can ignore a working block.
const browserTips = `  - Close the browser completely and open it again. Open tabs keep their
    connections, and browsers keep their own DNS cache.
  - Turn off "Secure DNS" / "DNS over HTTPS" in the browser settings: it
    asks a DNS server on the internet and skips /etc/hosts.
`

// blockHint is printed after a successful block of the real hosts file.
func blockHint(domains []string) string {
	return fmt.Sprintf("If it still loads, restart your browser. Confirm with: sadd check %s",
		strings.Join(domains[:min(len(domains), 3)], " "))
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
