// Package tui is an interactive Bubble Tea screen for sadd.
//
// It knows nothing about files, sudo or DNS: it only calls the functions in
// Backend. That keeps the UI separate from the logic (like cmd/ is), and
// makes the model testable with a fake backend.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// Backend is everything the TUI needs from the rest of the program.
type Backend struct {
	Load    func() ([]hosts.Entry, error)
	Block   func(input string) (string, error) // returns a status message
	Unblock func(domain string) (string, error)
}

// Run starts the TUI and blocks until the user quits.
func Run(b Backend) error {
	_, err := tea.NewProgram(newModel(b), tea.WithAltScreen()).Run()
	return err
}

type mode int

const (
	browsing mode = iota
	adding
)

type model struct {
	backend Backend
	entries []hosts.Entry
	cursor  int
	mode    mode
	input   textinput.Model
	status  string
	isError bool
	now     time.Time
}

// Messages. In Bubble Tea, slow work (file I/O) runs inside a tea.Cmd and
// reports back with a message; Update itself must stay fast.
type (
	loadedMsg struct {
		entries []hosts.Entry
		err     error
	}
	doneMsg struct {
		status string
		err    error
	}
	tickMsg time.Time
)

func newModel(b Backend) model {
	in := textinput.New()
	in.Placeholder = "youtube.com or a URL"
	in.CharLimit = 255
	return model{backend: b, input: in, now: time.Now()}
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

// tick refreshes the "time left" text and picks up expired entries.
func tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		m.entries = msg.entries
		m.now = time.Now()
		m.cursor = min(m.cursor, max(len(m.entries)-1, 0))
		return m, nil

	case doneMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus(msg.status, false)
		}
		return m, m.load() // always reload: the file is the source of truth

	case tickMsg:
		return m, tea.Batch(m.load(), tick())

	case tea.KeyMsg:
		if m.mode == adding {
			return m.updateAdding(msg)
		}
		return m.updateBrowsing(msg)
	}
	return m, nil
}

func (m model) updateBrowsing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "a", "b":
		m.mode = adding
		m.input.SetValue("")
		m.setStatus("", false)
		return m, m.input.Focus()
	case "d", "u", "x", "delete":
		if len(m.entries) == 0 {
			return m, nil
		}
		domain := m.entries[m.cursor].Domain
		return m, func() tea.Msg {
			status, err := m.backend.Unblock(domain)
			return doneMsg{status, err}
		}
	case "r":
		return m, m.load()
	}
	return m, nil
}

func (m model) updateAdding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = browsing
		m.input.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		m.mode = browsing
		m.input.Blur()
		if value == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			status, err := m.backend.Block(value)
			return doneMsg{status, err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) setStatus(s string, isError bool) {
	m.status, m.isError = s, isError
}

// Styles (Lip Gloss).
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("161")).Padding(0, 1)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("161")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	timedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle     = dimStyle
	appStyle      = lipgloss.NewStyle().Padding(1, 2)
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("sadd · blocked websites"))
	b.WriteString("\n\n")

	if len(m.entries) == 0 {
		b.WriteString(dimStyle.Render("No websites are blocked. Press a to block one."))
		b.WriteString("\n")
	}

	width := 0
	for _, e := range m.entries {
		width = max(width, len(e.Domain))
	}
	for i, e := range m.entries {
		pointer, name := "  ", e.Domain
		if i == m.cursor {
			pointer = cursorStyle.Render("› ")
			name = selectedStyle.Render(name)
		}
		pad := strings.Repeat(" ", width-len(e.Domain))

		info := Describe(e, m.now)
		if e.Timed() {
			info = timedStyle.Render(info)
		} else {
			info = dimStyle.Render(info)
		}
		fmt.Fprintf(&b, "%s%s%s  %s\n", pointer, name, pad, info)
	}

	b.WriteString("\n")
	if m.mode == adding {
		b.WriteString("Block: " + m.input.View() + "\n\n")
		b.WriteString(helpStyle.Render("enter block • esc cancel"))
	} else {
		if m.status != "" {
			style := okStyle
			if m.isError {
				style = errStyle
			}
			b.WriteString(style.Render(m.status) + "\n\n")
		}
		b.WriteString(helpStyle.Render("↑/↓ move • a block • d unblock • r reload • q quit"))
	}

	return appStyle.Render(b.String())
}
