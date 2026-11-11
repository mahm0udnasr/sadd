package blocklist

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// filler returns n valid blocking lines, to get past minDomains.
func filler(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "0.0.0.0 filler%d.com\n", i)
	}
	return b.String()
}

func TestParseKeepsOnlySafeNames(t *testing.T) {
	in := `# Title: test
127.0.0.1 localhost
::1 ip6-localhost
0.0.0.0 0.0.0.0
0.0.0.0 Bad-Site.com.   # comment after
0.0.0.0 www.bad-site.com
0.0.0.0 bad-site.com
6.6.6.6 mybank.com
0.0.0.0 not_valid!.com
0.0.0.0 two.com three.com
` + filler(minDomains)

	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bad-site.com", "www.bad-site.com", "two.com", "three.com"}
	if !slices.Equal(got[:4], want) {
		t.Errorf("got %v, want %v first", got[:4], want)
	}
	for _, bad := range []string{"mybank.com", "localhost", "0.0.0.0", "not_valid!.com"} {
		if slices.Contains(got, bad) {
			t.Errorf("%q must be skipped", bad)
		}
	}
}

func TestParseRejectsNonList(t *testing.T) {
	if _, err := Parse(strings.NewReader("<html>404 Not Found</html>")); err == nil {
		t.Error("an HTML page was accepted")
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, filler(minDomains))
	}))
	defer srv.Close()

	got, err := Download(context.Background(), Source{Name: "t", URL: srv.URL + "/hosts"})
	if err != nil || len(got) != minDomains {
		t.Fatalf("got %d domains, %v", len(got), err)
	}
	if _, err := Download(context.Background(), Source{Name: "t", URL: srv.URL + "/missing"}); err == nil {
		t.Error("a 404 was accepted")
	}
}

func TestGet(t *testing.T) {
	if s, err := Get("porn"); err != nil || !strings.HasPrefix(s.URL, "https://") {
		t.Errorf("porn = %+v, %v", s, err)
	}
	if _, err := Get("nope"); err == nil {
		t.Error("unknown list accepted")
	}
}
