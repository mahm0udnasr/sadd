// Package cmd holds the Cobra commands. It only parses input and prints
// output; the real work lives in internal/hosts.
package cmd

import (
	"fmt"
	"os"

	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/spf13/cobra"
)

// version is set at build time by the Makefile:
//
//	go build -ldflags "-X github.com/mahm0udnasr/sadd/cmd.version=v1.0.0"
//
// It must be a package-level string variable (not a const) for -X to work.
var version = "dev"

// rootCmd is the top of the command tree ("sadd").
// Subcommands (block, unblock, list) attach to it with rootCmd.AddCommand.
// Lowercase name: nothing outside this package needs to touch it.
var rootCmd = &cobra.Command{
	Use:   "sadd",
	Short: "Block and unblock websites via /etc/hosts",
	Long: `sadd blocks websites by managing its own section of /etc/hosts,
between "# >>> sadd start" and "# <<< sadd end".
Lines outside that section are never modified.

Run "sadd" with no command to open the interactive screen
(use "sudo sadd" to make changes there).`,
	Version: version, // enables "sadd --version"

	// Do not print the usage text on every runtime error (e.g. permission
	// denied); usage is only helpful for wrong arguments.
	SilenceUsage: true,

	// "sadd" alone opens the TUI. Without a terminal (a pipe, a script)
	// a full-screen app makes no sense, so show the help instead.
	// No Args check here: Cobra itself reports "unknown command" for
	// "sadd foo", because the root command has subcommands.
	RunE: func(cmd *cobra.Command, args []string) error {
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			return cmd.Help()
		}
		return runTUI()
	},

	// Runs before every subcommand. When HOSTS_FILE points somewhere else,
	// say so on stderr, so you always know which file is being changed.
	// (Remember: sudo drops HOSTS_FILE unless you pass it explicitly.)
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if p := hosts.Path(); p != hosts.DefaultPath {
			fmt.Fprintf(os.Stderr, "(using %s=%s)\n", hosts.PathEnv, p)
		}
	},
}

// Execute runs the root command. It is exported (capital E) because
// main.go, a different package, must call it.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already printed the error; just exit with a failure code
		// so shell scripts can detect it.
		os.Exit(1)
	}
}
