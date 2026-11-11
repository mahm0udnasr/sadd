package cmd

import (
	"fmt"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/timer"
	"github.com/spf13/cobra"
)

var expireReschedule bool

// expireCmd is what the systemd timers run. It is hidden from help because
// users normally never need it; every write command expires entries too.
var expireCmd = &cobra.Command{
	Use:   "expire",
	Short: "Remove timed blocks whose time is up",
	Long: `Remove timed blocks whose time is up.

With --reschedule (run at boot by sadd-expire.service), also create a
new timer for every timed block that has not ended yet: systemd timers
created by "block --for" are lost when the computer restarts.`,
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := update(nil) // nil change: only the expiry step runs
		if err != nil {
			return err
		}
		printWarning(r.Warning)
		if len(r.Expired) == 0 {
			fmt.Println("Nothing expired.")
		} else {
			printList("Expired:", r.Expired)
		}

		if expireReschedule {
			return reschedule()
		}
		return nil
	},
}

// reschedule creates one timer per distinct end time still in the future.
// A duplicate timer (if an old one survived) is harmless: expire only
// removes what has really expired, so running it twice changes nothing.
func reschedule() error {
	f, err := hosts.Load(hosts.Path())
	if err != nil {
		return err
	}

	now := time.Now()
	seen := make(map[int64]bool)
	var pending []time.Time
	add := func(at time.Time) {
		if at.After(now) && !seen[at.Unix()] {
			seen[at.Unix()] = true
			pending = append(pending, at)
		}
	}
	for _, e := range f.Entries() {
		if e.Timed() {
			add(e.Until)
		}
	}
	for _, l := range f.Lists() { // blocklists in their turn-off cooldown
		if l.TurningOff() {
			add(l.OffAt)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	exe, err := timer.Executable()
	if err != nil {
		return err
	}
	for _, at := range pending {
		unit, err := scheduleExpire(exe, at)
		if err != nil {
			return err
		}
		fmt.Printf("Auto-unblock scheduled (systemd timer %s).\n", unit)
	}
	return nil
}

func init() {
	expireCmd.Flags().BoolVar(&expireReschedule, "reschedule", false,
		"also re-create timers for timed blocks that have not ended (used at boot)")
	rootCmd.AddCommand(expireCmd)
}
