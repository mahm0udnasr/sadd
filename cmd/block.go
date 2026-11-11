package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/timer"
	"github.com/spf13/cobra"
)

var (
	blockGroups []string
	blockFor    string
)

var blockCmd = &cobra.Command{
	Use:   "block [domain]...",
	Short: "Block websites (optionally for a limited time)",
	Example: `  sudo sadd block youtube.com https://www.reddit.com/r/golang
  sudo sadd block --group social
  sudo sadd block youtube.com --for 2h`,
	RunE: func(cmd *cobra.Command, args []string) error {
		domains, err := collectDomains(args, blockGroups)
		if err != nil {
			return err
		}

		var d time.Duration // 0 = forever
		if blockFor != "" {
			if d, err = timer.ParseDuration(blockFor); err != nil {
				return err
			}
		}

		out, err := doBlock(domains, d)
		if err != nil {
			return err
		}

		printList("Expired:", out.Expired)
		printWarning(out.Warning)
		if len(out.Changed) == 0 {
			printList("Already blocked:", domains)
			return nil
		}
		if out.Until.IsZero() {
			printList("Blocked:", out.Changed)
		} else {
			printList(fmt.Sprintf("Blocked until %s:", out.Until.Format("Mon 15:04")), out.Changed)
			if out.TimerErr != nil {
				return out.TimerErr
			}
			fmt.Printf("Auto-unblock scheduled (systemd timer %s).\n", out.Unit)
		}
		if hosts.Path() == hosts.DefaultPath {
			fmt.Println(blockHint(out.Changed))
		}
		return nil
	},
}

// blockOutcome is everything doBlock did, for the caller to display.
type blockOutcome struct {
	result
	Until    time.Time // zero = forever
	Unit     string    // systemd timer name, for timed blocks
	TimerErr error     // the block is saved, but the timer failed
}

// doBlock blocks already-normalized domains, forever (d == 0) or for d.
// The CLI "block" command and the TUI both call it, so they behave the same.
func doBlock(domains []string, d time.Duration) (blockOutcome, error) {
	var out blockOutcome
	var exe string

	// Check everything that can fail BEFORE writing, so a timed block is
	// never saved without a timer to end it.
	if d > 0 {
		var err error
		if exe, err = timer.Executable(); err != nil {
			return out, err
		}
		// Whole seconds: the file stores RFC3339, which has no fractions.
		out.Until = time.Now().Add(d).Truncate(time.Second)
	}

	r, err := update(func(f *hosts.File) []string {
		return f.AddUntil(out.Until, domains...)
	})
	if err != nil {
		return out, err
	}
	out.result = r

	if d > 0 && len(r.Changed) > 0 {
		out.Unit, out.TimerErr = scheduleExpire(exe, out.Until)
	}
	return out, nil
}

// extendOutcome is everything doExtend did, for the caller to display.
type extendOutcome struct {
	result
	Skipped  []string // not blocked, or blocked forever: nothing to extend
	TimerErr error
}

// doExtend adds `by` to the end time of timed blocks. Each site keeps its
// own clock: 1h left + 30m = 1h30m left. An entry that already expired
// (but was not removed yet) restarts from now.
//
// Everything happens in ONE update call, so it is one write, one backup,
// and one undo step, even for many sites.
func doExtend(domains []string, by time.Duration) (extendOutcome, error) {
	var out extendOutcome
	exe, err := timer.Executable() // fail before writing, like doBlock
	if err != nil {
		return out, err
	}

	now := time.Now()
	untils := map[time.Time]bool{} // one timer per distinct end time
	r, err := update(func(f *hosts.File) []string {
		current := make(map[string]hosts.Entry)
		for _, e := range f.Entries() {
			current[e.Domain] = e
		}
		var changed []string
		for _, d := range domains {
			e, ok := current[d]
			if !ok || !e.Timed() {
				out.Skipped = append(out.Skipped, d)
				continue
			}
			base := e.Until
			if base.Before(now) {
				base = now
			}
			until := base.Add(by).Truncate(time.Second)
			changed = append(changed, f.AddUntil(until, d)...)
			untils[until] = true
		}
		return changed
	})
	if err != nil {
		return out, err
	}
	out.result = r

	// The old timers stay; when they fire, "sadd expire" finds nothing
	// expired yet and does nothing. The new timers do the real work.
	for at := range untils {
		if _, err := scheduleExpire(exe, at); err != nil {
			out.TimerErr = err
		}
	}
	return out, nil
}

// scheduleExpire creates the systemd timer. The block is already saved, so
// a failure here is reported but the sites stay blocked; any later sadd
// write command removes them once the time has passed.
func scheduleExpire(exe string, at time.Time) (string, error) {
	opts := timer.Options{At: at, Exe: exe, User: os.Geteuid() != 0}
	if p := hosts.Path(); p != hosts.DefaultPath {
		abs, err := filepath.Abs(p) // the timer runs in another directory
		if err != nil {
			return "", err
		}
		opts.HostsFile = abs
	}

	unit, err := timer.ScheduleExpire(opts)
	if err != nil {
		return "", fmt.Errorf("blocked, but the auto-unblock timer failed: %w\n"+
			"Unblock manually later with: sudo sadd unblock ...", err)
	}
	return unit, nil
}

func init() {
	blockCmd.Flags().StringSliceVarP(&blockGroups, "group", "g", nil,
		"block a group of sites (repeatable, see 'sadd groups')")
	blockCmd.Flags().StringVar(&blockFor, "for", "",
		"unblock automatically after this long (e.g. 30m, 2h, 1d)")
	rootCmd.AddCommand(blockCmd)
}
