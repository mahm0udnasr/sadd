// Package timer schedules "sadd expire" with a systemd transient timer.
package timer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options describe one scheduled run.
type Options struct {
	At        time.Time
	Exe       string // absolute path of sadd, from Executable()
	HostsFile string // passed to the job as HOSTS_FILE when not /etc/hosts
	User      bool   // use the user's systemd (no root), for test files
}

// ScheduleExpire asks systemd to run "<this binary> expire" at opts.At.
//
// It runs "sadd expire", NOT "sadd unblock <domain>": expire reads the end
// times stored in the hosts file, so if the user changes their mind in the
// meantime (unblocks, or blocks forever), the timer does the right thing.
//
// Limitation: transient timers do not survive a reboot. Any later sadd
// write command removes expired entries too, so nothing stays stuck.
func ScheduleExpire(opts Options) (unit string, err error) {
	unit = fmt.Sprintf("sadd-expire-%s-%d", opts.At.Format("20060102-150405"), os.Getpid())
	args := []string{
		"--unit=" + unit,
		"--description=sadd: unblock expired websites",
		"--on-calendar=" + opts.At.Format("2006-01-02 15:04:05"),
		// Default timer accuracy is 1 minute; we want it on time.
		"--timer-property=AccuracySec=1s",
		"--collect", // clean up the unit after it ran
	}
	if opts.User {
		args = append([]string{"--user"}, args...)
	}
	if opts.HostsFile != "" {
		args = append(args, "--setenv=HOSTS_FILE="+opts.HostsFile)
	}
	args = append(args, opts.Exe, "expire")

	out, err := exec.Command("systemd-run", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("systemd-run: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return unit, nil
}

// Executable returns the absolute path of the running sadd binary.
// "go run" builds into a temp folder that is deleted on exit, so a timer
// pointing there would fail later. Call it BEFORE changing the hosts file,
// so a timed block is never written without a timer to end it.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find sadd binary: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", fmt.Errorf("find sadd binary: %w", err)
	}
	if strings.Contains(exe, "go-build") {
		return "", errors.New("timed blocks need a built binary (go run's binary is deleted on exit).\n" +
			"Build it first: go build -o sadd . && sudo ./sadd block ... --for 2h")
	}
	return exe, nil
}
