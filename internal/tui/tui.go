// Package tui is the interactive screen for sadd, built with Charm's stack:
//
//	Bubble Tea  the Elm-style loop: Init -> Update(msg) -> View
//	Bubbles     ready-made components: list, spinner, help
//	Huh         forms: add sites, pick a duration, confirm (shown as dialogs)
//	Lip Gloss   colors and layout
//
// It knows nothing about files, sudo, DNS or systemd: it only calls the
// functions in Backend, so it can be tested with a fake backend.
//
// Files:
//
//	tui.go    model, messages, Update
//	keys.go   key bindings (shown by the help component)
//	items.go  list items for the Blocked and Groups tabs
//	forms.go  Huh forms and what happens when they are submitted
//	dialog.go draws the open form as a centered dialog over the screen
//	view.go   View and styles
//	format.go small text helpers shared with the CLI
package tui

import (
	"maps"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// BlockRequest blocks Domains (already normalized) for For (0 = forever).
type BlockRequest struct {
	Domains []string
	For     time.Duration
}

// Backend is everything the TUI needs from the rest of the program.
// Each action returns a short status message.
type Backend struct {
	Load    func() ([]hosts.Entry, error)
	Block   func(BlockRequest) (string, error)
	Extend  func(domains []string, by time.Duration) (string, error) // timed blocks only
	Unblock func(domains []string) (string, error)
	Restore func() (string, error)

	// CanWrite is false when changes are impossible (e.g. not running with
	// sudo). The TUI then shows READ-ONLY and refuses changes up front.
	CanWrite     bool
	ReadOnlyHint string
}

// Run starts the TUI and blocks until the user quits.
func Run(b Backend) error {
	_, err := tea.NewProgram(newModel(b), tea.WithAltScreen()).Run()
	return err
}

type tab int

const (
	tabBlocked tab = iota
	tabGroups
	tabCount // keep last
)

var tabNames = [tabCount]string{"Blocked", "Groups"}

type model struct {
	backend Backend
	keys    keyMap
	help    help.Model
	spinner spinner.Model

	tab           tab
	width, height int

	entries []hosts.Entry
	now     time.Time
	marked  map[string]bool // sites selected with space (Blocked tab)

	blockedList list.Model
	groupList   list.Model

	// The open Huh form, if any. Its values live in *formData on the heap:
	// Bubble Tea copies the model on every Update, so pointers into the
	// model itself would point at a stale copy.
	form     *huh.Form
	formKind formKind
	data     *formData

	saving   bool
	status   string
	isError  bool
	statusID int // only the newest status is cleared by its timer
}

// Messages. Slow work (file I/O) runs in a tea.Cmd and reports back with a
// message; Update itself must stay fast.
type (
	loadedMsg struct {
		entries []hosts.Entry
		err     error
	}
	doneMsg struct {
		status string
		err    error
	}
	tickMsg        time.Time
	clearStatusMsg int
)

func newModel(b Backend) model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	h := help.New()
	h.Styles.ShortKey = keyStyle
	h.Styles.FullKey = keyStyle
	h.Styles.ShortDesc = dimStyle
	h.Styles.FullDesc = dimStyle
	h.Styles.ShortSeparator = dimStyle
	h.Styles.FullSeparator = dimStyle

	m := model{
		backend:     b,
		keys:        newKeyMap(),
		help:        h,
		spinner:     sp,
		now:         time.Now(),
		blockedList: newList("site", "sites"),
		groupList:   newList("group", "groups"),
	}
	m.groupList.SetItems(groupItems(nil))
	m.keys.update(tabBlocked, b.CanWrite) // hide change keys in read-only mode
	m.resize(80, 24)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.load(), tick())
}

func (m model) load() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.backend.Load()
		return loadedMsg{entries, err}
	}
}

