package cmd

import (
	"fmt"
	"time"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/tui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Show blocked websites",
	Args:    cobra.NoArgs,
	// RunE (not Run) lets us return an error; Cobra prints it and
	// Execute exits with code 1.
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := hosts.Load(hosts.Path())
		if err != nil {
			return err
		}

		entries := f.Entries()
		if len(entries) == 0 {
			fmt.Println("No websites are blocked.")
			return nil
		}

		width := 0
		for _, e := range entries {
			width = max(width, len(e.Domain))
		}
		now := time.Now()
		for _, e := range entries {
			fmt.Printf("%-*s  %s\n", width, e.Domain, tui.Describe(e, now))
		}
		return nil
	},
}

// init runs automatically when the package loads; it attaches the
// subcommand to the root. Every command file follows this pattern.
func init() {
	rootCmd.AddCommand(listCmd)
}
