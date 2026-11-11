// Package hosts reads and splits a hosts file into the part sadd owns
// (the lines between the markers) and the parts it must never touch.
package hosts

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	// StartMarker and EndMarker surround the section that sadd manages.
	StartMarker = "# >>> sadd start"
	EndMarker   = "# <<< sadd end"

	// DefaultPath is the real system hosts file.
	DefaultPath = "/etc/hosts"

	// PathEnv lets you point sadd at a fake file for testing.
	PathEnv = "HOSTS_FILE"
)

// Errors for a damaged marker layout. They are package-level variables so
// callers (and tests) can check them with errors.Is.
var (
	ErrMissingEnd    = errors.New("found start marker without end marker")
	ErrMissingStart  = errors.New("found end marker without start marker")
	ErrDuplicateMark = errors.New("found a marker more than once")
)

// File is a hosts file split into three parts.
// Before and After are kept byte-for-byte so they can be written back
// unchanged. Block holds the raw lines between the markers (markers excluded).
type File struct {
	Before []string
	Block  []string
	After  []string

	// HasBlock reports whether the markers existed when the file was parsed.
	// It is false on the first run; Render then appends a new block at the
	// end of the file. Only Parse sets it.
	HasBlock bool

	// lists are the blocklist sections (see lists.go). Render writes them
	// right after the normal block.
	lists []List
}

// Path returns the hosts file to use: $HOSTS_FILE if set, else /etc/hosts.
func Path() string {
	if p := os.Getenv(PathEnv); p != "" {
		return p
	}
	return DefaultPath
}

// Load reads the file at path and parses it.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read hosts file: %w", err)
	}
	f, err := Parse(string(data))
	if err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

// Parse splits content into Before, Block and After, and pulls out the
// blocklist sections. It does no I/O, which makes it easy to test.
func Parse(content string) (File, error) {
	lines, lists, err := extractLists(splitLines(content))
	if err != nil {
		return File{}, err
	}
	f, err := parseBlock(lines)
	f.lists = lists
	return f, err
}

// parseBlock finds the normal sadd block between the markers.
func parseBlock(lines []string) (File, error) {

	start, end := -1, -1
	for i, line := range lines {
		// Trim so "  # >>> sadd start  " (or a stray \r) still matches.
		switch strings.TrimSpace(line) {
		case StartMarker:
			if start != -1 {
				return File{}, fmt.Errorf("line %d: %w", i+1, ErrDuplicateMark)
			}
			start = i
		case EndMarker:
			if end != -1 {
				return File{}, fmt.Errorf("line %d: %w", i+1, ErrDuplicateMark)
			}
			if start == -1 {
				// End seen before any start: the order is wrong.
				return File{}, fmt.Errorf("line %d: %w", i+1, ErrMissingStart)
			}
			end = i
		}
	}

	switch {
	case start == -1 && end == -1:
		// No markers: the whole file belongs to the user.
		return File{Before: lines}, nil
	case end == -1:
		return File{}, fmt.Errorf("line %d: %w", start+1, ErrMissingEnd)
	}

	return File{
		Before:   lines[:start],
		Block:    lines[start+1 : end],
		After:    lines[end+1:],
		HasBlock: true,
	}, nil
}

// splitLines splits on "\n" without producing a fake empty last line.
// strings.Split("a\nb\n", "\n") returns ["a" "b" ""]; that trailing ""
// is just the final newline, not a real line.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}