// tick refreshes "time left" and picks up entries a timer removed.
func tick() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// ---------------------------------------------------------------- Update

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		if m.form != nil {
			m.form = m.form.WithWidth(m.formWidth())
		}
		return m, nil

	case loadedMsg:
		if msg.err != nil {
			return m, m.setStatus(msg.err.Error(), true)
		}
		m.entries, m.now = msg.entries, time.Now()
		m.pruneMarks()
		cmds := []tea.Cmd{
			m.blockedList.SetItems(entryItems(m.entries, m.now, m.marked)),
			m.groupList.SetItems(groupItems(m.entries)),
		}
		return m, tea.Batch(cmds...)

	case doneMsg:
		m.saving = false
		var status tea.Cmd
		if msg.err != nil {
			status = m.setStatus(msg.err.Error(), true)
		} else {
			status = m.setStatus(msg.status, false)
		}
		return m, tea.Batch(status, m.load()) // the file is the source of truth

	case tickMsg:
		return m, tea.Batch(m.load(), tick())

	case clearStatusMsg:
		if int(msg) == m.statusID {
			m.status = ""
		}
		return m, nil

	case spinner.TickMsg:
		if !m.saving {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// An open form gets every other message (keys, cursor blink, ...).
	if m.form != nil {
		return m.updateForm(msg)
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		return m.handleKey(k)
	}
	return m.updateActive(msg)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}
	// While typing a filter in a list, every key belongs to the filter.
	// With a filter applied, esc clears it instead of quitting.
	if m.filtering() || (msg.String() == "esc" && m.filterApplied()) {
		return m.updateActive(msg)
	}
	// Then esc clears the selection. Only with nothing to clear, it quits.
	if msg.String() == "esc" && len(m.marked) > 0 {
		m.marked = nil
		return m, m.refreshBlocked()
	}

	// Read-only: refuse change keys up front instead of failing later.
	// (key.Matches ignores disabled bindings, so compare raw keys here.)
	if !m.backend.CanWrite && m.isWriteKey(msg.String()) {
		return m, m.setStatus("Read-only. "+m.backend.ReadOnlyHint, true)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.NextTab):
		return m.switchTab(m.tab + 1), nil
	case key.Matches(msg, m.keys.PrevTab):
		return m.switchTab(m.tab - 1), nil
	case key.Matches(msg, m.keys.Tab1):
		return m.switchTab(tabBlocked), nil
	case key.Matches(msg, m.keys.Tab2):
		return m.switchTab(tabGroups), nil
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.resize(m.width, m.height) // full help is taller
		return m, nil
	case key.Matches(msg, m.keys.Reload):
		return m, tea.Batch(m.load(), m.setStatus("Reloaded.", false))
	}

	switch {
	case key.Matches(msg, m.keys.Add):
		return m.openForm(formAdd, newAddForm, nil)
	case key.Matches(msg, m.keys.Undo):
		return m.openForm(formUndo, newUndoForm, nil)
	}

	if m.tab == tabBlocked {
		switch {
		case key.Matches(msg, m.keys.Details):
			if it, ok := m.blockedList.SelectedItem().(entryItem); ok {
				return m.openForm(formDetails, newDetailsForm,
					&formData{entry: it.e, now: m.now, ok: m.backend.CanWrite})
			}
			return m, nil
		case key.Matches(msg, m.keys.Select):
			if it, ok := m.blockedList.SelectedItem().(entryItem); ok {
				m.toggleMark(it.e.Domain)
				cmd := m.refreshBlocked()
				m.blockedList.CursorDown() // like a file manager: mark and move on
				return m, cmd
			}
			return m, nil
		case key.Matches(msg, m.keys.SelectAll):
			m.toggleAll()
			return m, m.refreshBlocked()
		case key.Matches(msg, m.keys.Unblock):
			if t := m.targets(); len(t) > 0 {
				return m.openForm(formUnblock, newUnblockForm, &formData{domains: domainsOf(t)})
			}
			return m, nil
		case key.Matches(msg, m.keys.Extend):
			return m.openExtend(m.targets())
		}
	}

	if m.tab == tabGroups {
		it, ok := m.groupList.SelectedItem().(groupItem)
		switch {
		case ok && key.Matches(msg, m.keys.BlockGroup):
			return m.openForm(formBlockGroup, newGroupForm, &formData{label: it.name, domains: it.domains})
		case ok && key.Matches(msg, m.keys.UnblockGroup):
			if len(it.blockedDomains) == 0 {
				return m, m.setStatus("Nothing from "+it.name+" is blocked.", false)
			}
			return m.openForm(formUnblock, newUnblockForm, &formData{label: it.name, domains: it.blockedDomains})
		}
	}

	return m.updateActive(msg)
}

// updateActive passes a message to the component of the current tab
// (list navigation and filtering).
func (m model) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.tab {
	case tabBlocked:
		m.blockedList, cmd = m.blockedList.Update(msg)
	case tabGroups:
		m.groupList, cmd = m.groupList.Update(msg)
	}
	return m, cmd
}

func (m model) switchTab(t tab) model {
	m.tab = (t + tabCount) % tabCount // wrap around both ways
	m.keys.update(m.tab, m.backend.CanWrite)
	m.resize(m.width, m.height) // help height can change per tab
	return m
}

