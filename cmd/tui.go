package cmd

import (
	"fmt"
	"strings"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive screen to view, block and unblock websites",
	Long: `Open an interactive screen. Viewing works without sudo;
blocking and unblocking /etc/hosts need: sudo sadd tui`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Wire the TUI to the same update() the CLI commands use, so both
		// follow exactly the same rules (normalize, expire, backup, flush).
		return tui.Run(tui.Backend{
			Load: func() ([]hosts.Entry, error) {
				f, err := hosts.Load(hosts.Path())
				return f.Entries(), err
			},
			Block: func(input string) (string, error) {
				domain, err := hosts.Normalize(input)
				if err != nil {
					return "", err
				}
				r, err := update(func(f *hosts.File) []string { return f.Add(domain) })
				return describeResult("Blocked", "Already blocked", domain, r), err
			},
			Unblock: func(domain string) (string, error) {
				r, err := update(func(f *hosts.File) []string { return f.Remove(domain) })
				return describeResult("Unblocked", "Not blocked", domain, r), err
			},
		})
	},
}

func describeResult(done, noop, domain string, r result) string {
	msg := done + " " + domain
	if len(r.Changed) == 0 {
		msg = noop + ": " + domain
	}
	if len(r.Expired) > 0 {
		msg += fmt.Sprintf(" (expired: %s)", strings.Join(r.Expired, ", "))
	}
	if r.Warning != nil {
		msg += " · warning: " + r.Warning.Error()
	}
	return msg
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
