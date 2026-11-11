package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mahm0udnasr/sadd/internal/groups"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// ---------------------------------------------------------------- harness

// fake is an in-memory backend, so the model can be tested without
// touching any file. It records the last block request.
type fake struct {
	entries  []hosts.Entry
	lastReq  *BlockRequest
	restored bool
	extended []string      // domains of the last Extend call
	extendBy time.Duration // and by how much
}

func (f *fake) backend(canWrite bool) Backend {
	return Backend{
		CanWrite:     canWrite,
		ReadOnlyHint: "use sudo",
		Load:         func() ([]hosts.Entry, error) { return f.entries, nil },
		Block: func(r BlockRequest) (string, error) {
			f.lastReq = &r
			for _, d := range r.Domains {
				f.entries = append(f.entries, hosts.Entry{Domain: d})
			}
			return "Blocked.", nil
		},
		Extend: func(domains []string, by time.Duration) (string, error) {
			f.extended, f.extendBy = domains, by
			for i := range f.entries {
				for _, d := range domains {
					if f.entries[i].Domain == d {
						f.entries[i].Until = f.entries[i].Until.Add(by)
					}
				}
			}
			return "Extended.", nil
		},
		Unblock: func(domains []string) (string, error) {
			drop := map[string]bool{}
			for _, d := range domains {
				drop[d] = true
			}
			var kept []hosts.Entry
			for _, e := range f.entries {
				if !drop[e.Domain] {
					kept = append(kept, e)
				}
			}
			f.entries = kept
			return "Unblocked.", nil
		},
		Restore: func() (string, error) { f.restored = true; return "Restored.", nil },
		Info:    func() []InfoRow { return []InfoRow{{"Hosts file", "/tmp/hosts"}} },
	}
}

func start(t *testing.T, f *fake, canWrite bool) model {
	t.Helper()
	m := newModel(f.backend(canWrite))
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	return drive(m, m.load()())
}

// drive feeds msg to the model and keeps feeding the messages its commands
// produce (Huh's "next field", our doneMsg/loadedMsg, ...), like the real
// Bubble Tea loop. Commands that wait on a timer (cursor blink, status
// clearing, the 15s tick) are skipped by a short timeout.
func drive(m model, msg tea.Msg) model {
	queue := []tea.Msg{msg}
	for steps := 0; len(queue) > 0 && steps < 300; steps++ {
		cur := queue[0]
		queue = queue[1:]
		next, cmd := m.Update(cur)
		m = next.(model)
		queue = append(queue, collect(cmd)...)
	}
	return m
}

func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		switch msg := msg.(type) {
		case nil, spinner.TickMsg, tea.QuitMsg:
			return nil
		case tea.BatchMsg:
			var out []tea.Msg
			for _, c := range msg {
				out = append(out, collect(c)...)
			}
			return out
		default:
			return []tea.Msg{msg}
		}
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func k(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func keys(m model, ks ...string) model {
	for _, s := range ks {
		m = drive(m, k(s))
	}
	return m
}

func typeText(m model, s string) model {
	for _, r := range s {
		m = drive(m, k(string(r)))
	}
	return m
}

// ---------------------------------------------------------------- tests

func TestTabs(t *testing.T) {
	m := start(t, &fake{}, true)
	m = keys(m, "tab")
	if m.tab != tabGroups {
		t.Fatalf("tab = %v, want Groups", m.tab)
	}
	m = keys(m, "shift+tab", "shift+tab") // wraps to the last tab
	if m.tab != tabAbout {
		t.Fatalf("tab = %v, want About", m.tab)
	}
	m = keys(m, "1")
	if m.tab != tabBlocked {
		t.Fatalf("tab = %v, want Blocked", m.tab)
	}
}

func TestAddForm(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)

	m = keys(m, "a")
	if m.form == nil {
		t.Fatal("a did not open the form")
	}
	m = typeText(m, "https://www.YouTube.com/watch x.com")
	if v := m.View(); !strings.Contains(v, "youtube.com, x.com") {
		t.Errorf("live preview missing:\n%s", v)
	}

	m = keys(m, "enter")                   // to "Block for"
	m = keys(m, "right", "right", "enter") // inline select: 30 minutes, submit

	if f.lastReq == nil {
		t.Fatalf("nothing blocked; view:\n%s", m.View())
	}
	if f.lastReq.For != 30*time.Minute || strings.Join(f.lastReq.Domains, ",") != "youtube.com,x.com" {
		t.Errorf("request = %+v", *f.lastReq)
	}
	if m.form != nil || len(m.entries) != 2 {
		t.Errorf("form open = %v, entries = %d", m.form != nil, len(m.entries))
	}
}

