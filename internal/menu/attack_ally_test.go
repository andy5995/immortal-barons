package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// An alliance partner is attackable, after the breach question. The original
// lists it with its letter and asks "Are you sure you wish break your
// agreement?" before the battle (grep `cap/kd3-01.cap` for that prompt); IB
// withheld the letter, which made confirmBreach unreachable for an alliance.
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
	// And on the info-op list too, silently: the covert menu pushes the picker's
	// breach flag as 0 for Send Spy and Spy on Relations (run_covert_operations_menu
	// at unit +0xb19, comparing the choice against '1' and '6'), so the picker
	// skips its break block and hands back the letter like any other realm.
	var info *targetRow
	for _, r := range covertTargets.rows(w) {
		if r.name == ally.Name {
			r := r
			info = &r
		}
	}
	if info == nil || !info.attackable {
		t.Errorf("%s cannot be looked at from the info-op list; spying on an ally costs the pact nothing", ally.Name)
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

// A covert operation that spends an agent breaks a pact the same way an attack
// does. BRE runs both through one picker (choose_target_empire, BRE.OVR
// 0x01aa99), and `cap/kd3-01.cap` catches the covert half word for word: the
// same "Are you sure you wish break your agreement?" ahead of a Demoralize
// Forces, the same revolt, and the operation still going out afterwards. IB left
// an ally off the covert list entirely until 2026-09-06, so the question could
// never be reached.
func TestACovertOpAgainstAnAllyBreaksThePactFirst(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	p.Gold, p.Agents = 1_000_000_000, 50
	p.CovertOpsToday = nil
	ally := recipients(w)[0]
	ally.Protection = 0
	ally.TreatyOffers = append(ally.TreatyOffers, game.TreatyOffer{From: p.Name, Type: "Full Defense Alliance"})
	if !w.AcceptTreaty(ally, p.Name, "Full Defense Alliance") {
		t.Fatal("the test could not form the alliance it is about")
	}
	morale := p.Morale

	// The ally's letter, agree to break the pact, one agent, then the pause.
	f := &fakeSession{keys: []rune(ally.Letter() + "y1\r ")}
	if res := covertAction(game.OpDemoralizeForces)(f, w); res != Stay {
		t.Fatalf("covert action returned %v, want Stay", res)
	}

	out := f.out.String()
	if !strings.Contains(out, "Break the agreement?") {
		t.Fatalf("no breach question was asked:\n%s", out)
	}
	if w.AreAllied(p, ally) {
		t.Error("the alliance survived a covert op that broke it")
	}
	if p.Morale >= morale {
		t.Errorf("breaking the pact cost no morale: %d -> %d", morale, p.Morale)
	}
	// BRE sends the agent anyway — the break is the price of the target, not an
	// alternative to the operation.
	if len(w.CovertQueue) != 1 {
		t.Errorf("queued %d operations, want the 1 the agent was sent on", len(w.CovertQueue))
	}
}

// The other half of the binary's split: an INFO operation aimed at an ally asks
// nothing and leaves the pact standing. BRE's covert menu pushes the picker's
// breach flag as 0 for exactly '1' and '6', so choose_target_empire returns the
// letter without reaching break_diplomatic_treaty.
func TestSpyingOnAnAllyLeavesThePactAlone(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	p.Gold, p.Agents = 1_000_000_000, 50
	ally := recipients(w)[0]
	ally.Protection = 0
	ally.TreatyOffers = append(ally.TreatyOffers, game.TreatyOffer{From: p.Name, Type: "Full Defense Alliance"})
	if !w.AcceptTreaty(ally, p.Name, "Full Defense Alliance") {
		t.Fatal("the test could not form the alliance it is about")
	}
	morale := p.Morale

	f := &fakeSession{keys: []rune(ally.Letter() + " ")} // the ally's letter, then the pause
	covertAction(game.OpSpyOnRelations)(f, w)

	out := f.out.String()
	if strings.Contains(out, "Break the agreement?") {
		t.Errorf("spying on an ally asked to break the pact:\n%s", out)
	}
	if !w.AreAllied(p, ally) {
		t.Error("the alliance did not survive being spied on")
	}
	if p.Morale != morale {
		t.Errorf("spying on an ally cost morale: %d -> %d", morale, p.Morale)
	}
}
