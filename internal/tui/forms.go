package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/timer"
)

// Every dialog is a Huh form. A form has one or more groups (pages); Huh
// shows one group at a time, and a group whose hide func returns true is
// skipped. That is how "Custom…" works: picking it reveals a second page
// where you type the duration.

type formKind int

const (
	formAdd formKind = iota
	formBlockGroup
	formUnblock
	formUndo
	formDetails
	formExtend
	formBlocklist
)

// formData holds the values Huh fields write into (via pointers).
type formData struct {
	input   string        // Add: what the user typed
	dur     time.Duration // Add / group / extend: how long (or customDuration)
	custom  string        // the typed duration when dur == customDuration
	ok      bool          // confirmations
	action  detailAction  // Details: what to do next
	domains []string      // which sites the dialog is about
	label   string        // group name, for titles and messages
	entry   hosts.Entry   // Details / Extend (one site): the site shown
	now     time.Time

	list       Blocklist       // Blocklist dialog
	listAction BlocklistAction // what to do with it
	cooldown   time.Duration   // how long turning it off takes
}

// customDuration is the "Custom…" option. Real durations are never
// negative, so -1 cannot clash with one.
const customDuration = time.Duration(-1)

// duration is the chosen time, resolving "Custom…" to what was typed.
// The custom field is validated before submit, so the error is ignored.
func (d *formData) duration() time.Duration {
	if d.dur == customDuration {
		v, _ := timer.ParseDuration(d.custom)
		return v
	}
	return d.dur
}

// presets are the durations offered in every duration picker.
var presets = []struct {
	name string
	d    time.Duration
}{
	{"15 minutes", 15 * time.Minute},
	{"30 minutes", 30 * time.Minute},
	{"1 hour", time.Hour},
	{"2 hours", 2 * time.Hour},
	{"4 hours", 4 * time.Hour},
	{"8 hours", 8 * time.Hour},
	{"1 day", 24 * time.Hour},
}

// blockOptions: Forever, the presets, Custom….
func blockOptions() []huh.Option[time.Duration] {
	opts := []huh.Option[time.Duration]{huh.NewOption("Forever", time.Duration(0))}
	for _, p := range presets {
		opts = append(opts, huh.NewOption(p.name, p.d))
	}
	return append(opts, huh.NewOption("Custom…", customDuration))
}

// extendOptions: "+15 minutes" … "+1 day", Custom…. No Forever: to block a
// site forever, add it again with the Add dialog.
func extendOptions() []huh.Option[time.Duration] {
	var opts []huh.Option[time.Duration]
	for _, p := range presets {
		opts = append(opts, huh.NewOption("+"+p.name, p.d))
	}
	return append(opts, huh.NewOption("Custom…", customDuration))
}

// customGroup is the hidden second page for typing a duration.
func customGroup(d *formData, title string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title(title).
			Placeholder("45m, 1h30m, 2d").
			Description("Units: m (minutes), h (hours), d (days).").
			Value(&d.custom).
			Validate(func(s string) error {
				_, err := timer.ParseDuration(s)
				return err
			}),
	).WithHideFunc(func() bool { return d.dur != customDuration })
}

// formBuilder makes the groups for one kind of form.
type formBuilder func(d *formData) []*huh.Group

func newAddForm(d *formData) []*huh.Group {
	return []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Websites to block").
				Placeholder("youtube.com reddit.com https://x.com/home").
				Value(&d.input).
				// Live preview under the title, recomputed as you type.
				DescriptionFunc(func() string { return addPreview(d.input) }, &d.input).
				Validate(func(s string) error {
					domains, errs := parseDomains(s)
					if len(errs) > 0 {
						return errs[0]
					}
					if len(domains) == 0 {
						return errors.New("type at least one website")
					}
					return nil
				}),
			// Inline: one line, "‹ Forever ›", changed with ←/→.
			huh.NewSelect[time.Duration]().
				Title("Block for").
				Options(blockOptions()...).
				Inline(true).
				Value(&d.dur),
		),
		customGroup(d, "Block for how long?"),
	}
}

