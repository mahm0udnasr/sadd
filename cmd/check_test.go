package cmd

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeResolver answers from a map; names not in it do not resolve.
func fakeResolver(t *testing.T, answers map[string]string) {
	t.Helper()
	old := lookupHost
	lookupHost = func(name string) ([]net.IP, error) {
		var ips []net.IP
		for _, a := range strings.Fields(answers[name]) {
			ips = append(ips, net.ParseIP(a))
		}
		return ips, nil
	}
	t.Cleanup(func() { lookupHost = old })
}

func checkHostsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	content := "127.0.0.1 localhost\n# >>> sadd start\n0.0.0.0 facebook.com\n:: facebook.com\n0.0.0.0 www.facebook.com\n:: www.facebook.com\n# <<< sadd end\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckBlocked(t *testing.T) {
	fakeResolver(t, map[string]string{
		"facebook.com":     "0.0.0.0 ::",
		"www.facebook.com": "0.0.0.0 ::",
	})
	var out bytes.Buffer
	if err := runCheck(&out, checkHostsFile(t), true, nil); err != nil {
		t.Fatalf("err = %v\n%s", err, &out)
	}
	if !strings.Contains(out.String(), "✓ facebook.com  blocked") || !strings.Contains(out.String(), "Secure DNS") {
		t.Errorf("output:\n%s", &out)
	}
}

func TestCheckListedButLeaking(t *testing.T) {
	fakeResolver(t, map[string]string{
		"facebook.com":     "0.0.0.0",
		"www.facebook.com": "157.240.1.35",
	})
	var out bytes.Buffer
	err := runCheck(&out, checkHostsFile(t), true, []string{"https://www.facebook.com/home"})
	if err == nil || err.Error() != "1 site of 1 not blocked" {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"in sadd's list", "www.facebook.com → 157.240.1.35", "nsswitch"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, &out)
		}
	}
}

func TestCheckNotListed(t *testing.T) {
	fakeResolver(t, map[string]string{"youtube.com": "142.250.1.1"})
	var out bytes.Buffer
	err := runCheck(&out, checkHostsFile(t), true, []string{"youtube.com", "facebook.com"})
	if err == nil {
		t.Fatal("want an error")
	}
	// facebook.com does not resolve at all in the fake: that counts as blocked.
	for _, want := range []string{"✗ youtube.com", "not blocked (youtube.com → 142.250.1.1)", "✓ facebook.com", "sudo sadd block youtube.com"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, &out)
		}
	}
}

func TestCheckTestFileOnlyChecksFile(t *testing.T) {
	fakeResolver(t, nil)
	lookupHost = func(string) ([]net.IP, error) { t.Fatal("resolver must not be used"); return nil, nil }
	var out bytes.Buffer
	if err := runCheck(&out, checkHostsFile(t), false, nil); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(&out, checkHostsFile(t), false, []string{"x.com"}); err == nil {
		t.Error("x.com is not in the list: want an error")
	}
}

func TestCheckInvalidInput(t *testing.T) {
	if err := runCheck(&bytes.Buffer{}, checkHostsFile(t), true, []string{"not a domain"}); err == nil {
		t.Error("want an error for invalid input")
	}
}