func TestAddFormValidation(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)
	m = keys(m, "a")
	m = typeText(m, "good.com localhost")
	m = keys(m, "enter")

	if m.form == nil || f.lastReq != nil {
		t.Fatal("invalid input was accepted")
	}
	if v := m.View(); !strings.Contains(v, "localhost") {
		t.Errorf("error not shown:\n%s", v)
	}
}

func TestEscCancelsForm(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)
	m = keys(m, "a")
	m = typeText(m, "x.com")
	m = keys(m, "esc")
	if m.form != nil || f.lastReq != nil {
		t.Error("esc did not cancel")
	}
	if m.tab != tabBlocked {
		t.Error("esc in a form should not quit or switch tabs")
	}
}

func TestUnblockConfirmDefaultsToKeep(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}, {Domain: "b.com"}}}
	m := start(t, f, true)

	m = keys(m, "d")
	if m.form == nil || !strings.Contains(m.View(), "Unblock a.com?") {
		t.Fatalf("no confirmation:\n%s", m.View())
	}
	m = keys(m, "enter") // default answer
	if len(f.entries) != 2 {
		t.Fatal("enter on the default answer unblocked")
	}

	m = keys(m, "down", "d", "y")
	if len(m.entries) != 1 || m.entries[0].Domain != "a.com" {
		t.Errorf("entries = %+v", m.entries)
	}
}

func TestBlockGroup(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)
	m = keys(m, "2", "enter")
	if m.form == nil {
		t.Fatal("enter on a group did not open the form")
	}
	m = keys(m, "down", "down", "down", "enter") // 1 hour

	name := groups.Names()[0]
	want, _ := groups.Get(name)
	if f.lastReq == nil || f.lastReq.For != time.Hour || len(f.lastReq.Domains) != len(want) {
		t.Fatalf("request = %+v", f.lastReq)
	}
	if v := m.View(); !strings.Contains(v, "all blocked") {
		t.Errorf("group should show all blocked:\n%s", v)
	}
}

func TestUnblockGroupWhenNothingBlocked(t *testing.T) {
	m := start(t, &fake{}, true)
	m = keys(m, "2", "u")
	if m.form != nil || !strings.Contains(m.status, "Nothing from") {
		t.Errorf("status = %q", m.status)
	}
}

func TestUndo(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)
	m = keys(m, "z", "y")
	if !f.restored {
		t.Error("restore not called")
	}
}

func TestReadOnly(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}}}
	m := start(t, f, false)
	if !strings.Contains(m.View(), "read-only") {
		t.Error("read-only not shown")
	}
	for _, ch := range []string{"a", "d", "z"} {
		m = keys(m, ch)
		if m.form != nil || !m.isError || !strings.Contains(m.status, "use sudo") {
			t.Errorf("key %q: form=%v status=%q", ch, m.form != nil, m.status)
		}
	}
	m = keys(m, "2", "enter")
	if m.form != nil {
		t.Error("group form opened in read-only mode")
	}
	if strings.Contains(m.help.View(m.keys), "add sites") {
		t.Error("help shows change keys in read-only mode")
	}
	if fresh := start(t, f, false); strings.Contains(fresh.View(), "add sites") {
		t.Error("help shows change keys at startup in read-only mode")
	}
}

func TestFilterKeysDoNotTriggerActions(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "youtube.com"}, {Domain: "reddit.com"}}}
	m := start(t, f, true)
	m = keys(m, "/")
	m = typeText(m, "add")
	if m.form != nil || m.tab != tabBlocked {
		t.Fatal("typing in the filter triggered a shortcut")
	}
	m = keys(m, "esc")
	if m.filtering() {
		t.Error("esc did not leave filtering")
	}
}

