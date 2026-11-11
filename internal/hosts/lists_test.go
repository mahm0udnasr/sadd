package hosts

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 11, 20, 0, 0, 0, time.UTC)

func TestListRoundTrip(t *testing.T) {
	f, _ := Parse("127.0.0.1 localhost\n")
	f.Add("youtube.com")
	f.SetList("porn", "https://example.org/hosts", t0, []string{"a.com", "www.a.com"})

	out := f.Render()
	for _, want := range []string{
		"# >>> sadd blocklist porn\n# downloaded 2026-11-11T20:00:00Z from https://example.org/hosts\n",
		"0.0.0.0 a.com\n:: a.com\n0.0.0.0 www.a.com\n:: www.a.com\n# <<< sadd blocklist porn\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	g, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if g.Render() != out {
		t.Errorf("not stable:\n%s\nvs\n%s", g.Render(), out)
	}
	// The blocklist must not leak into the normal entries.
	if d := g.Domains(); len(d) != 1 || d[0] != "youtube.com" {
		t.Errorf("Domains = %v", d)
	}
	info, ok := g.List("porn")
	if !ok || info.Domains != 2 || info.Source != "https://example.org/hosts" || !info.Downloaded.Equal(t0) {
		t.Errorf("info = %+v, %v", info, ok)
	}
}

func TestListUserLinesUntouched(t *testing.T) {
	in := "# mine\n10.0.0.1 nas\n# >>> sadd start\n# <<< sadd end\n" +
		"# >>> sadd blocklist porn\n0.0.0.0 a.com\n:: a.com\n# <<< sadd blocklist porn\n" +
		"# after\n"
	f, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if out := f.Render(); out != in {
		t.Errorf("changed:\n%s", out)
	}
}

func TestListTurnOff(t *testing.T) {
	var f File
	f.SetList("porn", "u", t0, []string{"a.com"})
	if !f.SetListOffAt("porn", t0.Add(10*time.Minute)) {
		t.Fatal("list not found")
	}
	if info, _ := f.List("porn"); !info.TurningOff() {
		t.Fatal("not turning off")
	}

	if got := f.ExpireLists(t0.Add(5 * time.Minute)); len(got) != 0 {
		t.Errorf("removed too early: %v", got)
	}
	// Cancel, then schedule again and let it pass.
	f.SetListOffAt("porn", time.Time{})
	if info, _ := f.List("porn"); info.TurningOff() {
		t.Error("cancel did not work")
	}
	f.SetListOffAt("porn", t0.Add(10*time.Minute))
	if got := f.ExpireLists(t0.Add(10 * time.Minute)); len(got) != 1 {
		t.Errorf("not removed on time: %v", got)
	}
	if _, ok := f.List("porn"); ok {
		t.Error("still on")
	}
}

func TestSetListReplacesAndCancelsTurnOff(t *testing.T) {
	var f File
	f.SetList("porn", "u", t0, []string{"a.com"})
	f.SetListOffAt("porn", t0.Add(time.Minute))
	f.SetList("porn", "u", t0, []string{"a.com", "b.com"}) // an update
	info, _ := f.List("porn")
	if info.Domains != 2 || info.TurningOff() || len(f.Lists()) != 1 {
		t.Errorf("info = %+v, lists = %d", info, len(f.Lists()))
	}
}

func TestBrokenListSections(t *testing.T) {
	for _, in := range []string{
		"# >>> sadd blocklist porn\n0.0.0.0 a.com\n",                                                       // no end
		"# <<< sadd blocklist porn\n",                                                                      // no start
		"# >>> sadd blocklist porn\n# >>> sadd blocklist x\n# <<< sadd blocklist porn\n",                   // nested
		"# >>> sadd blocklist p\n# <<< sadd blocklist p\n# >>> sadd blocklist p\n# <<< sadd blocklist p\n", // twice
	} {
		if _, err := Parse(in); !errors.Is(err, ErrBadList) {
			t.Errorf("Parse(%q) err = %v", in, err)
		}
	}
}
