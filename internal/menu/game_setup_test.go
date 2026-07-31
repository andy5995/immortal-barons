package menu

import (
	"strings"
	"testing"
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
		"Turns, Land and Money", "War, Trade and Board", // both panels
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

	// With it off, the league group is absent entirely.
	f2 := &fakeSession{keys: []rune("  ")}
	w2 := newWorld()
	w2.Config.IBBS = false
	gameSetup(f2, w2)
	if strings.Contains(f2.out.String(), "Coordinator") {
		t.Error("league group should not appear on a standalone board")
	}
}
