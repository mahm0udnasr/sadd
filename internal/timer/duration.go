package timer

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MinDuration is the shortest timed block allowed.
const MinDuration = time.Minute

// ParseDuration is time.ParseDuration plus a "d" (day) unit:
// "90m", "2h", "1h30m", "1d", "2d".
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	var d time.Duration
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: use e.g. 30m, 2h, 1h30m, 1d", s)
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(s); err != nil {
			return 0, fmt.Errorf("invalid duration %q: use e.g. 30m, 2h, 1h30m, 1d", s)
		}
	}

	if d < MinDuration {
		return 0, fmt.Errorf("duration %q is too short (minimum %s)", s, MinDuration)
	}
	return d, nil
}
