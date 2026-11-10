package groups

import (
	"testing"

	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// Every built-in domain must survive Normalize unchanged, otherwise
// "unblock --group" could miss what "block --group" added.
func TestBuiltInDomainsAreNormalized(t *testing.T) {
	for _, name := range Names() {
		domains, _ := Get(name)
		for _, d := range domains {
			got, err := hosts.Normalize(d)
			if err != nil || got != d {
				t.Errorf("group %s: %q normalizes to %q, %v", name, d, got, err)
			}
		}
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := Get("nope"); err == nil {
		t.Error("expected an error for an unknown group")
	}
}

func TestGetReturnsCopy(t *testing.T) {
	a, _ := Get("social")
	a[0] = "changed"
	b, _ := Get("social")
	if b[0] == "changed" {
		t.Error("Get exposes the internal slice")
	}
}
