// Package groups holds named lists of domains, e.g. "social".
package groups

import (
	"fmt"
	"slices"
	"sort"
)

// builtIn lists common sites. Domains here must already be normalized
// (lowercase, no "www."); a test checks this.
// Subdomains like m.facebook.com are listed on purpose: /etc/hosts has
// no wildcards, so each one must be blocked explicitly.
var builtIn = map[string][]string{
	"social": {
		"facebook.com", "m.facebook.com", "instagram.com", "threads.net",
		"x.com", "twitter.com", "tiktok.com", "snapchat.com",
		"reddit.com", "old.reddit.com", "linkedin.com",
	},
	"video": {
		"youtube.com", "m.youtube.com", "youtu.be",
		"netflix.com", "twitch.tv", "vimeo.com",
	},
	"news": {
		"news.ycombinator.com", "bbc.com", "cnn.com", "nytimes.com",
	},
	"shopping": {
		"amazon.com", "amazon.eg", "aliexpress.com", "ebay.com", "noon.com",
	},
}

// Get returns a copy of the group's domains.
func Get(name string) ([]string, error) {
	domains, ok := builtIn[name]
	if !ok {
		return nil, fmt.Errorf("unknown group %q (available: %v)", name, Names())
	}
	// Return a copy so callers cannot modify the shared map's slice.
	return slices.Clone(domains), nil
}

// Names returns all group names, sorted (map order in Go is random).
func Names() []string {
	names := make([]string, 0, len(builtIn))
	for n := range builtIn {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
