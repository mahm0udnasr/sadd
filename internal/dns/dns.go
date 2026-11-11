// Package dns talks to the system resolver: it clears the DNS cache so
// hosts changes apply at once, and looks names up the way other programs do.
package dns

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// Flush runs "resolvectl flush-caches" (systemd-resolved, default on Ubuntu).
func Flush() error {
	out, err := exec.Command("resolvectl", "flush-caches").CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolvectl flush-caches: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Lookup returns the addresses the system gives for name, using
// "getent ahosts". getent goes through glibc and /etc/nsswitch.conf, the same
// path browsers and most programs use, so it shows what they will see.
// (Go's own resolver reads /etc/hosts itself and can disagree with glibc.)
//
// A name that does not resolve at all returns no addresses and no error.
func Lookup(name string) ([]net.IP, error) {
	out, err := exec.Command("getent", "ahosts", name).Output()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 2:
		return nil, nil // exit code 2 = "key not found": the name does not resolve
	case errors.Is(err, exec.ErrNotFound):
		return lookupGo(name) // no getent (unusual): fall back to Go's resolver
	case err != nil:
		return nil, fmt.Errorf("getent ahosts %s: %w", name, err)
	}
	return parseGetent(out), nil
}

// parseGetent reads lines like "0.0.0.0  STREAM facebook.com" and returns
// each distinct address once, in order.
func parseGetent(out []byte) []net.IP {
	var ips []net.IP
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || seen[fields[0]] {
			continue
		}
		if ip := net.ParseIP(fields[0]); ip != nil {
			seen[fields[0]] = true
			ips = append(ips, ip)
		}
	}
	return ips
}

func lookupGo(name string) ([]net.IP, error) {
	ips, err := net.LookupIP(name)
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return nil, nil
	}
	return ips, err
}
