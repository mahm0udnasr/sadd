package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// writeAtomic replaces path with data so that readers see either the old
// file or the new one, never a half-written file.
//
// How: write a temp file in the SAME directory, flush it to disk, then
// rename it over the target. rename(2) is atomic only within one
// filesystem, which is why the temp file must live next to the target.
//
// The new file keeps perm and, when possible, the owner of the old one.
func writeAtomic(path string, data []byte, perm os.FileMode) (err error) {
	// If path is a symlink, write to the real file. Renaming over the link
	// itself would replace the link with a regular file.
	if real, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
		path = real
	}
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".sadd-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// On any failure below, remove the temp file so no junk is left in /etc.
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	// CreateTemp always uses 0600; set the permissions we actually want.
	if err = tmp.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err = copyOwner(path, tmp); err != nil {
		return err
	}
	// Sync forces the data onto the disk before the rename, so a power cut
	// cannot leave an empty file behind.
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}

	// Best effort: persist the rename itself (the directory entry).
	if d, dirErr := os.Open(dir); dirErr == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// copyOwner gives tmp the same uid/gid as the existing file at path.
// If path does not exist yet (e.g. the first backup), there is nothing to copy.
func copyOwner(path string, tmp *os.File) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := tmp.Chown(int(st.Uid), int(st.Gid)); err != nil {
		return fmt.Errorf("chown temp file: %w", err)
	}
	return nil
}
