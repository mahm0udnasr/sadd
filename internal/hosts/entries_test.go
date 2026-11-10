package hosts

import (
	"reflect"
	"testing"
	"time"
)

func TestTimedEntries(t *testing.T) {
	now := time.Date(2026, 11, 10, 22, 0, 0, 0, time.UTC)
	in2h := now.Add(2 * time.Hour)

	var f File
	f.Add("reddit.com")
	if changed := f.AddUntil(in2h, "youtube.com"); !reflect.DeepEqual(changed, []string{"youtube.com"}) {
		t.Fatalf("AddUntil changed = %q", changed)
	}

	// Survives a render/parse round trip.
	f2, err := Parse(f.Render())
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{Domain: "reddit.com"}, {Domain: "youtube.com", Until: in2h}}
	got := f2.Entries()
	if len(got) != 2 || got[0] != want[0] || got[1].Domain != "youtube.com" || !got[1].Until.Equal(in2h) {
		t.Fatalf("entries = %+v, want %+v", got, want)
	}

	// Not expired yet.
	if removed := f2.Expire(now.Add(time.Hour)); removed != nil {
		t.Errorf("expired too early: %q", removed)
	}
	// Exactly at the end time: expired.
	if removed := f2.Expire(in2h); !reflect.DeepEqual(removed, []string{"youtube.com"}) {
		t.Errorf("Expire removed %q, want [youtube.com]", removed)
	}
	if got := f2.Domains(); !reflect.DeepEqual(got, []string{"reddit.com"}) {
		t.Errorf("after expire: %q", got)
	}
}

// Blocking forever after a timed block must clear the end time, so the
// pending timer no longer unblocks it. This is why the timer runs
// "sadd expire" instead of "sadd unblock <domain>".
func TestPermanentBlockOverridesTimer(t *testing.T) {
	now := time.Now()
	var f File
	f.AddUntil(now.Add(time.Hour), "youtube.com")

	if changed := f.Add("youtube.com"); !reflect.DeepEqual(changed, []string{"youtube.com"}) {
		t.Fatalf("Add changed = %q, want [youtube.com]", changed)
	}
	if removed := f.Expire(now.Add(2 * time.Hour)); removed != nil {
		t.Errorf("permanent block was expired: %q", removed)
	}
	if again := f.Add("youtube.com"); again != nil {
		t.Errorf("re-adding the same permanent block changed %q", again)
	}
}

func TestRemoveDropsUntilLine(t *testing.T) {
	var f File
	f.AddUntil(time.Now().Add(time.Hour), "youtube.com")
	f.Remove("youtube.com")
	if len(f.Block) != 0 {
		t.Errorf("block not empty after remove: %q", f.Block)
	}
}
