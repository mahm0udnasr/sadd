package hosts

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddRemove(t *testing.T) {
	var f File

	added := f.Add("youtube.com", "reddit.com", "youtube.com")
	if want := []string{"youtube.com", "reddit.com"}; !reflect.DeepEqual(added, want) {
		t.Fatalf("Add returned %q, want %q", added, want)
	}
	if len(f.Block) != 8 {
		t.Fatalf("block = %q, want 8 lines", f.Block)
	}

	if again := f.Add("youtube.com"); again != nil {
		t.Errorf("adding an existing domain returned %q, want nil", again)
	}

	removed := f.Remove("youtube.com", "not-there.com")
	if want := []string{"youtube.com"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("Remove returned %q, want %q", removed, want)
	}
	if got, want := f.Domains(), []string{"reddit.com"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Domains after remove = %q, want %q", got, want)
	}
}

// The most important property: lines outside the block never change,
// and Parse(Render(f)) gives back the same File.
func TestRenderRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hosts"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if got := f.Render(); got != string(data) {
		t.Errorf("unchanged file did not render identically:\n%s", got)
	}

	f.Add("reddit.com")
	again, err := Parse(f.Render())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.Before, f.Before) || !reflect.DeepEqual(again.After, f.After) {
		t.Error("Before/After changed after adding a domain")
	}
	if want := []string{"youtube.com", "reddit.com"}; !reflect.DeepEqual(again.Domains(), want) {
		t.Errorf("domains = %q, want %q", again.Domains(), want)
	}
}

func TestRenderFirstRun(t *testing.T) {
	f, _ := Parse("127.0.0.1 localhost\n")
	f.Add("x.com")
	want := "127.0.0.1 localhost\n\n" + StartMarker + "\n" +
		strings.Join(HostLines("x.com"), "\n") + "\n" + EndMarker + "\n"
	if got := f.Render(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	original := "127.0.0.1 localhost\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Add("youtube.com")
	if err := Save(path, f); err != nil {
		t.Fatalf("Save: %v", err)
	}

	backup, err := os.ReadFile(BackupPath(path))
	if err != nil || string(backup) != original {
		t.Errorf("backup = %q, %v; want %q", backup, err, original)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Domains(); !reflect.DeepEqual(got, []string{"youtube.com"}) {
		t.Errorf("domains after save = %q", got)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("permissions changed to %v", info.Mode().Perm())
	}
}