func TestAboutPage(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "youtube.com", Until: time.Now().Add(time.Hour)}}}
	m := start(t, f, true)
	m = keys(m, "3")
	v := m.View()
	for _, want := range []string{"Hosts file", "/tmp/hosts", "Next unblock", "youtube.com"} {
		if !strings.Contains(v, want) {
			t.Errorf("About missing %q:\n%s", want, v)
		}
	}
}

// The layout must fit the terminal exactly: never taller (it would
// scroll) and never wider (lines would wrap).
func TestViewFitsTerminal(t *testing.T) {
	f := &fake{entries: []hosts.Entry{
		{Domain: "youtube.com", Until: time.Now().Add(time.Hour)},
		{Domain: "facebook.com"}, {Domain: "news.ycombinator.com"},
	}}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {64, 20}} {
		m := start(t, f, true)
		m = drive(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		screens := map[string]model{
			"blocked": keys(m, "1"),
			"groups":  keys(m, "2"),
			"about":   keys(m, "3"),
			"form":    keys(m, "1", "a"),
			"details": keys(m, "1", "enter"),
			"extend":  keys(m, "1", "e"),
			"custom":  keys(keys(m, "1", "e"), "left", "enter"),
			"help":    keys(m, "1", "?"),
		}
		for name, s := range screens {
			v := s.View()
			if h := lipgloss.Height(v); h > size[1] {
				t.Errorf("%dx%d %s: height %d", size[0], size[1], name, h)
			}
			if w := lipgloss.Width(v); w > size[0] {
				t.Errorf("%dx%d %s: width %d", size[0], size[1], name, w)
			}
		}
	}
}

func TestDescribe(t *testing.T) {
	now := time.Date(2026, 11, 10, 22, 0, 0, 0, time.Local)
	tests := []struct {
		e    hosts.Entry
		want string
	}{
		{hosts.Entry{Domain: "a.com"}, "forever"},
		{hosts.Entry{Domain: "a.com", Until: now.Add(-time.Minute)}, "expired"},
		{hosts.Entry{Domain: "a.com", Until: now.Add(119 * time.Minute)}, "(1h59m left)"},
		{hosts.Entry{Domain: "a.com", Until: now.Add(26 * time.Hour)}, "(1d2h left)"},
		{hosts.Entry{Domain: "a.com", Until: now.Add(30 * time.Second)}, "(<1m left)"},
	}
	for _, tt := range tests {
		if got := Describe(tt.e, now); !strings.Contains(got, tt.want) {
			t.Errorf("Describe(%v) = %q, want it to contain %q", tt.e.Until, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------- dialogs

func TestDialogIsCenteredOverDimmedScreen(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "youtube.com"}}}
	m := start(t, f, true)
	m = keys(m, "d")
	v := m.View()
	if !strings.Contains(v, "╭") || !strings.Contains(v, "Unblock youtube.com?") {
		t.Fatalf("no dialog box:\n%s", v)
	}
	// The screen behind is still there (dimmed), not replaced.
	if !strings.Contains(v, "Blocked") || !strings.Contains(v, "sadd") {
		t.Errorf("screen behind the dialog is gone:\n%s", v)
	}
	if h := lipgloss.Height(v); h != 30 {
		t.Errorf("height = %d, want 30", h)
	}
}

func TestOverlay(t *testing.T) {
	bg := "aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc"
	got := overlay(bg, "XX", 10, 3)
	lines := strings.Split(got, "\n")
	// ANSI colors aside, the middle line is cut around the dialog.
	if plain := stripANSI(lines[1]); plain != "bbbbXXbbbb" {
		t.Errorf("middle line = %q", plain)
	}
	if plain := stripANSI(lines[0]); plain != "aaaaaaaaaa" {
		t.Errorf("top line = %q", plain)
	}
}

func TestCustomDuration(t *testing.T) {
	f := &fake{}
	m := start(t, f, true)
	m = keys(m, "a")
	m = typeText(m, "x.com")
	m = keys(m, "enter", "left", "enter") // ← from Forever wraps to Custom…
	if m.form == nil || !strings.Contains(m.View(), "how long") {
		t.Fatalf("custom page not shown:\n%s", m.View())
	}
	m = typeText(m, "nope")
	m = keys(m, "enter")
	if f.lastReq != nil {
		t.Fatal("an invalid duration was accepted")
	}
	for range 4 {
		m = keys(m, "backspace")
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlU}) // clear the input
	m = typeText(m, "1h30m")
	m = keys(m, "enter")
	if f.lastReq == nil || f.lastReq.For != 90*time.Minute {
		t.Fatalf("request = %+v; view:\n%s", f.lastReq, m.View())
	}
}

