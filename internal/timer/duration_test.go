package timer

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	ok := map[string]time.Duration{
		"2h":    2 * time.Hour,
		"90m":   90 * time.Minute,
		"1h30m": 90 * time.Minute,
		"1d":    24 * time.Hour,
		" 3d ":  72 * time.Hour,
	}
	for in, want := range ok {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}

	for _, in := range []string{"", "2", "abc", "30s", "-1h", "xd", "0d"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) should fail", in)
		}
	}
}
