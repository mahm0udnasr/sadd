package hosts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeAtomic(path, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new\n" {
		t.Errorf("content = %q, want %q", got, "new\n")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("perm = %v, want 0644 (CreateTemp uses 0600)", info.Mode().Perm())
	}

	// No leftover temp files.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (temp file left behind?)", len(entries))
	}
}

func TestWriteAtomicKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-hosts")
	link := filepath.Join(dir, "hosts")
	os.WriteFile(real, []byte("old\n"), 0o644)
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	if err := writeAtomic(link, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Lstat(link)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(real); string(got) != "new\n" {
		t.Errorf("target content = %q", got)
	}
}

func TestWriteAtomicFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	// The target is a directory, so the final rename must fail.
	target := filepath.Join(dir, "target")
	os.Mkdir(target, 0o755)
	os.WriteFile(filepath.Join(target, "x"), nil, 0o644) // non-empty: rename cannot replace it

	if err := writeAtomic(target, []byte("data"), 0o644); err == nil {
		t.Fatal("expected an error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (temp file left behind)", len(entries))
	}
}

func TestRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	os.WriteFile(path, []byte("127.0.0.1 localhost\n"), 0o644)

	f, _ := Load(path)
	f.Add("youtube.com")
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}

	// restore -> youtube is gone
	if err := Restore(path); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if f, _ := Load(path); len(f.Domains()) != 0 {
		t.Errorf("after restore domains = %q, want none", f.Domains())
	}

	// restore again -> youtube is back (undo the restore)
	if err := Restore(path); err != nil {
		t.Fatal(err)
	}
	if f, _ := Load(path); len(f.Domains()) != 1 {
		t.Errorf("after second restore domains = %q, want [youtube.com]", f.Domains())
	}
}

func TestRestoreNoBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	os.WriteFile(path, []byte("x\n"), 0o644)
	if err := Restore(path); !errors.Is(err, ErrNoBackup) {
		t.Errorf("err = %v, want ErrNoBackup", err)
	}
}

func TestRestoreDamagedBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	os.WriteFile(path, []byte("good\n"), 0o644)
	os.WriteFile(BackupPath(path), []byte(StartMarker+"\n"), 0o644) // no end marker

	if err := Restore(path); !errors.Is(err, ErrMissingEnd) {
		t.Errorf("err = %v, want ErrMissingEnd", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "good\n" {
		t.Error("hosts file changed even though restore was refused")
	}
}
