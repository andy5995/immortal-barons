package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestDiplomacyMenuListsTreatyTypes checks that the Diplomacy menu (#68)
// shows treaty types as direct items — BRE's layout — instead of hiding them
// behind a single "Modify Diplomacy" item.
func TestDiplomacyMenuListsTreatyTypes(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "0", menus.Diplomacy) // Quit immediately
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{
		"Tariff Trade Agreement",
		"Free Trade Agreement",
		"Full Defense Alliance",
		"Declaration Of War",
		"View Treaties",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Diplomacy menu missing item %q; output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Modify Diplomacy") {
		t.Errorf("Diplomacy menu should no longer show the old \"Modify Diplomacy\" item; output:\n%s", out)
	}
}

// TestNegotiateTreatyProposesToTarget drives a treaty-type item end to end:
// pick the target empire, and it should record a pending offer of the right
// kind (ProposeTreaty, not an immediately-formed treaty).
func TestNegotiateTreatyProposesToTarget(t *testing.T) {
	w := newWorld()
	p := w.Player()
	var target *game.Empire
	for _, e := range w.Empires {
		if e != p {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("newWorld() should seed at least one AI empire")
	}

	action := negotiateTreaty("Free Trade Agreement")
	f := &fakeSession{keys: []rune("A")} // pick the first (only) listed empire, BRE's lettered Id
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}

	if w.World.HasTreaty(p, target, "Free Trade Agreement") {
		t.Fatal("a lone proposal should not form the treaty yet")
	}
	found := false
	for _, o := range target.TreatyOffers {
		if o.From == p.Name && o.Type == "Free Trade Agreement" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a pending Free Trade Agreement offer from %s on %s, got %v", p.Name, target.Name, target.TreatyOffers)
	}
}

// TestNegotiateTreatyAcceptsMatchingOffer checks that selecting the same
// treaty-type item toward an empire that already offered it accepts and
// forms the treaty, rather than sending a second, separate offer.
func TestNegotiateTreatyAcceptsMatchingOffer(t *testing.T) {
	w := newWorld()
	p := w.Player()
	var target *game.Empire
	for _, e := range w.Empires {
		if e != p {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("newWorld() should seed at least one AI empire")
	}
	w.World.ProposeTreaty(target, p, "Intelligence Alliance")

	action := negotiateTreaty("Intelligence Alliance")
	f := &fakeSession{keys: []rune("Ay")} // pick the empire, then confirm accepting the offer
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	if !w.World.HasTreaty(p, target, "Intelligence Alliance") {
		t.Fatalf("want the Intelligence Alliance formed; treaties: %v", w.World.TreatiesBetween(p, target))
	}
}

// TestDeclareWarEndsTheRelation checks BRE's Declaration Of War ends the
// standing agreement with the chosen target, leaving the pair hostile.
//
// A pair holds exactly ONE relation (#88), so this also pins the replacement
// rule: accepting a second pact supersedes the first rather than stacking with
// it. The test used to assert two treaties could be held at once.
func TestDeclareWarEndsTheRelation(t *testing.T) {
	w := newWorld()
	p := w.Player()
	var target *game.Empire
	for _, e := range w.Empires {
		if e != p {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("newWorld() should seed at least one AI empire")
	}
	w.World.ProposeTreaty(p, target, "Full Defense Alliance")
	w.World.AcceptTreaty(target, p.Name, "Full Defense Alliance")
	w.World.ProposeTreaty(p, target, "Free Trade Agreement")
	w.World.AcceptTreaty(target, p.Name, "Free Trade Agreement")
	if got := w.World.Relation(p, target); got != "Free Trade Agreement" {
		t.Fatalf("the later pact should replace the earlier one, got %q", got)
	}

	f := &fakeSession{keys: []rune("1\ry")} // pick the empire, then confirm
	if res := declareWar(f, w); res != Stay {
		t.Fatalf("declareWar = %v, want Stay", res)
	}
	if got := w.World.Relation(p, target); got != game.RelationEnemy {
		t.Errorf("after a Declaration Of War the pair should be %q, got %q", game.RelationEnemy, got)
	}
}
