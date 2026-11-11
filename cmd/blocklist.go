package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mahm0udnasr/sadd/internal/blocklist"
	"github.com/mahm0udnasr/sadd/internal/hosts"
	"github.com/mahm0udnasr/sadd/internal/timer"
	"github.com/mahm0udnasr/sadd/internal/tui"
	"github.com/spf13/cobra"
)

// listCooldown is how long "off" waits before the list is really removed.
// Turning on is instant; turning off is slow on purpose: the urge passes,
// the cooldown stays. Turning it on again during the wait cancels it.
const listCooldown = 10 * time.Minute

// scheduleListOff makes systemd run "sadd expire" when the cooldown ends.
// It is a variable so tests do not create real systemd timers.
var scheduleListOff = func(at time.Time) error {
	exe, err := timer.Executable()
	if err != nil {
		return err
	}
	_, err = scheduleExpire(exe, at)
	return err
}

var blocklistCmd = &cobra.Command{
	Use:   "blocklist",
	Short: "Turn big ready-made blocklists on or off (e.g. porn)",
	Long: `Blocklists are big lists of domains downloaded from the internet,
kept in their own section of /etc/hosts. With no subcommand, shows them.

Turning a list on is instant. Turning it off waits ` + tui.ShortDuration(listCooldown) + `
first; turning it on again during that time cancels.`,
	Example: `  sadd blocklist
  sudo sadd blocklist on porn
  sudo sadd blocklist off porn
  sudo sadd blocklist update`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		lists, err := blocklistStatus()
		if err != nil {
			return err
		}
		for _, l := range lists {
			fmt.Printf("%-8s %s\n", l.Name, describeList(l))
		}
		return nil
	},
}

var blocklistOnCmd = &cobra.Command{
	Use:       "on <name>",
	Short:     "Download a blocklist and turn it on (or cancel turning it off)",
	Args:      cobra.ExactArgs(1),
	ValidArgs: blocklistNames(),
	RunE: func(cmd *cobra.Command, args []string) error {
		return printResult(blocklistOn(args[0]))
	},
}

var blocklistOffCmd = &cobra.Command{
	Use:       "off <name>",
	Short:     "Turn a blocklist off, after a " + tui.ShortDuration(listCooldown) + " cooldown",
	Args:      cobra.ExactArgs(1),
	ValidArgs: blocklistNames(),
	RunE: func(cmd *cobra.Command, args []string) error {
		return printResult(blocklistOff(args[0]))
	},
}

var blocklistUpdateCmd = &cobra.Command{
	Use:   "update [name]...",
	Short: "Download the newest version of the blocklists that are on",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Downloading…")
		return printResult(blocklistUpdate(args))
	},
}

func printResult(msg string, err error) error {
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}

// ---------------------------------------------------------------- shared
// The CLI commands above and the TUI both call these.

// blocklistStatus lists every known blocklist, on or off.
func blocklistStatus() ([]tui.Blocklist, error) {
	f, err := hosts.Load(hosts.Path())
	if err != nil {
		return nil, err
	}
	var out []tui.Blocklist
	for _, src := range blocklist.All() {
		b := tui.Blocklist{Name: src.Name, Title: src.Title, About: src.About}
		if info, ok := f.List(src.Name); ok {
			b.On, b.Domains, b.Downloaded, b.OffAt = true, info.Domains, info.Downloaded, info.OffAt
		}
		out = append(out, b)
	}
	return out, nil
}

func describeList(l tui.Blocklist) string {
	switch {
	case !l.On:
		return "off  (" + l.About + ")"
	case !l.OffAt.IsZero():
		return fmt.Sprintf("on, turning off at %s (in %s)", l.OffAt.Format("15:04"), tui.ShortDuration(time.Until(l.OffAt)))
	default:
		return fmt.Sprintf("on   %s sites, downloaded %s", tui.Thousands(l.Domains), l.Downloaded.Format("Mon Jan 2 15:04"))
	}
}

