package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/tui"
)

// tempHosts points HOSTS_FILE at a fresh file for this test only.
func tempHosts(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(hosts.PathEnv, path)
	return path
}

func domainsIn(t *testing.T, path string) []string {
	t.Helper()
	f, err := hosts.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f.Domains()
}

func TestTUIBackendBlockUnblock(t *testing.T) {
	path := tempHosts(t)

	msg, err := tuiBlock(tui.BlockRequest{Domains: []string{"youtube.com"}})
	if err != nil || msg != "Blocked youtube.com." {
		t.Fatalf("block: %q, %v", msg, err)
	}
	msg, _ = tuiBlock(tui.BlockRequest{Domains: []string{"youtube.com"}})
	if msg != "Already blocked." {
		t.Errorf("second block: %q", msg)
	}

	msg, err = tuiBlock(tui.BlockRequest{Domains: []string{"news.ycombinator.com", "bbc.com", "cnn.com", "nytimes.com"}})
	if err != nil || !strings.HasPrefix(msg, "Blocked 4 sites") {
		t.Fatalf("several: %q, %v", msg, err)
	}

	if msg, err := tuiUnblock([]string{"youtube.com", "not-blocked.com"}); err != nil || msg != "Unblocked youtube.com." {
		t.Fatalf("unblock: %q, %v", msg, err)
	}
	want := []string{"news.ycombinator.com", "bbc.com", "cnn.com", "nytimes.com"}
	if got := domainsIn(t, path); !reflect.DeepEqual(got, want) {
		t.Errorf("domains = %q, want %q", got, want)
	}
}

func TestCanWrite(t *testing.T) {
	path := tempHosts(t)
	if !canWrite(path) {
		t.Error("own temp file should be writable")
	}

	// Read-only folder: atomic rename would fail, so canWrite must say no.
	dir := filepath.Dir(path)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if os.Geteuid() != 0 && canWrite(path) {
		t.Error("read-only folder reported as writable")
	}

	if os.Geteuid() != 0 && canWrite(hosts.DefaultPath) {
		t.Error("/etc/hosts reported writable without root")
	}
}