func newGroupForm(d *formData) []*huh.Group {
	return []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[time.Duration]().
				Title(fmt.Sprintf("Block the %s group", d.label)).
				Description(fmt.Sprintf("%s: %s", plural(len(d.domains), "site"), preview(d.domains, 4))).
				Options(blockOptions()...).
				Value(&d.dur),
		),
		customGroup(d, "Block "+d.label+" for how long?"),
	}
}

func newUnblockForm(d *formData) []*huh.Group {
	title := "Unblock " + d.domains[0] + "?"
	desc := "It will load again in your browser."
	if len(d.domains) > 1 {
		title = fmt.Sprintf("Unblock %s?", plural(len(d.domains), "site"))
		desc = preview(d.domains, 4)
		if d.label != "" {
			desc = "Everything blocked from the " + d.label + " group:\n" + desc
		}
	}
	return []*huh.Group{huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(desc).
			Affirmative("Unblock").
			Negative("Keep blocked"). // the default: a focus tool stays safe
			Value(&d.ok),
	)}
}

func newUndoForm(d *formData) []*huh.Group {
	return []*huh.Group{huh.NewGroup(
		huh.NewConfirm().
			Title("Undo the last change?").
			Description("Swaps the hosts file with its backup.\nRun undo again to redo.").
			Affirmative("Undo").
			Negative("Cancel").
			Value(&d.ok),
	)}
}

// detailAction is what the Details dialog leads to.
type detailAction int

const (
	actClose detailAction = iota
	actExtend
	actUnblock
)

// newDetailsForm shows one site. Its "buttons" are a Select; choosing
// Extend or Unblock opens that dialog next (see submitForm).
// d.ok here means "changes are allowed" (not read-only).
func newDetailsForm(d *formData) []*huh.Group {
	// Close comes first, so the cursor starts on the safe choice. (Huh also
	// scrolls a preselected option to the top of the list, hiding the
	// options above it, so the default should be the first one anyway.)
	opts := []huh.Option[detailAction]{huh.NewOption("Close", actClose)}
	if d.ok {
		if d.entry.Timed() {
			opts = append(opts, huh.NewOption("Extend the timer", actExtend))
		}
		opts = append(opts, huh.NewOption("Unblock", actUnblock))
	}

	d.action = actClose
	return []*huh.Group{huh.NewGroup(
		huh.NewSelect[detailAction]().
			Title(d.entry.Domain).
			Description(detailsText(d.entry, d.now, d.ok)).
			Options(opts...).
			Value(&d.action),
	)}
}

// detailsText is the body of the Details dialog.
func detailsText(e hosts.Entry, now time.Time, canWrite bool) string {
	var b strings.Builder
	if e.Timed() {
		fmt.Fprintf(&b, "◷ %s\n", Remaining(e, now))
		b.WriteString("Unblocks by itself when the time is up.\n")
	} else {
		b.WriteString("Blocked until you unblock it.\n")
	}
	b.WriteString("\nLines in the hosts file:\n")
	for _, l := range hosts.HostLines(e.Domain) {
		b.WriteString("  " + l + "\n")
	}
	if !canWrite {
		b.WriteString("\nRead-only: run with sudo to change it.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func newExtendForm(d *formData) []*huh.Group {
	title := "Extend " + d.domains[0]
	desc := "Now: " + Remaining(d.entry, d.now)
	if len(d.domains) > 1 {
		title = "Extend " + plural(len(d.domains), "site")
		desc = preview(d.domains, 4) + "\nEach keeps its own timer, plus the extra time."
	}
	if d.label != "" { // sites that were skipped
		desc += "\n" + d.label
	}
	d.dur = 30 * time.Minute
	return []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[time.Duration]().
				Title(title).
				Description(desc).
				Options(extendOptions()...).
				Inline(true). // "‹ +30 minutes ›": ←/→ to change
				Value(&d.dur),
		),
		customGroup(d, "Extend by how long?"),
	}
}

