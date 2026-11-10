package cmd

import (
	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/spf13/cobra"
)

var unblockGroups []string

var unblockCmd = &cobra.Command{
	Use:   "unblock [domain]...",
	Short: "Unblock websites",
	Example: `  sudo sadd unblock youtube.com
  sudo sadd unblock --group social`,
	RunE: func(cmd *cobra.Command, args []string) error {
		domains, err := collectDomains(args, unblockGroups)
		if err != nil {
			return err
		}

		r, err := update(func(f *hosts.File) []string {
			return f.Remove(domains...)
		})
		if err != nil {
			return err
		}

		printList("Expired:", r.Expired)
		printWarning(r.Warning)
		if len(r.Changed) == 0 {
			printList("Not blocked:", domains)
			return nil
		}
		printList("Unblocked:", r.Changed)
		return nil
	},
}

func init() {
	unblockCmd.Flags().StringSliceVarP(&unblockGroups, "group", "g", nil,
		"unblock a group of sites (repeatable, see 'sadd groups')")
	rootCmd.AddCommand(unblockCmd)
}
