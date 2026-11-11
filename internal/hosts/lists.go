package hosts

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// A blocklist is a big downloaded list of domains (e.g. 77,000 porn
// sites). It gets its own marked section, separate from the normal block,
// so it shows as ONE thing ("porn: on") instead of 77,000 entries:
//
//	# >>> sadd blocklist porn
//	# downloaded 2026-11-11T20:00:00+02:00 from https://...
//	# off-at 2026-11-11T20:10:00+02:00      <- only while turning off
//	0.0.0.0 example.com
//	:: example.com
//	...
//	# <<< sadd blocklist porn
//
// Like timed blocks, all state lives in the file itself.

const (
	listStartPrefix = "# >>> sadd blocklist "
	listEndPrefix   = "# <<< sadd blocklist "
	downloadedTag   = "# downloaded "
	offAtTag        = "# off-at "
)

// ErrBadList is returned (wrapped) for a damaged blocklist section.
var ErrBadList = errors.New("damaged blocklist section")

// List is one blocklist section: its name and the raw lines inside.
type List struct {
	Name  string
	Lines []string
}

// ListInfo describes a blocklist that is on.
type ListInfo struct {
	Name       string
	Source     string    // URL it was downloaded from
	Downloaded time.Time // when
	Domains    int
	OffAt      time.Time // zero = staying on; else it turns off then
}

// TurningOff reports whether the list is waiting to be removed.
func (i ListInfo) TurningOff() bool { return !i.OffAt.IsZero() }

// extractLists pulls every blocklist section out of lines. What is left
// is parsed as before, so the normal block works exactly as it did.
func extractLists(lines []string) (rest []string, lists []List, err error) {
	seen := map[string]bool{}
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, listEndPrefix) {
			return nil, nil, fmt.Errorf("line %d: %w: end marker without start", i+1, ErrBadList)
		}
		name, ok := strings.CutPrefix(t, listStartPrefix)
		if !ok {
			rest = append(rest, lines[i])
			continue
		}
		if seen[name] {
			return nil, nil, fmt.Errorf("line %d: %w: %q appears twice", i+1, ErrBadList, name)
		}
		seen[name] = true

		end := -1
		for j := i + 1; j < len(lines); j++ {
			tj := strings.TrimSpace(lines[j])
			if tj == listEndPrefix+name {
				end = j
				break
			}
			if strings.HasPrefix(tj, listStartPrefix) || strings.HasPrefix(tj, listEndPrefix) {
				break // another section starts first: this one is not closed
			}
		}
		if end == -1 {
			return nil, nil, fmt.Errorf("line %d: %w: %q has no end marker", i+1, ErrBadList, name)
		}
		lists = append(lists, List{Name: name, Lines: lines[i+1 : end]})
		i = end
	}
	return rest, lists, nil
}

// renderLists writes the sections back, each between its markers.
func renderLists(lists []List) []string {
	var out []string
	for _, l := range lists {
		out = append(out, listStartPrefix+l.Name)
		out = append(out, l.Lines...)
		out = append(out, listEndPrefix+l.Name)
	}
	return out
}

// Lists describes every blocklist that is on, in file order.
func (f File) Lists() []ListInfo {
	out := make([]ListInfo, 0, len(f.lists))
	for _, l := range f.lists {
		out = append(out, l.info())
	}
	return out
}

// List describes one blocklist, if it is on.
func (f File) List(name string) (ListInfo, bool) {
	if i := f.listIndex(name); i >= 0 {
		return f.lists[i].info(), true
	}
	return ListInfo{}, false
}

func (l List) info() ListInfo {
	info := ListInfo{Name: l.Name}
	for _, line := range l.Lines {
		switch {
		case strings.HasPrefix(line, "0.0.0.0 "):
			info.Domains++ // each domain has one IPv4 line (and one IPv6)
		case strings.HasPrefix(line, downloadedTag):
			// "# downloaded <time> from <url>"
			f := strings.Fields(strings.TrimPrefix(line, downloadedTag))
			if len(f) == 3 && f[1] == "from" {
				info.Downloaded, _ = time.Parse(time.RFC3339, f[0])
				info.Source = f[2]
			}
		case strings.HasPrefix(line, offAtTag):
			info.OffAt, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line, offAtTag))
		}
	}
	return info
}

// SetList turns a blocklist on with these domains, replacing an older
// copy. It also cancels a pending turn-off. Domains must already be
// validated (see package blocklist).
func (f *File) SetList(name, source string, downloaded time.Time, domains []string) {
	lines := make([]string, 0, 1+2*len(domains))
	lines = append(lines, fmt.Sprintf("%s%s from %s", downloadedTag, downloaded.Format(time.RFC3339), source))
	for _, d := range domains {
		lines = append(lines, "0.0.0.0 "+d, ":: "+d)
	}

	l := List{Name: name, Lines: lines}
	if i := f.listIndex(name); i >= 0 {
		f.lists[i] = l
	} else {
		f.lists = append(f.lists, l)
	}
}

// SetListOffAt schedules a blocklist to turn off at `at`; a zero `at`
// cancels that (the list stays on). It reports whether the list is on.
func (f *File) SetListOffAt(name string, at time.Time) bool {
	i := f.listIndex(name)
	if i < 0 {
		return false
	}
	var lines []string
	for _, line := range f.lists[i].Lines {
		if !strings.HasPrefix(line, offAtTag) {
			lines = append(lines, line)
		}
	}
	if !at.IsZero() {
		// Right after the "downloaded" line, so it is easy to find by eye.
		lines = slices.Insert(lines, min(1, len(lines)), offAtTag+at.Format(time.RFC3339))
	}
	f.lists[i].Lines = lines
	return true
}

// RemoveList turns a blocklist off right away. It reports whether it was on.
func (f *File) RemoveList(name string) bool {
	i := f.listIndex(name)
	if i < 0 {
		return false
	}
	f.lists = slices.Delete(f.lists, i, i+1)
	return true
}

// ExpireLists removes the blocklists whose turn-off time has passed.
func (f *File) ExpireLists(now time.Time) []string {
	var removed []string
	for _, info := range f.Lists() {
		if info.TurningOff() && !info.OffAt.After(now) {
			f.RemoveList(info.Name)
			removed = append(removed, info.Name+" blocklist")
		}
	}
	return removed
}

func (f File) listIndex(name string) int {
	for i, l := range f.lists {
		if l.Name == name {
			return i
		}
	}
	return -1
}

// ValidDomain checks the DNS rules for a domain exactly as written: no
// normalizing, so "www.x.com" stays "www.x.com". Used for downloaded lists.
func ValidDomain(host string) error { return validate(host) }