// blocklistOn turns a list on. If it is already turning off, it only
// cancels that (no download needed: the list is still there).
func blocklistOn(name string) (string, error) {
	src, err := blocklist.Get(name)
	if err != nil {
		return "", err
	}
	f, err := hosts.Load(hosts.Path())
	if err != nil {
		return "", err
	}
	if info, ok := f.List(name); ok {
		if !info.TurningOff() {
			return fmt.Sprintf("The %s blocklist is already on (%s sites). For the newest version: sadd blocklist update",
				name, tui.Thousands(info.Domains)), nil
		}
		r, err := update(func(f *hosts.File) []string {
			f.SetListOffAt(name, time.Time{})
			return []string{name}
		})
		if err != nil {
			return "", err
		}
		return "Cancelled: the " + name + " blocklist stays on." + extras(r, nil), nil
	}

	n, r, err := installList(src)
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("The %s blocklist is on: %s sites blocked.", name, tui.Thousands(n))
	if hosts.Path() == hosts.DefaultPath {
		msg += " Still loads? Restart the browser."
	}
	return msg + extras(r, nil), nil
}

// blocklistOff starts the cooldown. The list is removed by "sadd expire"
// (the timer), or by any sadd write command after the time has passed.
func blocklistOff(name string) (string, error) {
	if _, err := blocklist.Get(name); err != nil {
		return "", err
	}
	f, err := hosts.Load(hosts.Path())
	if err != nil {
		return "", err
	}
	info, ok := f.List(name)
	switch {
	case !ok:
		return "The " + name + " blocklist is not on.", nil
	case info.TurningOff():
		return fmt.Sprintf("The %s blocklist is already turning off at %s.", name, info.OffAt.Format("15:04")), nil
	}

	at := time.Now().Add(listCooldown).Truncate(time.Second)
	r, err := update(func(f *hosts.File) []string {
		f.SetListOffAt(name, at)
		return []string{name}
	})
	if err != nil {
		return "", err
	}
	var timerErr error
	if err := scheduleListOff(at); err != nil {
		// Safe direction: the list just stays on longer. Any sadd write
		// command after that time removes it.
		timerErr = fmt.Errorf("the turn-off timer failed (%v); it turns off with the next sadd command after %s",
			err, at.Format("15:04"))
	}
	msg := fmt.Sprintf("The %s blocklist turns off at %s (in %s). Changed your mind? Turn it on again before then.",
		name, at.Format("15:04"), tui.ShortDuration(listCooldown))
	return msg + extras(r, timerErr), nil
}

// blocklistUpdate downloads a fresh copy of the named lists, or of every
// list that is on. Lists in their turn-off cooldown are left alone:
// an update must not quietly cancel a turn-off.
func blocklistUpdate(names []string) (string, error) {
	f, err := hosts.Load(hosts.Path())
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		for _, l := range f.Lists() {
			names = append(names, l.Name)
		}
		if len(names) == 0 {
			return "No blocklist is on.", nil
		}
	}

	var done []string
	for _, name := range names {
		src, err := blocklist.Get(name)
		if err != nil {
			return "", err
		}
		info, ok := f.List(name)
		switch {
		case !ok:
			done = append(done, name+" is off (turn it on first)")
			continue
		case info.TurningOff():
			done = append(done, name+" is turning off, not updated")
			continue
		}
		n, _, err := installList(src)
		if err != nil {
			return "", err
		}
		done = append(done, fmt.Sprintf("%s: %s sites (%+d)", name, tui.Thousands(n), n-info.Domains))
	}
	return "Updated " + strings.Join(done, "; ") + ".", nil
}

// installList downloads src and writes it as one change (one backup).
func installList(src blocklist.Source) (int, result, error) {
	// Check permissions BEFORE downloading 2 MB for nothing.
	if err := requireRoot(hosts.Path(), os.Geteuid()); err != nil {
		return 0, result{}, err
	}
	domains, err := blocklist.Download(context.Background(), src)
	if err != nil {
		return 0, result{}, err
	}
	r, err := update(func(f *hosts.File) []string {
		f.SetList(src.Name, src.URL, time.Now().Truncate(time.Second), domains)
		return []string{src.Name}
	})
	return len(domains), r, err
}

func blocklistNames() []string {
	var names []string
	for _, s := range blocklist.All() {
		names = append(names, s.Name)
	}
	return names
}

func init() {
	blocklistCmd.AddCommand(blocklistOnCmd, blocklistOffCmd, blocklistUpdateCmd)
	rootCmd.AddCommand(blocklistCmd)
}