func TestMultiSelectUnblock(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}, {Domain: "b.com"}, {Domain: "c.com"}}}
	m := start(t, f, true)
	m = keys(m, " ", " ") // mark a.com and b.com (space moves down)
	if len(m.marked) != 2 || !strings.Contains(m.View(), "2 selected") {
		t.Fatalf("marked = %v", m.marked)
	}
	m = keys(m, "d")
	if !strings.Contains(m.View(), "Unblock 2 sites?") {
		t.Fatalf("dialog:\n%s", m.View())
	}
	m = keys(m, "y")
	if len(m.entries) != 1 || m.entries[0].Domain != "c.com" || len(m.marked) != 0 {
		t.Errorf("entries = %+v, marked = %v", m.entries, m.marked)
	}
}

func TestEscClearsSelectionBeforeQuitting(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}}}
	m := start(t, f, true)
	m = keys(m, " ")
	next, cmd := m.Update(k("esc"))
	m = next.(model)
	if len(m.marked) != 0 {
		t.Error("esc did not clear the selection")
	}
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Error("esc quit while sites were selected")
		}
	}
}

func TestSelectAll(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}, {Domain: "b.com"}}}
	m := start(t, f, true)
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(m.marked) != 2 {
		t.Fatalf("marked = %v", m.marked)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(m.marked) != 0 {
		t.Errorf("second ctrl+a should select none, marked = %v", m.marked)
	}
}

func TestDetailsThenExtend(t *testing.T) {
	until := time.Now().Add(time.Hour)
	f := &fake{entries: []hosts.Entry{{Domain: "youtube.com", Until: until}}}
	m := start(t, f, true)
	m = keys(m, "enter")
	v := m.View()
	for _, want := range []string{"youtube.com", "left", "0.0.0.0 youtube.com", "Extend the timer"} {
		if !strings.Contains(v, want) {
			t.Errorf("details missing %q:\n%s", want, v)
		}
	}
	m = keys(m, "down", "enter") // Close -> Extend
	if m.formKind != formExtend {
		t.Fatalf("Extend dialog not opened; view:\n%s", m.View())
	}
	m = keys(m, "enter") // default: +30 minutes
	if len(f.extended) != 1 || f.extendBy != 30*time.Minute {
		t.Errorf("extend = %v by %v", f.extended, f.extendBy)
	}
}

func TestDetailsCloseIsDefault(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com"}}}
	m := start(t, f, true)
	m = keys(m, "enter", "enter")
	if m.form != nil || len(f.entries) != 1 {
		t.Errorf("enter, enter should just close; form=%v entries=%d", m.form != nil, len(f.entries))
	}
}

func TestDetailsReadOnly(t *testing.T) {
	f := &fake{entries: []hosts.Entry{{Domain: "a.com", Until: time.Now().Add(time.Hour)}}}
	m := start(t, f, false)
	m = keys(m, "enter")
	v := m.View()
	if m.form == nil || strings.Contains(v, "Extend the timer") || !strings.Contains(v, "Read-only") {
		t.Errorf("read-only details:\n%s", v)
	}
}

func TestExtendSkipsForever(t *testing.T) {
	f := &fake{entries: []hosts.Entry{
		{Domain: "a.com"},
		{Domain: "b.com", Until: time.Now().Add(time.Hour)},
	}}
	m := start(t, f, true)
	m = keys(m, "e") // cursor on a.com, blocked forever
	if m.form != nil || !strings.Contains(m.status, "forever") {
		t.Fatalf("form=%v status=%q", m.form != nil, m.status)
	}
	m = drive(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	m = keys(m, "e")
	if !strings.Contains(m.View(), "Skipped") {
		t.Errorf("skipped site not named:\n%s", m.View())
	}
	m = keys(m, "enter")
	if strings.Join(f.extended, ",") != "b.com" {
		t.Errorf("extended = %v", f.extended)
	}
}

func stripANSI(s string) string { return ansi.Strip(s) }
