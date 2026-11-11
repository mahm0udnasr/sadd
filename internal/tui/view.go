package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// ---------------------------------------------------------------- styles

// One accent color, neutral grays, and color only where it means
// something (accent = selected, green = ok, red = error, amber = timed).
var (
	accent    = lipgloss.AdaptiveColor{Light: "#6C50FF", Dark: "#8B7BFF"}
	accentDim = lipgloss.AdaptiveColor{Light: "#9A88FF", Dark: "#6A5ED0"}
	text      = lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#EDEDED"}
	subtle    = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6E6E6E"}
	faint     = lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#2E2E2E"}
	green     = lipgloss.AdaptiveColor{Light: "#0E9F6E", Dark: "#34D399"}
	red       = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	amber     = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	shade     = lipgloss.AdaptiveColor{Light: "#C4C4C4", Dark: "#4A4A4A"} // screen behind a dialog

	appStyle    = lipgloss.NewStyle().Padding(1, 2)
	logoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(accent).Bold(true).Padding(0, 1)
	dimStyle    = lipgloss.NewStyle().Foreground(subtle)
	accentStyle = lipgloss.NewStyle().Foreground(accent)
	keyStyle    = lipgloss.NewStyle().Foreground(text)
	okStyle     = lipgloss.NewStyle().Foreground(green)
	errStyle    = lipgloss.NewStyle().Foreground(red)
	amberStyle  = lipgloss.NewStyle().Foreground(amber)
	shadeStyle  = lipgloss.NewStyle().Foreground(shade)

	// Tabs are plain words; the active one is underlined in the accent color.
	tabStyle       = lipgloss.NewStyle().Foreground(subtle).Padding(0, 1).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(faint)
	activeTabStyle = tabStyle.Foreground(accent).Bold(true).Border(lipgloss.ThickBorder(), false, false, true, false).BorderForeground(accent)
)

// ---------------------------------------------------------------- layout

// Height of everything except the body: app padding (2), header (1),
// gap (1), tabs (2), gap (1), gap + status (2), help (n).
func (m model) chromeHeight() int {
	return 2 + 1 + 1 + 2 + 1 + 2 + lipgloss.Height(m.helpView())
}

func (m model) innerWidth() int { return max(m.width-4, 20) }

// resize gives every component its share of the screen.
func (m *model) resize(w, h int) {
	m.width, m.height = w, h
	m.help.Width = m.innerWidth()
	bodyH := max(h-m.chromeHeight(), 3)
	m.blockedList.SetSize(m.innerWidth(), bodyH)
	m.groupList.SetSize(m.innerWidth(), bodyH)
}

// helpView is the key help, cut to the screen width. (bubbles/help v1.0.0
// can overflow when even its "…" no longer fits, so we cut it ourselves.)
func (m model) helpView() string {
	return lipgloss.NewStyle().MaxWidth(m.innerWidth()).Render(m.help.View(m.keys))
}

func (m model) bodyHeight() int { return max(m.height-m.chromeHeight(), 3) }

// ---------------------------------------------------------------- View

// View is the normal screen, plus the open dialog (if any) on top of it.
func (m model) View() string {
	screen := m.screen()
	if m.form == nil {
		return screen
	}
	return overlay(screen, m.dialogView(), m.width, m.height)
}

func (m model) screen() string {
	var body string
	switch {
	case m.tab == tabBlocked && len(m.entries) == 0:
		body = m.emptyState()
	case m.tab == tabBlocked:
		body = m.blockedList.View()
	case m.tab == tabGroups:
		body = m.groupList.View()
	}
	// Keep the footer at the same place on every tab.
	body = lipgloss.NewStyle().Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(body)

	footer := m.statusLine() + "\n" + m.helpView()

	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		m.header(),
		"",
		m.tabs(),
		"",
		body,
		"",
		footer,
	))
}

func (m model) header() string {
	var mode string
	if m.backend.CanWrite {
		mode = okStyle.Render("● ") + dimStyle.Render("read-write")
	} else {
		mode = amberStyle.Render("● read-only")
	}
	count := plural(len(m.entries), "site") + " blocked"
	timed := ""
	if t := m.timedCount(); t > 0 {
		timed = fmt.Sprintf(" · %d timed", t)
	}
	logo := logoStyle.Render("sadd")
	subtitle := "  " + dimStyle.Render("website blocker")

	// Try the full header first, then drop parts until it fits.
	variants := [][2]string{
		{logo + subtitle, mode + dimStyle.Render("  ·  "+count+timed)},
		{logo, mode + dimStyle.Render("  ·  "+count+timed)},
		{logo, mode + dimStyle.Render("  ·  "+count)},
		{logo, mode},
	}
	for _, v := range variants {
		gap := m.innerWidth() - lipgloss.Width(v[0]) - lipgloss.Width(v[1])
		if gap >= 1 {
			return v[0] + strings.Repeat(" ", gap) + v[1]
		}
	}
	return logo
}

func (m model) tabs() string {
	parts := make([]string, 0, tabCount+1)
	for i, name := range tabNames {
		if tab(i) == m.tab {
			parts = append(parts, activeTabStyle.Render(name))
		} else {
			parts = append(parts, tabStyle.Render(name))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
	// A faint line under the rest of the row, like a modern tab bar.
	if rest := m.innerWidth() - lipgloss.Width(row); rest > 0 {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row,
			lipgloss.NewStyle().Foreground(faint).Render(strings.Repeat("─", rest)))
	}
	return row
}

func (m model) emptyState() string {
	msg := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(text).Bold(true).Render("Nothing is blocked"),
		"",
		dimStyle.Render(m.emptyHint()),
	)
	return lipgloss.Place(m.innerWidth(), m.bodyHeight(), lipgloss.Center, lipgloss.Center, msg)
}

func (m model) emptyHint() string {
	if !m.backend.CanWrite {
		return m.backend.ReadOnlyHint
	}
	for _, l := range m.lists {
		if l.On {
			return "The " + l.Title + " is on (see Groups). Press a to add sites."
		}
	}
	return "Press a to add sites, or go to Groups."
}

func (m model) statusLine() string {
	switch {
	case m.saving:
		text := m.savingText
		if text == "" {
			text = "Saving…"
		}
		return m.spinner.View() + dimStyle.Render(" "+text)
	case m.status == "" && len(m.marked) > 0 && m.tab == tabBlocked:
		return accentStyle.Render(fmt.Sprintf("● %d selected", len(m.marked))) +
			dimStyle.Render("  d unblock · e extend · esc clear")
	case m.status == "":
		return ""
	case m.isError:
		return errStyle.Render("✗ " + firstLine(m.status))
	default:
		return okStyle.Render("✓ ") + dimStyle.Render(firstLine(m.status))
	}
}

// ---------------------------------------------------------------- helpers

func (m model) timedCount() int {
	n := 0
	for _, e := range m.entries {
		if e.Timed() {
			n++
		}
	}
	return n
}

// parseDomains normalizes space-separated input. It returns the valid
// domains (deduplicated) and one error per invalid word.
func parseDomains(s string) ([]string, []error) {
	var domains []string
	var errs []error
	seen := map[string]bool{}
	for _, f := range strings.Fields(s) {
		d, err := hosts.Normalize(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !seen[d] {
			seen[d] = true
			domains = append(domains, d)
		}
	}
	return domains, errs
}

// preview shows the first n items and "…" if there are more.
func preview(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:n], ", ") + ", …"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
