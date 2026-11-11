package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mahm0udnasr/sadd/internal/blocklist"
	"github.com/mahm0udnasr/sadd/internal/hosts"
)

// fakeList serves a list of n domains instead of the real StevenBlack
// download, and replaces the systemd timer with a recorder.
func fakeList(t *testing.T, n int) (timers *[]time.Time) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# test list\n6.6.6.6 mybank.com") // a redirect: must be ignored
		for i := range n {
			fmt.Fprintf(w, "0.0.0.0 site%d.com\n", i)
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(blocklist.SetURLForTest("porn", srv.URL))

	var got []time.Time
	old := scheduleListOff
	scheduleListOff = func(at time.Time) error { got = append(got, at); return nil }
	t.Cleanup(func() { scheduleListOff = old })
	return &got
}

func listInfo(t *testing.T, path string) (hosts.ListInfo, bool) {
	t.Helper()
	f, err := hosts.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f.List("porn")
}

func TestBlocklistOnOffCancel(t *testing.T) {
	path := tempHosts(t)
	timers := fakeList(t, 150)

	msg, err := blocklistOn("porn")
	if err != nil || !strings.Contains(msg, "150 sites") {
		t.Fatalf("on: %q, %v", msg, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "mybank") || !strings.Contains(string(data), ":: site149.com") {
		t.Fatal("hosts file content is wrong")
	}
	if len(domainsIn(t, path)) != 0 {
		t.Error("blocklist leaked into the normal entries")
	}

	// Off only starts the cooldown; the list is still there.
	if msg, err = blocklistOff("porn"); err != nil || !strings.Contains(msg, "turns off at") {
		t.Fatalf("off: %q, %v", msg, err)
	}
	info, ok := listInfo(t, path)
	if !ok || !info.TurningOff() || len(*timers) != 1 {
		t.Fatalf("after off: %+v on=%v timers=%v", info, ok, *timers)
	}
	if d := time.Until(info.OffAt); d < listCooldown-time.Minute || d > listCooldown {
		t.Errorf("turns off in %v, want about %v", d, listCooldown)
	}

	// On again during the cooldown cancels it.
	if msg, _ = blocklistOn("porn"); !strings.Contains(msg, "stays on") {
		t.Errorf("cancel: %q", msg)
	}
	if info, _ := listInfo(t, path); info.TurningOff() {
		t.Error("still turning off")
	}
}

func TestBlocklistRemovedAfterCooldown(t *testing.T) {
	path := tempHosts(t)
	fakeList(t, 150)
	blocklistOn("porn")

	// Pretend the cooldown is over, then run what the timer runs.
	f, _ := hosts.Load(path)
	f.SetListOffAt("porn", time.Now().Add(-time.Second))
	hosts.Save(path, f)

	r, err := update(nil)
	if err != nil || len(r.Expired) != 1 {
		t.Fatalf("expire: %+v, %v", r, err)
	}
	if _, ok := listInfo(t, path); ok {
		t.Error("list still on after the cooldown")
	}
}

func TestBlocklistUpdate(t *testing.T) {
	tempHosts(t)
	fakeList(t, 150)
	if msg, _ := blocklistUpdate(nil); msg != "No blocklist is on." {
		t.Errorf("update with nothing on: %q", msg)
	}
	blocklistOn("porn")
	if msg, err := blocklistUpdate(nil); err != nil || !strings.Contains(msg, "porn: 150 sites (+0)") {
		t.Errorf("update: %q, %v", msg, err)
	}
	// An update must not cancel a turn-off.
	blocklistOff("porn")
	if msg, _ := blocklistUpdate(nil); !strings.Contains(msg, "not updated") {
		t.Errorf("update while turning off: %q", msg)
	}
}

func TestBlocklistUnknown(t *testing.T) {
	tempHosts(t)
	if _, err := blocklistOn("nope"); err == nil {
		t.Error("unknown list accepted")
	}
}
