package cmd

import (
	"testing"

	"github.com/mahm0udnasr/sadd/internal/hosts"
)

func TestRequireRoot(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		euid    int
		wantErr bool
	}{
		{"root on real file", hosts.DefaultPath, 0, false},
		{"user on real file", hosts.DefaultPath, 1000, true},
		{"user on test file", "/tmp/fake-hosts", 1000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireRoot(tt.path, tt.euid)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"youtube.com":                   "youtube.com",
		"https://youtube.com/watch":     "https://youtube.com/watch",
		"https://youtube.com/watch?v=1": "'https://youtube.com/watch?v=1'",
		"a&b":                           "'a&b'",
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
