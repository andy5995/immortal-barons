package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// An alliance partner is attackable, after the breach question. The original
// lists it with its letter and asks "Are you sure you wish break your
// agreement?" before the battle (`cap/kd3-01.cap` line 13411); IB withheld the
// letter, which made confirmBreach unreachable for an alliance.
func TestAnAllianceDoesNotHideARealmFromTheWarMenu(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	ally := recipients(w)[0]
	ally.Protection = 0
	ally.TreatyOffers = append(ally.TreatyOffers, game.TreatyOffer{From: p.Name, Type: "Full Defense Alliance"})
	if !w.AcceptTreaty(ally, p.Name, "Full Defense Alliance") {
		t.Fatal("the test could not form the alliance it is about")
	}

	rows := warTargets.rows(w)
	var found *targetRow
	for i := range rows {
		if rows[i].name == ally.Name {
			found = &rows[i]
		}
	}
	if found == nil {
		t.Fatalf("the ally is missing from the war menu's list entirely: %+v", rows)
	}
	if !found.attackable {
		t.Errorf("%s is allied and therefore unattackable; the original lets you break the pact and attack", ally.Name)
	}
	// The covert list is the other half of the rule: those operations ask no
	// breach question, so the pact must keep the realm off it.
	for _, r := range covertTargets.rows(w) {
		if r.name == ally.Name && r.attackable {
			t.Errorf("%s is reachable from the covert list with the alliance intact", ally.Name)
		}
	}
}

// The whole flow, not only the list: picking an ally must raise the breach
// prompt and, on a yes, fight the battle.
func TestAttackingAnAllyAsksToBreakThePactFirst(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	p.Troopers = 1_000_000
	ally := recipients(w)[0]
	ally.Protection = 0
	ally.TreatyOffers = append(ally.TreatyOffers, game.TreatyOffer{From: p.Name, Type: "Full Defense Alliance"})
	if !w.AcceptTreaty(ally, p.Name, "Full Defense Alliance") {
		t.Fatal("the test could not form the alliance it is about")
	}
	before := ally.Troopers
	f := &fakeSession{keys: []rune("Ay")} // pick the ally, then agree to break the pact

	regularAttack(f, w)

	out := f.out.String()
	if !strings.Contains(out, "Break the agreement?") {
		t.Fatalf("no breach question was asked:\n%s", out)
	}
	if ally.Troopers >= before {
		t.Errorf("the attack never landed: ally troopers %d -> %d", before, ally.Troopers)
	}
	if w.AreAllied(p, ally) {
		t.Error("the alliance survived an attack that broke it")
	}
}
