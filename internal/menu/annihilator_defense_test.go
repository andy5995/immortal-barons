package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A Gooie Kablooie squatting on the planet is offered to every baron at the
// start of their turn, jets and all — the original asks there rather than behind
// a menu item, because a baron who never opens the InterPlanetary menu would
// never see it (#112).
func TestAnnihilatorDefenseSendsJetsAtALandedWeapon(t *testing.T) {
	w := newWorld()
	var p *game.Empire
	w.With(func() {
		w.Config.IBBS = true
		p = w.Player()
		p.Regions = game.RegionMix{Agricultural: 5000}
		p.Jets = 200_000
		w.Incoming = &game.Annihilator{
			Creator: "Wildside", Launched: true, Intact: 100,
			ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
		}
	})
	// "y" to attack, then the whole air force.
	f := &fakeSession{keys: []rune("y200000\r")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	for _, want := range []string{"Days Until Self-Destruct", "Wildside", "100%", "jets were destroyed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the defense screen never showed %q:\n%s", want, out)
		}
	}
	var intact, jets int
	w.With(func() {
		if w.Incoming != nil {
			intact = w.Incoming.Intact
		}
		jets = w.Player().Jets
	})
	if intact >= 100 || intact <= 0 {
		t.Errorf("weapon is %d%% intact after a full wave, want a dent short of destruction", intact)
	}
	if jets >= 200_000 {
		t.Errorf("jets came home free: %d", jets)
	}
}

// Nothing is asked when no weapon has landed, so the prompt never costs a
// keypress in the common case.
func TestAnnihilatorDefenseIsSilentWithNothingToFight(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("")}
	annihilatorDefense(f, w)
	if out := f.out.String(); out != "" {
		t.Errorf("the defense prompt spoke with no weapon on the planet:\n%s", out)
	}
}
