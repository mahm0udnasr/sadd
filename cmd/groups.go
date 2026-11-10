package cmd

import (
	"fmt"
	"strings"

	"github.com/mahm0udnasr/sadd/internal/groups"
	"github.com/spf13/cobra"
)

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "Show the built-in groups of sites",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		for _, name := range groups.Names() {
			domains, _ := groups.Get(name)
			fmt.Printf("%-9s %s\n", name, strings.Join(domains, " "))
		}
	},
}

func init() {
	rootCmd.AddCommand(groupsCmd)
}
