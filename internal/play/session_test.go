package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func TestSessionOnboardsAndSaves(t *testing.T) {
	cfg := cfgIn(t.TempDir()) // reuse the helper from play_test.go
	w := game.NewWorldSeed(cfg, 1)
	saved := false
	f := &fakeSession{keys: []rune(" \rKhanate\r0")} // splash, language (English), realm name, quit
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

// TestPreMenuBannerNamesTheBuild pins where the program name and version are
// stated: under the maintenance notice, above the opening menu's "Game started
// on" header. The order is what the test is for — the two neighbours are
// printed by different packages, so nothing else holds them in sequence.
func TestPreMenuBannerNamesTheBuild(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	w.StartedDate = "2026-08-27"
	f := &fakeSession{keys: []rune(" \rKhanate\ry0")} // splash, English, realm, confirm, quit
	if _, err := Session(f, Identity{Handle: "Khan"}, w, cfg, "", game.MaintReport{}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	// Reached the opening menu, not just "produced some output".
	if !strings.Contains(out, "Today's News") || !strings.Contains(out, "Game started on") {
		t.Fatalf("never reached the opening menu:\n%s", out)
	}
	maint := strings.Index(out, "Maintenance has already been run today.")
	banner := strings.Index(out, game.NameVersion())
	started := strings.Index(out, "Game started on")
	if maint < 0 || banner < 0 {
		t.Fatalf("maint notice at %d, %q at %d:\n%s", maint, game.NameVersion(), banner, out)
	}
	if !(maint < banner && banner < started) {
		t.Fatalf("want maintenance(%d) < version(%d) < game-started(%d)", maint, banner, started)
	}
}
