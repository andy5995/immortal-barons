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
	f := &fakeSession{keys: []rune(" \rKhanate\ry0")} // splash, English, realm, confirm, quit
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
