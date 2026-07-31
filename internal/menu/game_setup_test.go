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
