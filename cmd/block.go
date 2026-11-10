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

		// Parse --for before touching the file, so a typo writes nothing.
		var until time.Time // zero = forever
		var exe string
		if blockFor != "" {
			d, err := timer.ParseDuration(blockFor)
			if err != nil {
				return err
			}
			if exe, err = timer.Executable(); err != nil {
				return err
			}
			// Whole seconds: the file stores RFC3339, which has no fractions.
			until = time.Now().Add(d).Truncate(time.Second)
		}

		r, err := update(func(f *hosts.File) []string {
			return f.AddUntil(until, domains...)
		})
		if err != nil {
			return err
		}

		printList("Expired:", r.Expired)
		printWarning(r.Warning)
		if len(r.Changed) == 0 {
			printList("Already blocked:", domains)
			return nil
		}

		if until.IsZero() {
			printList("Blocked:", r.Changed)
			return nil
		}
		printList(fmt.Sprintf("Blocked until %s:", until.Format("Mon 15:04")), r.Changed)
		return scheduleExpire(exe, until)
	},
}

// scheduleExpire creates the systemd timer. The block is already saved, so
// a failure here is reported but the sites stay blocked; any later sadd
// write command removes them once the time has passed.
func scheduleExpire(exe string, at time.Time) error {
	opts := timer.Options{At: at, Exe: exe, User: os.Geteuid() != 0}
	if p := hosts.Path(); p != hosts.DefaultPath {
		abs, err := filepath.Abs(p) // the timer runs in another directory
		if err != nil {
			return err
		}
		opts.HostsFile = abs
	}

	unit, err := timer.ScheduleExpire(opts)
	if err != nil {
		return fmt.Errorf("blocked, but the auto-unblock timer failed: %w\n"+
			"Unblock manually later with: sudo sadd unblock ...", err)
	}
	fmt.Printf("Auto-unblock scheduled (systemd timer %s).\n", unit)
	return nil
}

func init() {
	blockCmd.Flags().StringSliceVarP(&blockGroups, "group", "g", nil,
		"block a group of sites (repeatable, see 'sadd groups')")
	blockCmd.Flags().StringVar(&blockFor, "for", "",
		"unblock automatically after this long (e.g. 30m, 2h, 1d)")
	rootCmd.AddCommand(blockCmd)
}
