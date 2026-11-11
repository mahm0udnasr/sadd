package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// Blocklist is a big downloaded list (e.g. porn) as the TUI sees it.
// It is ONE row on the Groups tab, never 77,000 rows on Blocked.
type Blocklist struct {
	Name, Title, About string
	On                 bool
	Domains            int
	Downloaded         time.Time
	OffAt              time.Time // zero = not turning off
}

// TurningOff reports whether the list is in its turn-off cooldown.
func (b Blocklist) TurningOff() bool { return b.On && !b.OffAt.IsZero() }

// BlocklistAction is what the blocklist dialog asks the backend to do.
type BlocklistAction int

const (
	BlocklistNone   BlocklistAction = iota // "Close"
	BlocklistOn                            // turn on, or cancel turning off
	BlocklistOff                           // start the cooldown
	BlocklistUpdate                        // download the newest version
)

// blocklistItem is one blocklist on the Groups tab.
type blocklistItem struct {
	b   Blocklist
	now time.Time
}

func (i blocklistItem) Title() string       { return i.b.Title }
func (i blocklistItem) FilterValue() string { return i.b.Title }
func (i blocklistItem) Description() string {
	switch {
	case i.b.TurningOff():
		return "◷ turning off in " + ShortDuration(i.b.OffAt.Sub(i.now)) + " · enter to keep it on"
	case i.b.On:
		return fmt.Sprintf("✓ on · %s sites", Thousands(i.b.Domains))
	default:
		return "off · " + i.b.About
	}
}

// newBlocklistForm is the dialog for one blocklist. Its choices depend on
// the state. The first option is the default (see newDetailsForm for why).
func newBlocklistForm(d *formData) []*huh.Group {
	b := d.list
	var title, desc string
	var opts []huh.Option[BlocklistAction]
	switch {
	case b.TurningOff():
		title = b.Title + " is turning off"
		desc = fmt.Sprintf("It turns off at %s (in %s).", b.OffAt.Format("15:04"), ShortDuration(b.OffAt.Sub(d.now)))
		opts = []huh.Option[BlocklistAction]{
			huh.NewOption("Keep it on", BlocklistOn),
			huh.NewOption("Close", BlocklistNone),
		}
	case b.On:
		title = b.Title + " is on"
		desc = fmt.Sprintf("%s sites blocked, downloaded %s.\nTurning it off takes %s: the urge passes, the wait stays.",
			Thousands(b.Domains), b.Downloaded.Format("Mon Jan 2 15:04"), ShortDuration(d.cooldown))
		opts = []huh.Option[BlocklistAction]{
			huh.NewOption("Close", BlocklistNone),
			huh.NewOption("Update the list now", BlocklistUpdate),
			huh.NewOption("Turn off (in "+ShortDuration(d.cooldown)+")", BlocklistOff),
		}
	default:
		title = b.Title + " is off"
		desc = "Downloads " + b.About + ".\nTurning it off later takes " + ShortDuration(d.cooldown) + "."
		opts = []huh.Option[BlocklistAction]{
			huh.NewOption("Turn on", BlocklistOn),
			huh.NewOption("Cancel", BlocklistNone),
		}
	}
	d.listAction = opts[0].Value
	return []*huh.Group{huh.NewGroup(
		huh.NewSelect[BlocklistAction]().
			Title(title).
			Description(desc).
			Options(opts...).
			Value(&d.listAction),
	)}
}

// submitBlocklist runs the chosen action. Downloads take a few seconds,
// so the spinner says so.
func (m model) submitBlocklist(d *formData) (tea.Model, tea.Cmd) {
	if d.listAction == BlocklistNone {
		return m, nil
	}
	name, action := d.list.Name, d.listAction
	if action == BlocklistUpdate || (action == BlocklistOn && !d.list.On) {
		m.savingText = "Downloading the " + d.list.Title + " list…"
	}
	return m, m.startSaving(func() tea.Msg {
		status, err := m.backend.SetBlocklist(name, action)
		return doneMsg{status, err}
	})
}

// Thousands formats 76801 as "76,801" (the CLI uses it too).
func Thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
