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
		w.Incoming = []*game.Annihilator{{
			Creator: "Wildside", Launched: true, Intact: 100,
			ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
		}}
	})
	// "y" to attack, weapon 1 (asked even for one, as the original asks), then
	// the whole air force.
	f := &fakeSession{keys: []rune("y1\r200000\r")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	for _, want := range []string{"Days Until Self-Destruct", "Wildside", "100%", "jets were destroyed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the defense screen never showed %q:\n%s", want, out)
		}
	}
	var intact, jets int
	w.With(func() {
		if len(w.Incoming) > 0 {
			intact = w.Incoming[0].Intact
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

// With more than one weapon on the ground the screen lists them and asks which,
// as the original does. The jets must reach the one that was picked.
func TestAnnihilatorDefensePicksAmongSeveralWeapons(t *testing.T) {
	w := newWorld()
	var p *game.Empire
	w.With(func() {
		w.Config.IBBS = true
		p = w.Player()
		p.Regions = game.RegionMix{Agricultural: 5000}
		p.Jets = 200_000
		for _, from := range []string{"Wildside", "The Eclipse"} {
			w.Incoming = append(w.Incoming, &game.Annihilator{
				Creator: from, Launched: true, Intact: 100,
				ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
			})
		}
	})
	// "y" to attack, then weapon 2, then the whole air force.
	f := &fakeSession{keys: []rune("y2\r200000\r")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	for _, want := range []string{"Enter Gooie Number", "Wildside", "The Eclipse", "jets were destroyed"} {
		if !strings.Contains(out, want) {
			t.Errorf("the defense screen never showed %q:\n%s", want, out)
		}
	}
	var picked, other int
	w.With(func() {
		if d := w.IncomingFrom("The Eclipse"); d != nil {
			picked = d.Intact
		}
		if d := w.IncomingFrom("Wildside"); d != nil {
			other = d.Intact
		}
	})
	if picked >= 100 {
		t.Errorf("the weapon picked is %d%% intact — the jets went somewhere else", picked)
	}
	if other != 100 {
		t.Errorf("the weapon NOT picked is %d%% intact, want 100: one sortie hit both", other)
	}
}

// Saying no leaves before the table is drawn. The original asks first for this
// reason: the screen is forced on every baron at the start of their turn, and
// one who is not spending jets today should not have to read a list to decline.
func TestAnnihilatorDefenseAsksBeforeDrawingTheList(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		p := w.Player()
		p.Regions = game.RegionMix{Agricultural: 5000}
		p.Jets = 200_000
		w.Incoming = []*game.Annihilator{{
			Creator: "Wildside", Launched: true, Intact: 100,
			ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
		}}
	})
	f := &fakeSession{keys: []rune("n")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Do you wish to attack") {
		t.Fatalf("the question was never asked:\n%s", out)
	}
	for _, unwanted := range []string{"Days Until Self-Destruct", "Wildside", "Enter Gooie Number"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("declining still drew %q:\n%s", unwanted, out)
		}
	}
}

// A baron with no jets is told so instead of being shown a list of things they
// cannot touch — the original's order too, its jet check sitting between the
// question and the table.
func TestAnnihilatorDefenseTellsAJetlessBaronBeforeTheList(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		p := w.Player()
		p.Regions = game.RegionMix{Agricultural: 5000}
		p.Jets = 0
		w.Incoming = []*game.Annihilator{{
			Creator: "Wildside", Launched: true, Intact: 100,
			ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
		}}
	})
	f := &fakeSession{keys: []rune("y\r")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "you have none") {
		t.Errorf("a baron with no jets was not told:\n%s", out)
	}
	if strings.Contains(out, "Days Until Self-Destruct") {
		t.Errorf("the list was drawn for a baron who cannot attack:\n%s", out)
	}
}

// A number past the end is asked again rather than taken as a cancel. The
// original's number reader loops on an over-max value and shows the limit, so a
// typo must not throw a baron out of a screen they are shown once a turn.
func TestAnnihilatorDefenseAsksAgainForANumberPastTheEnd(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		p := w.Player()
		p.Regions = game.RegionMix{Agricultural: 5000}
		p.Jets = 200_000
		for _, from := range []string{"Wildside", "The Eclipse"} {
			w.Incoming = append(w.Incoming, &game.Annihilator{
				Creator: from, Launched: true, Intact: 100,
				ArrivesDay: w.GameDay, DaysLeft: game.AnnihilatorSiegeDays,
			})
		}
	})
	// "y", then a number past the end, then a good one, then the air force. No
	// key between the refusal and the re-ask: a pause there would be one the
	// original does not ask for, and a stray key in this script would hide it.
	f := &fakeSession{keys: []rune("y9\r1\r200000\r")}
	annihilatorDefense(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "only 2 to choose from.") {
		t.Errorf("an out-of-range number was not refused with the limit:\n%s", out)
	}
	if !strings.Contains(out, "jets were destroyed") {
		t.Errorf("the screen gave up instead of asking again:\n%s", out)
	}
	var intact int
	w.With(func() {
		if d := w.IncomingFrom("Wildside"); d != nil {
			intact = d.Intact
		}
	})
	if intact >= 100 {
		t.Errorf("weapon 1 is %d%% intact — the retry picked something else", intact)
	}
}
