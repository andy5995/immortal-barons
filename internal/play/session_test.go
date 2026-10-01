package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func TestSessionOnboardsAndSaves(t *testing.T) {
	cfg := cfgIn(t.TempDir()) // reuse the helper from play_test.go
	w := game.NewWorldSeed(cfg, 1)
	saved := false
	f := &fakeSession{keys: []rune(" \r1Khanate\ry0")} // splash, language (English), realm name, confirm, quit
	if _, err := Session(f, Identity{Handle: "Khan"}, w, cfg, "", game.MaintReport{}, func() error { saved = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("khan") == nil {
		t.Fatal("empire should have been onboarded into the shared world")
	}
	if !saved {
		t.Fatal("Session should call save at end of session")
	}
}

// TestOnboardLangForwardsDrainInput guards the onboarding wrapper's DrainInput
// forward: without it, the trailing-Enter drain after the onboarding
// Quit?/Confirm? answers silently no-ops for callers who picked a language.
func TestOnboardLangForwardsDrainInput(t *testing.T) {
	spy := &drainSpySession{}
	session.Drain(onboardLang{Session: spy, lang: "nl"})
	if !spy.drained {
		t.Fatal("onboardLang did not forward DrainInput to its inner session")
	}
}

type drainSpySession struct {
	session.Session
	drained bool
}

func (d *drainSpySession) DrainInput() { d.drained = true }

// TestOpeningMenuNamesTheBuild pins the version to the opening menu's HEADER,
// above "Game started on", rather than to a one-off line printed after the
// splash: a caller returning from any submenu redraws the header and nothing
// else, which is what used to leave the version showing on the first screen
// only. Escapes are stripped because the header highlights digit runs, which
// splits the version string.
func TestOpeningMenuNamesTheBuild(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	w.StartedDate = "2026-08-27"
	f := &fakeSession{keys: []rune(" \r1Khanate\ry0")} // splash, English, realm, confirm, quit
	if _, err := Session(f, Identity{Handle: "Khan"}, w, cfg, "", game.MaintReport{}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	out := ansiEsc.ReplaceAllString(f.out.String(), "")
	// Reached the opening menu, not just "produced some output".
	if !strings.Contains(out, "Today's News") || !strings.Contains(out, "Game started on") {
		t.Fatalf("never reached the opening menu:\n%s", out)
	}
	banner := strings.Index(out, game.NameVersion())
	started := strings.Index(out, "Game started on")
	if banner < 0 {
		t.Fatalf("%q is missing from the opening menu:\n%s", game.NameVersion(), out)
	}
	if banner > started {
		t.Errorf("want the version(%d) above game-started(%d)", banner, started)
	}
	// Drawn with the header, so it comes back on every redraw of the menu.
	if n := strings.Count(out, game.NameVersion()); n != strings.Count(out, "Game started on") {
		t.Errorf("version appears %d times, the header %d", n, strings.Count(out, "Game started on"))
	}
}

// A realm is played by one session at a time: a second login on the same
// handle, however it is cased or spaced, is turned away before anything loads,
// and the realm opens again once the first session ends.
func TestSecondSessionOnARealmIsRefused(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	held, err := lockRealm(cfg, "Khan")
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	f := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
	reason, err := Run(f, Identity{Handle: " KHAN "}, cfg, "2026-07-03")
	if err != nil || reason != "busy" {
		t.Fatalf("second session: reason %q, err %v; want busy", reason, err)
	}
	if !strings.Contains(f.out.String(), "already being played elsewhere") {
		t.Errorf("no refusal shown:\n%s", f.out.String())
	}
	if strings.Contains(f.out.String(), "Khanate") {
		t.Error("the refused session went on to onboard")
	}

	held.Release()
	f2 := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
	if reason, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil || reason == "busy" {
		t.Fatalf("after release: reason %q, err %v", reason, err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("khan") == nil {
		t.Error("the session after release did not create the realm")
	}
}

// The lock file is named by a fixed-length hash, so a handle far past any file
// name limit still logs in.
func TestLongHandleStillLocks(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	held, err := lockRealm(cfg, strings.Repeat("x", 400))
	if err != nil {
		t.Fatalf("lock for a 400-byte handle: %v", err)
	}
	held.Release()
}
