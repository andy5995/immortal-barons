package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The Game Setup screen is paged: the full ruleset does not fit 80x25, so it
// prints in two panels with a pause between them. Both must appear, and a limit
// whose zero means "no limit" must say so rather than printing a bare 0.
func TestGameSetupPagesAndNamesZeroLimits(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")} // dismiss both pauses
	w := newWorld()
	w.Config.GameLength = 0     // endless
	w.Config.MaxRegions = 0     // unlimited
	w.Config.IdleDaysRemove = 7 // a real limit, to check the unit
	gameSetup(f, w)
	out := f.out.String()

	for _, want := range []string{
		"The day and the game", "Land", "Money", // page one groups
		"Forces and trade", "Attacking", "This board", // page two groups
		"Turns per day", "New land per realm/day", // page one
		"Buy military", "Attack damage", // page two
		"Endless", "Unlimited", "7 days",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Paused"); n != 2 {
		t.Errorf("expected a pause after each of the two panels, got %d\n%s", n, out)
	}
}

// With Inter-BBS play on, the screen must say the ruleset is not this sysop's
// to set — the Coordinator broadcasts it to every board in the league.
func TestGameSetupNamesTheLeague(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")}
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "eye of the storm"
	gameSetup(f, w)
	out := f.out.String()

	for _, want := range []string{"The league", "eye of the storm", "Coordinator"} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q with IBBS on:\n%s", want, out)
		}
	}
	// An unnamed league gets no row at all rather than a blank one.
	if strings.Contains(out, "League ") {
		t.Errorf("Game Setup shows a League row for a league with no name:\n%s", out)
	}

	// Named, it is the first thing the league group says.
	fn := &fakeSession{keys: []rune("  ")}
	wn := newWorld()
	wn.Config.IBBS = true
	wn.Config.BoardID = "eye of the storm"
	wn.Config.LeagueName = "Southern Cross"
	gameSetup(fn, wn)
	if !strings.Contains(fn.out.String(), "Southern Cross") {
		t.Errorf("Game Setup omits the league name:\n%s", fn.out.String())
	}

	// With it off, the league group is absent entirely.
	f2 := &fakeSession{keys: []rune("  ")}
	w2 := newWorld()
	w2.Config.IBBS = false
	gameSetup(f2, w2)
	if strings.Contains(f2.out.String(), "Coordinator") {
		t.Error("league group should not appear on a standalone board")
	}
}

// Game Setup reports the money cap. A player who does not know the figure only
// discovers it by losing gold to it, and the sysop can set it to anything from
// two billion up, so it cannot be assumed.
func TestGameSetupShowsTheMoneyCap(t *testing.T) {
	for _, c := range []struct {
		billions int
		want     string
	}{
		{game.MoneyCapMinBillions, "2B"},
		{50, "50B"},
		{game.MoneyCapMaxBillions, "999B"},
	} {
		w := newWorld()
		w.Config.MoneyCapBillions = c.billions
		f := &fakeSession{keys: []rune("     ")}
		gameSetup(f, w)
		out := anyEscape.ReplaceAllString(f.out.String(), "")
		if !strings.Contains(out, "Most gold you can hold") {
			t.Fatal("Game Setup omits the money cap row")
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("cap %dB: screen should show %q", c.billions, c.want)
		}
	}
}
