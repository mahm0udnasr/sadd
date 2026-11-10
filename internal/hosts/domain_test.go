package hosts

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	ok := []struct{ in, want string }{
		{"youtube.com", "youtube.com"},
		{"YouTube.COM", "youtube.com"},
		{"  youtube.com  ", "youtube.com"},
		{"www.youtube.com", "youtube.com"},
		{"https://www.YouTube.com/watch?v=abc", "youtube.com"},
		{"http://youtube.com:8080/", "youtube.com"},
		{"youtube.com/feed/trending", "youtube.com"},
		{"youtube.com.", "youtube.com"},
		{"https://user:pass@reddit.com", "reddit.com"},
		{"m.youtube.com", "m.youtube.com"}, // other subdomains are kept
		{"www.m.youtube.com", "m.youtube.com"},
		{"news.ycombinator.com", "news.ycombinator.com"},
		{"x.com", "x.com"},
		{"my-site.co.uk", "my-site.co.uk"},
	}
	for _, tt := range ok {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	bad := []string{
		"",
		"   ",
		"localhost",
		"youtube",      // no dot
		"192.168.1.1",  // IP
		"[::1]",        // IPv6
		"-bad.com",     // leading hyphen
		"bad..com",     // empty label
		"you_tube.com", // underscore
		"يوتيوب.com",   // non-ASCII
		"https://",     // no host
	}
	for _, in := range bad {
		t.Run("bad/"+in, func(t *testing.T) {
			got, err := Normalize(in)
			if !errors.Is(err, ErrInvalidDomain) {
				t.Errorf("Normalize(%q) = %q, %v; want ErrInvalidDomain", in, got, err)
			}
		})
	}
}

func TestHostLines(t *testing.T) {
	want := []string{
		"0.0.0.0 youtube.com",
		":: youtube.com",
		"0.0.0.0 www.youtube.com",
		":: www.youtube.com",
	}
	if got := HostLines("youtube.com"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDomains(t *testing.T) {
	f := File{Block: []string{
		"0.0.0.0 youtube.com",
		":: youtube.com",
		"0.0.0.0 www.youtube.com",
		"",
		"# a comment",
		"0.0.0.0 reddit.com",
		":: www.reddit.com",
	}}
	want := []string{"youtube.com", "reddit.com"}
	if got := f.Domains(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
