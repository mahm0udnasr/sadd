package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A dialog is a Huh form drawn in a bordered box in the middle of the
// screen, on top of the normal screen, which is dimmed behind it.
//
// Lip Gloss v1 has no layers, so overlay() does it by hand, line by line:
//
//	screen line:  "  youtube.com          Always blocked     "
//	dialog line:             "│ Unblock?  │"
//	result:       "  youtube.c│ Unblock?  │blocked     "
//	              <-- left --><-- dialog --><-- right -->
//
// Both sides are cut by display width (not bytes: "◷" is 3 bytes but 1
// column), with the colors stripped so the background looks dim.

// dialogStyle is the box around the form.
var dialogStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(accent).
	Padding(1, 2)

// dialogWidth is the outer width of the box, border included.
func (m model) dialogWidth() int { return max(min(m.width-4, 64), 30) }

// formWidth is the room inside the box: border (2) + padding (4).
func (m model) formWidth() int { return m.dialogWidth() - 6 }

// dialogView is the open form in its box, never taller than the screen.
func (m model) dialogView() string {
	box := dialogStyle.Width(m.dialogWidth() - 2).Render(m.form.View()) // Width excludes the border
	return lipgloss.NewStyle().MaxHeight(m.height).Render(box)
}

// overlay draws fg centered on bg, which is width x height cells.
// Every bg cell outside fg is dimmed.
func overlay(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	fgLines := strings.Split(fg, "\n")
	fgWidth := lipgloss.Width(fg)

	x := max((width-fgWidth)/2, 0)
	y := max((height-len(fgLines))/2, 0)

	for i, line := range bgLines {
		plain := ansi.Strip(line)
		j := i - y // which dialog line lands on this screen line
		if j < 0 || j >= len(fgLines) {
			bgLines[i] = shadeStyle.Render(plain)
			continue
		}
		left := ansi.Truncate(plain, x, "")
		left += strings.Repeat(" ", x-ansi.StringWidth(left)) // short line: pad
		right := ansi.TruncateLeft(plain, x+fgWidth, "")
		bgLines[i] = shadeStyle.Render(left) + fgLines[j] + shadeStyle.Render(right)
	}
	return strings.Join(bgLines, "\n")
}
