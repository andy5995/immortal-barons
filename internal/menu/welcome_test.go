package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func welcomeWorld(t *testing.T, ibbs bool) *game.World {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.IBBS = ibbs
	cfg.BoardID = "Alpha"
	return game.NewWorldSeed(cfg, 1)
}

// InterBBS Scores is on the Welcome menu only where there is a league to score,
// the same condition that hides it on the System menu.
func TestWelcomeShowsInterBBSScoresOnlyInALeague(t *testing.T) {
	for _, tc := range []struct {
		ibbs bool
		want bool
	}{{false, false}, {true, true}} {
		f := &fakeSession{keys: []rune("0")}
		if _, err := Welcome(f, welcomeWorld(t, tc.ibbs), "newcomer", "", Term{UTF8: true}); err != nil {
			t.Fatalf("ibbs=%v: %v", tc.ibbs, err)
		}
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "[Welcome]") {
			t.Fatalf("ibbs=%v: the menu was not drawn:\n%s", tc.ibbs, out)
		}
		if got := strings.Contains(out, "InterBBS Scores"); got != tc.want {
			t.Errorf("ibbs=%v: InterBBS Scores shown = %v, want %v", tc.ibbs, got, tc.want)
		}
	}
}

// The menu renders in the language the picker just took, though no empire
// exists yet to carry it.
func TestWelcomeRendersInTheLanguageJustPicked(t *testing.T) {
	f := &fakeSession{keys: []rune("0")}
	if _, err := Welcome(f, welcomeWorld(t, false), "newcomer", "de", Term{UTF8: true}); err != nil {
		t.Fatal(err)
	}
	if out := stripANSI(f.out.String()); !strings.Contains(out, "Spieleinstellungen") {
		t.Errorf("the menu did not render in German:\n%s", out)
	}
}

// Create Realm reports back so the caller goes on to name one; Quit does not.
func TestWelcomeReportsWhichWayItLeft(t *testing.T) {
	for keys, want := range map[string]bool{"1": true, "0": false, "\r": true} {
		f := &fakeSession{keys: []rune(keys)}
		got, err := Welcome(f, welcomeWorld(t, false), "newcomer", "", Term{UTF8: true})
		if err != nil {
			t.Fatalf("keys %q: %v", keys, err)
		}
		if got != want {
			t.Errorf("keys %q: create = %v, want %v", keys, got, want)
		}
	}
}

// Reading the rules must not end the session. The post-action check reads a
// missing empire as an eliminated one, which told a newcomer their empire had
// collapsed the moment they looked at Game Setup and closed the session under
// them. Asserts the menu was REACHED again after the action, not just drawn once.
func TestWelcomeActionsDoNotEndTheSession(t *testing.T) {
	// Game Setup, its two pauses, See Scores, its pause, then Quit.
	f := &fakeSession{keys: []rune("G  3 0")}
	create, err := Welcome(f, welcomeWorld(t, false), "newcomer", "", Term{UTF8: true})
	if err != nil {
		t.Fatalf("the session ended during a Welcome action: %v", err)
	}
	if create {
		t.Error("Quit should not report a realm to create")
	}
	out := stripANSI(f.out.String())
	if strings.Contains(out, "collapsed") {
		t.Errorf("a caller with no realm was told their empire collapsed:\n%s", out)
	}
	if !strings.Contains(out, "Turns per day") {
		t.Errorf("Game Setup was never reached:\n%s", out)
	}
	if n := strings.Count(out, "[Welcome]"); n < 3 {
		t.Errorf("the menu was drawn %d times, want a redraw after each action", n)
	}
}