// openForm builds and shows a dialog. d holds the values it starts with.
func (m model) openForm(kind formKind, build formBuilder, d *formData) (tea.Model, tea.Cmd) {
	if d == nil {
		d = &formData{}
	}
	m.data = d

	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel"))

	m.form = huh.NewForm(build(m.data)...).
		WithTheme(formTheme()).
		WithKeyMap(keys).
		WithWidth(m.formWidth()).
		WithShowHelp(true)
	m.formKind = kind
	return m, m.form.Init()
}

// updateForm passes a message to the open form, then reacts when the
// user submits (StateCompleted) or cancels (StateAborted).
func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f, cmd := m.form.Update(msg)
	if f, ok := f.(*huh.Form); ok {
		m.form = f
	}

	switch m.form.State {
	case huh.StateAborted:
		m.form, m.data = nil, nil
		return m, m.setStatus("Cancelled.", false)
	case huh.StateCompleted:
		return m.submitForm()
	}
	return m, cmd
}

func (m model) submitForm() (tea.Model, tea.Cmd) {
	kind, d := m.formKind, m.data
	m.form, m.data = nil, nil

	switch kind {
	case formAdd:
		domains, _ := parseDomains(d.input) // already validated by the form
		return m, m.startSaving(m.blockCmd(domains, d.duration()))
	case formBlockGroup:
		return m, m.startSaving(m.blockCmd(d.domains, d.duration()))
	case formUnblock:
		if !d.ok {
			return m, m.setStatus("Kept blocked.", false)
		}
		domains := d.domains
		m.marked = nil // the marked sites are gone now
		return m, m.startSaving(func() tea.Msg {
			status, err := m.backend.Unblock(domains)
			return doneMsg{status, err}
		})
	case formUndo:
		if !d.ok {
			return m, m.setStatus("Cancelled.", false)
		}
		return m, m.startSaving(func() tea.Msg {
			status, err := m.backend.Restore()
			return doneMsg{status, err}
		})
	case formDetails:
		// One dialog leads to the next.
		switch d.action {
		case actExtend:
			return m.openForm(formExtend, newExtendForm,
				&formData{domains: []string{d.entry.Domain}, entry: d.entry, now: m.now})
		case actUnblock:
			return m.openForm(formUnblock, newUnblockForm, &formData{domains: []string{d.entry.Domain}})
		}
		return m, nil
	case formBlocklist:
		return m.submitBlocklist(d)
	case formExtend:
		domains, by := d.domains, d.duration()
		return m, m.startSaving(func() tea.Msg {
			status, err := m.backend.Extend(domains, by)
			return doneMsg{status, err}
		})
	}
	return m, nil
}

func (m model) blockCmd(domains []string, d time.Duration) tea.Cmd {
	return func() tea.Msg {
		status, err := m.backend.Block(BlockRequest{Domains: domains, For: d})
		return doneMsg{status, err}
	}
}

// addPreview is the live line under "Websites to block".
func addPreview(input string) string {
	domains, errs := parseDomains(input)
	switch {
	case len(errs) > 0:
		return "✗ " + errs[0].Error()
	case len(domains) == 0:
		return "Separate with spaces. Full URLs are fine."
	default:
		return "→ " + strings.Join(domains, ", ") + " (and www.)"
	}
}

// formTheme is Huh's Charm theme with our accent color. The dialog box
// draws the border, so the form's own left bar is turned off.
func formTheme() *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Title = t.Focused.Title.Foreground(accent)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(accent).Foreground(lipgloss.Color("#FFFFFF"))
	t.Focused.Base = t.Focused.Base.BorderLeft(false).PaddingLeft(0)
	t.Blurred.Base = t.Blurred.Base.BorderLeft(false).PaddingLeft(0)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	return t
}
