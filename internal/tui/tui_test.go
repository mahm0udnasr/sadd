package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// fakeBackend keeps entries in memory, so the model can be tested
// without touching any file.
func fakeBackend(entries *[]hosts.Entry) Backend {
	return Backend{
		Load: func() ([]hosts.Entry, error) { return *entries, nil },
		Block: func(in string) (string, error) {
			if in == "bad" {
				return "", errors.New("invalid domain")
			}
			*entries = append(*entries, hosts.Entry{Domain: in})
			return "Blocked " + in, nil
		},
		Unblock: func(d string) (string, error) {
			var kept []hosts.Entry
			for _, e := range *entries {
				if e.Domain != d {
					kept = append(kept, e)
				}
			}
			*entries = kept
			return "Unblocked " + d, nil
		},
	}
}

// run feeds one message to the model, then follows the commands it returns,
// but only our own doneMsg/loadedMsg. Other commands (cursor blink, ticks)
// would wait on timers or loop forever, so they are skipped.
//
// Only call it for keys that do NOT return the textinput focus/blink
// command; see press for those.
func run(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(model)
	for cmd != nil {
		out := cmd()
		switch out.(type) {
		case doneMsg, loadedMsg:
			next, cmd = m.Update(out)
			m = next.(model)
		default:
			return m
		}
	}
	return m
}

// press sends a key and ignores the returned command entirely
// (used for "a", which returns a blinking-cursor command).
func press(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestBlockAndUnblock(t *testing.T) {
	entries := []hosts.Entry{{Domain: "reddit.com"}}
	m := newModel(fakeBackend(&entries))
	m = run(t, m, loadedMsg{entries: entries})

	// a -> type -> enter
	m = press(m, key("a"))
	if m.mode != adding {
		t.Fatal("pressing a should start adding")
	}
	for _, r := range "youtube.com" {
		m = press(m, key(string(r)))
	}
	m = run(t, m, key("enter"))

	if len(m.entries) != 2 || m.entries[1].Domain != "youtube.com" {
		t.Fatalf("entries = %+v", m.entries)
	}
	if !strings.Contains(m.View(), "youtube.com") {
		t.Error("view does not show the new domain")
	}

	// move down and unblock
	m = run(t, m, key("down"))
	m = run(t, m, key("d"))
	if len(m.entries) != 1 || m.entries[0].Domain != "reddit.com" {
		t.Fatalf("after unblock entries = %+v", m.entries)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped after delete)", m.cursor)
	}
}

func TestBlockErrorShown(t *testing.T) {
	var entries []hosts.Entry
	m := newModel(fakeBackend(&entries))
	m = press(m, key("a"))
	for _, r := range "bad" {
		m = press(m, key(string(r)))
	}
	m = run(t, m, key("enter"))
	if !m.isError || !strings.Contains(m.View(), "invalid domain") {
		t.Errorf("error not shown; status=%q", m.status)
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
