package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive screen to view, block and unblock websites",
	Long: `Open an interactive, tabbed screen. Running plain "sadd" does the same.
Viewing works without sudo; blocking and unblocking /etc/hosts need: sudo sadd`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return runTUI() },
}

// runTUI opens the interactive screen. Both "sadd" and "sadd tui" call it.
func runTUI() error {
	path := hosts.Path()

	// Wire the TUI to the same functions the CLI commands use (update,
	// doBlock), so both follow exactly the same rules.
	return tui.Run(tui.Backend{
		CanWrite:     canWrite(path),
		ReadOnlyHint: "Run 'sudo sadd' to make changes.",

		Load: func() ([]hosts.Entry, error) {
			f, err := hosts.Load(path)
			return f.Entries(), err
		},
		Block:   tuiBlock,
		Extend:  tuiExtend,
		Unblock: tuiUnblock,
		Restore: func() (string, error) { return tuiRestore(path) },

		Blocklists:   blocklistStatus,
		SetBlocklist: tuiBlocklist,
		ListCooldown: listCooldown,
	})
}

// isTerminal reports whether f is an interactive terminal (not a pipe or
// a file), so "sadd | less" or a script gets the help text, not a TUI.
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func tuiBlock(req tui.BlockRequest) (string, error) {
	out, err := doBlock(req.Domains, req.For)
	if err != nil {
		return "", err
	}

	var msg string
	switch {
	case len(out.Changed) == 0:
		msg = "Already blocked."
	case out.Until.IsZero():
		msg = "Blocked " + summary(out.Changed) + "."
	default:
		msg = fmt.Sprintf("Blocked %s until %s.", summary(out.Changed), out.Until.Format("Mon 15:04"))
	}
	if len(out.Changed) > 0 && hosts.Path() == hosts.DefaultPath {
		msg += " Still loads? Restart the browser."
	}
	return msg + extras(out.result, out.TimerErr), nil
}

func tuiExtend(domains []string, by time.Duration) (string, error) {
	out, err := doExtend(domains, by)
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("Extended %s by %s.", summary(out.Changed), tui.ShortDuration(by))
	if len(out.Changed) == 0 {
		msg = "Nothing to extend."
	}
	if len(out.Skipped) > 0 {
		msg += " Skipped (not timed): " + strings.Join(out.Skipped, ", ") + "."
	}
	return msg + extras(out.result, out.TimerErr), nil
}

func tuiBlocklist(name string, a tui.BlocklistAction) (string, error) {
	switch a {
	case tui.BlocklistOn:
		return blocklistOn(name)
	case tui.BlocklistOff:
		return blocklistOff(name)
	case tui.BlocklistUpdate:
		return blocklistUpdate([]string{name})
	}
	return "", nil
}

func tuiUnblock(domains []string) (string, error) {
	r, err := update(func(f *hosts.File) []string { return f.Remove(domains...) })
	if err != nil {
		return "", err
	}
	msg := "Unblocked " + summary(r.Changed) + "."
	if len(r.Changed) == 0 {
		msg = "Nothing to unblock."
	}
	return msg + extras(r, nil), nil
}

func tuiRestore(path string) (string, error) {
	if err := hosts.Restore(path); err != nil {
		return "", friendlyPermError(path, err)
	}
	msg := "Restored the previous version. Press z again to redo."
	if err := flushDNS(path); err != nil {
		msg += " Warning: " + err.Error()
	}
	return msg, nil
}

// summary names one site, or counts several.
func summary(domains []string) string {
	if len(domains) == 1 {
		return domains[0]
	}
	return fmt.Sprintf("%d sites", len(domains))
}

// extras appends expired entries and warnings to a status message.
func extras(r result, timerErr error) string {
	var s string
	if len(r.Expired) > 0 {
		s += " Expired: " + strings.Join(r.Expired, ", ") + "."
	}
	for _, w := range []error{r.Warning, timerErr} {
		if w != nil {
			s += " Warning: " + w.Error()
		}
	}
	return s
}

// canWrite reports whether sadd could save changes to path right now.
// Atomic writes create a temp file and rename it, so the FOLDER must be
// writable too, not just the file.
func canWrite(path string) bool {
	if requireRoot(path, os.Geteuid()) != nil {
		return false
	}
	const wOK = 2 // W_OK from <unistd.h>
	return syscall.Access(path, wOK) == nil && syscall.Access(filepath.Dir(path), wOK) == nil
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
