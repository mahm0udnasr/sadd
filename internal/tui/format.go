package tui

import (
	"fmt"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// Describe says how long an entry stays blocked, e.g.
// "forever", "until Tue 23:15 (1h59m left)", "expired".
// The CLI "list" command uses it too, so both show the same text.
func Describe(e hosts.Entry, now time.Time) string {
	switch {
	case !e.Timed():
		return "forever"
	case e.Expired(now):
		return "expired (removed on next change)"
	default:
		return fmt.Sprintf("until %s (%s left)", e.Until.Format("Mon 15:04"), ShortDuration(e.Until.Sub(now)))
	}
}

// Remaining is a compact form for the TUI: "1h35m left · until Wed 01:07".
func Remaining(e hosts.Entry, now time.Time) string {
	if e.Expired(now) {
		return "expired"
	}
	return ShortDuration(e.Until.Sub(now)) + " left · until " + e.Until.Format("Mon 15:04")
}

// ShortDuration rounds to minutes (exported: the CLI uses it too): "1h59m", "45m", "2d3h", "<1m".
func ShortDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	d = d.Round(time.Minute)
	days := int(d / (24 * time.Hour))
	h := int(d/time.Hour) % 24
	m := int(d/time.Minute) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, h)
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
