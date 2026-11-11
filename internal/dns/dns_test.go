package dns

import (
	"fmt"
	"testing"
)

func TestParseGetent(t *testing.T) {
	out := []byte(`0.0.0.0         STREAM facebook.com
0.0.0.0         DGRAM
0.0.0.0         RAW
::              STREAM
::              DGRAM
garbage line
`)
	got := fmt.Sprint(parseGetent(out))
	if got != "[0.0.0.0 ::]" {
		t.Errorf("parseGetent = %s, want [0.0.0.0 ::]", got)
	}
	if ips := parseGetent(nil); len(ips) != 0 {
		t.Errorf("empty output gave %v", ips)
	}
}
