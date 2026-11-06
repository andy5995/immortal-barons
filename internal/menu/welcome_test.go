package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func welcomeWorld(t *testing.T, ibbs bool) *game.World {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.IBBS = ibbs
	cfg.BoardID = "Alpha"
	w := game.NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	return w
}

// InterBBS Scores and Travel Times are on the Welcome menu only in a league:
// both are league screens, with nothing to show on a lone board.
func TestWelcomeShowsLeagueScreensOnlyInALeague(t *testing.T) {
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
		for _, label := range []string{"InterBBS Scores", "Travel Times"} {
			if got := strings.Contains(out, label); got != tc.want {
				t.Errorf("ibbs=%v: %s shown = %v, want %v", tc.ibbs, label, got, tc.want)
			}
		}
	}
}

// Travel Times runs from the Welcome menu, where no empire exists yet.
func TestWelcomeTravelTimesRunsWithNoEmpire(t *testing.T) {
	w := welcomeWorld(t, true)
	w.LeagueNodes = []game.LeagueNode{{Number: 2, Name: "Far Star BBS"}}
	f := &fakeSession{keys: []rune("T 0")}
	if _, err := Welcome(f, w, "newcomer", "", Term{UTF8: true}); err != nil {
		t.Fatal(err)
	}
	out := stripANSI(f.out.String())
	for _, want := range []string{"Average Turn Around Times to All BBSes", "Far Star BBS"} {
		if !strings.Contains(out, want) {
			t.Errorf("Travel Times did not render %q:\n%s", want, out)
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
