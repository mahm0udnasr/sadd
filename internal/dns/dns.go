// Package dns clears the system DNS cache so hosts changes apply at once.
package dns

import (
	"fmt"
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
