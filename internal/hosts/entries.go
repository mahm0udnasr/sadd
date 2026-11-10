package hosts

import (
	"strings"
	"time"
)

// untilTag starts the comment line that stores a timed block's end time:
//
//	# until 2026-11-11T01:00:00+02:00 youtube.com
//
// The block belongs to sadd, so it is a safe place to keep this state:
// no database needed, and the file stays the single source of truth.
const untilTag = "# until "

// Entry is one blocked domain. A zero Until means "blocked forever".
type Entry struct {
	Domain string
	Until  time.Time
}

// Timed reports whether the entry expires.
func (e Entry) Timed() bool { return !e.Until.IsZero() }

// Expired reports whether a timed entry's end time has passed.
func (e Entry) Expired(now time.Time) bool { return e.Timed() && !e.Until.After(now) }

// Entries returns the blocked domains in file order, without duplicates and
// without the "www." copies, each with its end time if it has one.
func (f File) Entries() []Entry {
	until := make(map[string]time.Time)
	seen := make(map[string]bool)
	var out []Entry

	for _, line := range f.Block {
		trimmed := strings.TrimSpace(line)

		if rest, ok := strings.CutPrefix(trimmed, untilTag); ok {
			// rest = "<RFC3339 time> <domain>"
			fields := strings.Fields(rest)
			if len(fields) == 2 {
				if t, err := time.Parse(time.RFC3339, fields[0]); err == nil {
					until[fields[1]] = t
				}
			}
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue // empty line or other comment
		}
		d := strings.TrimPrefix(fields[1], "www.")
		if !seen[d] {
			seen[d] = true
			out = append(out, Entry{Domain: d})
		}
	}

	// Attach end times last: the comment comes before its host lines.
	for i := range out {
		out[i].Until = until[out[i].Domain]
	}
	return out
}

// Domains returns just the domain names of Entries.
func (f File) Domains() []string {
	entries := f.Entries()
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Domain
	}
	return out
}

// Add blocks domains forever. See AddUntil.
func (f *File) Add(domains ...string) []string {
	return f.AddUntil(time.Time{}, domains...)
}

// AddUntil blocks domains until the given time (zero time = forever).
// The newest command wins: blocking an already-blocked domain replaces its
// end time. It returns the domains that are new or whose end time changed.
func (f *File) AddUntil(until time.Time, domains ...string) []string {
	entries := f.Entries()
	index := make(map[string]int, len(entries))
	for i, e := range entries {
		index[e.Domain] = i
	}

	var changed []string
	for _, d := range domains {
		i, exists := index[d]
		switch {
		case !exists:
			index[d] = len(entries)
			entries = append(entries, Entry{Domain: d, Until: until})
			changed = append(changed, d)
		case !entries[i].Until.Equal(until):
			entries[i].Until = until
			changed = append(changed, d)
		}
	}
	if len(changed) > 0 {
		f.setEntries(entries)
	}
	return changed
}

// Remove unblocks domains. It returns those that were actually blocked.
func (f *File) Remove(domains ...string) []string {
	drop := toSet(domains)
	return f.removeIf(func(e Entry) bool { return drop[e.Domain] })
}

// Expire removes timed entries whose end time is at or before now.
func (f *File) Expire(now time.Time) []string {
	return f.removeIf(func(e Entry) bool { return e.Expired(now) })
}

func (f *File) removeIf(match func(Entry) bool) []string {
	var kept []Entry
	var removed []string
	for _, e := range f.Entries() {
		if match(e) {
			removed = append(removed, e.Domain)
		} else {
			kept = append(kept, e)
		}
	}
	if len(removed) > 0 {
		f.setEntries(kept)
	}
	return removed
}

// setEntries rebuilds the whole block from entries.
// Do NOT set f.HasBlock here: it records whether the file had markers
// when it was read, and Render needs that to format a first-run block.
func (f *File) setEntries(entries []Entry) {
	block := make([]string, 0, len(entries)*5)
	for _, e := range entries {
		if e.Timed() {
			block = append(block, untilTag+e.Until.Format(time.RFC3339)+" "+e.Domain)
		}
		block = append(block, HostLines(e.Domain)...)
	}
	f.Block = block
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, s := range items {
		set[s] = true
	}
	return set
}