// isWriteKey reports whether k would change something on the current tab.
func (m model) isWriteKey(k string) bool {
	bindings := []key.Binding{m.keys.Add, m.keys.Undo}
	switch m.tab {
	case tabBlocked:
		bindings = append(bindings, m.keys.Unblock, m.keys.Extend, m.keys.Select, m.keys.SelectAll)
	case tabGroups:
		bindings = append(bindings, m.keys.BlockGroup, m.keys.UnblockGroup)
	}
	for _, b := range bindings {
		for _, bk := range b.Keys() {
			if bk == k {
				return true
			}
		}
	}
	return false
}

func (m model) filterApplied() bool {
	switch m.tab {
	case tabBlocked:
		return m.blockedList.FilterState() == list.FilterApplied
	case tabGroups:
		return m.groupList.FilterState() == list.FilterApplied
	}
	return false
}

func (m model) filtering() bool {
	switch m.tab {
	case tabBlocked:
		return m.blockedList.FilterState() == list.Filtering
	case tabGroups:
		return m.groupList.FilterState() == list.Filtering
	}
	return false
}

// setStatus shows a message and clears it after a few seconds.
// Errors stay longer, so there is time to read them.
func (m *model) setStatus(s string, isError bool) tea.Cmd {
	m.status, m.isError = s, isError
	m.statusID++
	id := m.statusID
	wait := 4 * time.Second
	if isError {
		wait = 10 * time.Second
	}
	return tea.Tick(wait, func(time.Time) tea.Msg { return clearStatusMsg(id) })
}

// startSaving shows the spinner while cmd runs.
func (m *model) startSaving(cmd tea.Cmd) tea.Cmd {
	m.saving = true
	m.status = ""
	return tea.Batch(m.spinner.Tick, cmd)
}

// ---------------------------------------------------------------- selection

// The selection is a map from domain to true. It is replaced, never
// changed in place: Bubble Tea keeps old copies of the model around, and
// a shared map would change under them too.

func (m *model) toggleMark(domain string) {
	marked := make(map[string]bool, len(m.marked)+1)
	maps.Copy(marked, m.marked)
	if marked[domain] {
		delete(marked, domain)
	} else {
		marked[domain] = true
	}
	m.marked = marked
}

// toggleAll selects every site, or none if all are already selected.
func (m *model) toggleAll() {
	if len(m.marked) == len(m.entries) {
		m.marked = nil
		return
	}
	m.marked = make(map[string]bool, len(m.entries))
	for _, e := range m.entries {
		m.marked[e.Domain] = true
	}
}

// pruneMarks forgets selected sites that are no longer blocked.
func (m *model) pruneMarks() {
	if len(m.marked) == 0 {
		return
	}
	kept := make(map[string]bool, len(m.marked))
	for _, e := range m.entries {
		if m.marked[e.Domain] {
			kept[e.Domain] = true
		}
	}
	m.marked = kept
}

// refreshBlocked redraws the Blocked list, e.g. after the selection changed.
func (m *model) refreshBlocked() tea.Cmd {
	return m.blockedList.SetItems(entryItems(m.entries, m.now, m.marked))
}

// targets is what Unblock and Extend act on: the selected sites (in list
// order), or else the one under the cursor.
func (m model) targets() []hosts.Entry {
	var out []hosts.Entry
	for _, e := range m.entries {
		if m.marked[e.Domain] {
			out = append(out, e)
		}
	}
	if len(out) > 0 {
		return out
	}
	if it, ok := m.blockedList.SelectedItem().(entryItem); ok {
		return []hosts.Entry{it.e}
	}
	return nil
}

// openExtend opens the Extend dialog for the timed sites among targets.
// Sites blocked forever have no timer to extend; they are skipped and
// named in the dialog.
func (m model) openExtend(targets []hosts.Entry) (tea.Model, tea.Cmd) {
	var timed []hosts.Entry
	var forever []string
	for _, e := range targets {
		if e.Timed() {
			timed = append(timed, e)
		} else {
			forever = append(forever, e.Domain)
		}
	}
	switch {
	case len(targets) == 0:
		return m, nil
	case len(timed) == 0 && len(targets) == 1:
		return m, m.setStatus(targets[0].Domain+" is blocked forever; there is no timer to extend.", false)
	case len(timed) == 0:
		return m, m.setStatus("None of the selected sites has a timer to extend.", false)
	}

	d := &formData{domains: domainsOf(timed), entry: timed[0], now: m.now}
	if len(forever) > 0 {
		d.label = "Skipped, blocked forever: " + preview(forever, 3)
	}
	return m.openForm(formExtend, newExtendForm, d)
}

func domainsOf(entries []hosts.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Domain
	}
	return out
}
