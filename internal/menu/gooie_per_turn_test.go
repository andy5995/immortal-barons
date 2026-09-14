package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The Gooie defense is a turn stage, not an entry stage: BRE runs it as stage 12
// of run_player_turn, so a baron playing three turns gets three sorties. It sat
// ahead of the turn loop until 2026-09-14 and was offered once per entry. Every
// other test of this screen calls annihilatorDefense directly, so nothing would
// notice it being hoisted back out — this one drives two whole turns and counts
// the offers.
// turnKeys is one whole turn with nothing done in it: the pauses, Quit at the
// Bank, Spending and Attack menus, "no" to the Gooie, then Enter to take the
// continue prompt's default and start the next turn.
const turnKeys = "    000n\r"

func TestGooieDefenseIsOfferedOnEveryTurn(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Prefs.AutoPayMaint = true
	p.TurnsLeft = 2
	w.World.Config.IBBS = true
	w.World.Incoming = append(w.World.Incoming, &game.Annihilator{
		Creator: "The Eclipse", Launched: true, Intact: 100,
		ArrivesDay: w.World.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
	})

	// Turn 1: four pauses, Quit Bank, Quit Spending, Quit Attack, decline the
	// Gooie, accept "Continue to your next turn?". Turn 2 repeats without the
	// first-turn screens and declines the continue prompt to stop.
	f := &fakeSession{keys: []rune(strings.Repeat(turnKeys, 3) + " ")}
	runTurn(f, w)

	out := stripANSI(f.out.String())
	if n := strings.Count(out, "Do you wish to attack a Gooie Kablooie?"); n != 2 {
		t.Errorf("the Gooie defense was offered %d times over two turns, want 2:\n%s", n, out)
	}
	// The script must have played both turns, not run dry inside the first.
	if p.TurnsLeft != 0 {
		t.Errorf("TurnsLeft = %d after the script, want 0 (both turns played)", p.TurnsLeft)
	}
}
