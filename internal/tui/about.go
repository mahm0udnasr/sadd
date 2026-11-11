package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
)

// aboutMarkdown builds the About page as Markdown; Glamour turns it into
// styled terminal text (headings, tables, code spans).
func (m model) aboutMarkdown() string {
	var b strings.Builder
	b.WriteString("# sadd\n\nBlock distracting websites by managing a small, marked section of `/etc/hosts`.\n\n")

	b.WriteString("## Status\n\n")
	for _, r := range m.info {
		fmt.Fprintf(&b, "- **%s:** %s\n", r.Label, r.Value)
	}
	blocked := plural(len(m.entries), "site")
	if t := m.timedCount(); t > 0 {
		blocked += fmt.Sprintf(" (%d timed)", t)
	}
	fmt.Fprintf(&b, "- **Blocked:** %s\n", blocked)
	if e, ok := m.nextExpiry(); ok {
		fmt.Fprintf(&b, "- **Next unblock:** `%s`, %s\n", e.Domain, Remaining(e, m.now))
	}

	b.WriteString(`
## How it works

- Everything lives between ` + "`# >>> sadd start`" + ` and ` + "`# <<< sadd end`" + `. Lines outside are never touched.
- Each site is blocked for IPv4 and IPv6, with and without ` + "`www.`" + `.
- Writes are atomic (temp file, then rename) and a backup is saved first. **z** undoes the last change.
- Timed blocks store their end time in the file; a systemd timer removes them, even after a reboot.
- Still loads after blocking? Restart the browser (open tabs keep their connections) and turn off its "Secure DNS". ` + "`sadd check`" + ` shows what the system really resolves.
- Subdomains are not covered automatically: block ` + "`m.youtube.com`" + ` too, or use a group.

## Command line

` + "```sh" + `
sudo sadd block youtube.com --for 2h
sudo sadd block --group social
sadd list
sadd check facebook.com
sudo sadd restore
` + "```" + `
`)
	return b.String()
}

// renderAbout renders the Markdown for the current width into the viewport.
func (m *model) renderAbout() {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(m.markdownStyle),
		glamour.WithWordWrap(max(m.about.Width-2, 20)),
	)
	if err != nil {
		m.about.SetContent(m.aboutMarkdown())
		return
	}
	out, err := r.Render(m.aboutMarkdown())
	if err != nil {
		out = m.aboutMarkdown()
	}
	m.about.SetContent(out)
}
