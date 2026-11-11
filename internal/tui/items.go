package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/mahm0udnasr/sadd/internal/groups"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// A bubbles list shows anything that implements list.Item (FilterValue).
// The default delegate also uses Title and Description, which gives the
// clean two-line look.

// entryItem is one blocked site on the Blocked tab.
type entryItem struct {
	e      hosts.Entry
	now    time.Time
	marked bool // selected with space
}

// Title shows a dot in front of selected sites. FilterValue returns the
// SAME text on purpose: the list highlights filter matches by character
// position in the title, so the two must line up, or the highlight would
// be shifted by the "● " prefix.
func (i entryItem) Title() string {
	if i.marked {
		return "● " + i.e.Domain
	}
	return i.e.Domain
}
func (i entryItem) FilterValue() string { return i.Title() }
func (i entryItem) Description() string {
	if i.e.Timed() {
		return "◷ " + Remaining(i.e, i.now)
	}
	return "Always blocked"
}

func entryItems(entries []hosts.Entry, now time.Time, marked map[string]bool) []list.Item {
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = entryItem{e, now, marked[e.Domain]}
	}
	return items
}

// groupItem is one built-in group on the Groups tab.
type groupItem struct {
	name           string
	domains        []string
	blockedDomains []string
}

func (i groupItem) Title() string       { return i.name }
func (i groupItem) FilterValue() string { return i.name }
func (i groupItem) Description() string {
	state := fmt.Sprintf("%d of %d blocked", len(i.blockedDomains), len(i.domains))
	if len(i.blockedDomains) == len(i.domains) {
		state = "✓ all blocked"
	}
	return state + " · " + preview(i.domains, 3)
}

// groupItems lists the blocklists first, then the built-in groups.
func groupItems(entries []hosts.Entry, lists []Blocklist, now time.Time) []list.Item {
	blocked := make(map[string]bool, len(entries))
	for _, e := range entries {
		blocked[e.Domain] = true
	}
	var items []list.Item
	for _, b := range lists {
		items = append(items, blocklistItem{b, now})
	}
	for _, name := range groups.Names() {
		domains, _ := groups.Get(name)
		it := groupItem{name: name, domains: domains}
		for _, d := range domains {
			if blocked[d] {
				it.blockedDomains = append(it.blockedDomains, d)
			}
		}
		items = append(items, it)
	}
	return items
}

// newList creates a list with our look and without the parts we draw
// ourselves (title, help) or handle ourselves (quit).
func newList(singular, plural string) list.Model {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = d.Styles.NormalTitle.Foreground(text)
	d.Styles.NormalDesc = d.Styles.NormalDesc.Foreground(subtle)
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(accent).BorderForeground(accent)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(accentDim).BorderForeground(accent)
	d.Styles.FilterMatch = d.Styles.FilterMatch.Foreground(accent)

	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetStatusBarItemName(singular, plural)
	l.Styles.NoItems = dimStyle.Padding(0, 2)
	l.FilterInput.PromptStyle = l.FilterInput.PromptStyle.Foreground(accent)
	l.FilterInput.Cursor.Style = l.FilterInput.Cursor.Style.Foreground(accent)

	// We handle quitting and help ourselves.
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	return l
}
