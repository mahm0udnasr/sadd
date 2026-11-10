package cmd

import (
	"fmt"
	"os"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Swap the hosts file with its last backup (run again to undo)",
	Long: `Restore puts back the hosts file as it was before the last change.

The current file becomes the new backup, so running restore twice
returns to where you started.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := hosts.Path()
		if err := requireRoot(path, os.Geteuid()); err != nil {
			return err
		}
		if err := hosts.Restore(path); err != nil {
			return friendlyPermError(path, err)
		}
		printWarning(flushDNS(path))

		fmt.Printf("Restored %s from %s\n", path, hosts.BackupPath(path))
		fmt.Println("Run 'sadd restore' again to undo.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
