package hosts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ErrNoBackup means restore was asked for but no .bak file exists yet.
var ErrNoBackup = errors.New("no backup found")

// BackupPath is where Save copies the old file before writing.
func BackupPath(path string) string {
	return path + ".bak"
}

// Render joins the three parts back into file content.
// Before and After are written exactly as they were read.
func (f File) Render() string {
	lines := make([]string, 0, len(f.Before)+len(f.Block)+len(f.After)+3)
	lines = append(lines, f.Before...)

	// First run: separate our new block from the user's lines with a blank line.
	if !f.HasBlock && len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}

	lines = append(lines, StartMarker)
	lines = append(lines, f.Block...)
	lines = append(lines, EndMarker)
	lines = append(lines, f.After...)

	return strings.Join(lines, "\n") + "\n"
}

// Save backs up the current file to path.bak, then writes f to path.
// Both writes are atomic, and the file keeps its original permissions.
//
// Order matters: if writing the hosts file fails, the backup already holds
// the current content, so nothing is lost.
func Save(path string, f File) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat hosts file: %w", err)
	}
	perm := info.Mode().Perm()

	old, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts file for backup: %w", err)
	}
	if err := writeAtomic(BackupPath(path), old, perm); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	if err := writeAtomic(path, []byte(f.Render()), perm); err != nil {
		return fmt.Errorf("write hosts file: %w", err)
	}
	return nil
}

// Restore swaps the hosts file and its backup: the backup becomes the
// hosts file, and the current hosts file becomes the new backup.
// So running restore twice gets you back where you started (undo/redo).
func Restore(path string) error {
	bakPath := BackupPath(path)

	backup, err := os.ReadFile(bakPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNoBackup, bakPath)
		}
		return fmt.Errorf("read backup: %w", err)
	}
	// Refuse to restore a backup whose markers are broken; a later block
	// command would fail on it anyway.
	if _, err := Parse(string(backup)); err != nil {
		return fmt.Errorf("backup %s looks damaged, not restoring: %w", bakPath, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat hosts file: %w", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}

	// Hosts file first: that is what the user asked for. Both contents are
	// in memory, so a failure in the second write loses nothing on screen.
	if err := writeAtomic(path, backup, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write hosts file: %w", err)
	}
	if err := writeAtomic(bakPath, current, info.Mode().Perm()); err != nil {
		return fmt.Errorf("restored, but could not save the previous state as backup: %w", err)
	}
	return nil
}
