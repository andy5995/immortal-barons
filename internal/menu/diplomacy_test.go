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
	// Mark the first (only) listed empire by BRE's lettered Id, close the list
	// with RETURN, then decline the covering note.
	f := &fakeSession{keys: []rune("A\rn")}
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", f.out.String())
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
	f := &fakeSession{keys: []rune("A\ry")} // mark the empire, close the list, confirm the accept
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

	var letter string
	w.With(func() { letter = w.EmpireLetter(target) })
	f := &fakeSession{keys: []rune(letter + "\ry")} // mark the empire, close the list, confirm
	if res := declareWar(f, w); res != Stay {
		t.Fatalf("declareWar = %v, want Stay", res)
	}
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", f.out.String())
	}
	if got := w.World.Relation(p, target); got != game.RelationEnemy {
		t.Errorf("after a Declaration Of War the pair should be %q, got %q", game.RelationEnemy, got)
	}
}

// offeredType reports whether from has a pending offer of ttype on e.
func offeredType(e *game.Empire, from, ttype string) bool {
	for _, o := range e.TreatyOffers {
		if o.From == from && o.Type == ttype {
			return true
		}
	}
	return false
}

// Every Diplomacy action that addresses a realm takes a LIST: the picker is the
// multi-select one, so two letters propose the same pact to two realms from one
// action (BRE.OVR 0x1b65e, called twice from its diplomacy menu).
func TestNegotiateTreatyProposesToSeveralRealms(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	p := w.Player()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	first, second, untouched := rows[0], rows[2], rows[1]

	action := negotiateTreaty("Full Defense Alliance")
	// Marked out of order, to pin that the send follows the marks and not the
	// order they were pressed in.
	keys := string(second.letter) + string(first.letter) + "\rn"
	f := &fakeSession{keys: []rune(keys)}
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", f.out.String())
	}
	for _, r := range []pickRow{first, second} {
		if !offeredType(r.e, p.Name, "Full Defense Alliance") {
			t.Errorf("%s holds no pending offer from %s, got %v", r.name, p.Name, r.e.TreatyOffers)
		}
	}
	if len(untouched.e.TreatyOffers) != 0 {
		t.Errorf("%s was not marked but holds %v", untouched.name, untouched.e.TreatyOffers)
	}
}

// Declaration Of War takes the same list, so one action can end several
// relations at once.
func TestDeclareWarOnSeveralRealms(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	p := w.Player()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	first, second, spared := rows[0], rows[2], rows[1]
	for _, r := range rows {
		treatyWith(w, r.e)
	}

	f := &fakeSession{keys: []rune(string(first.letter) + string(second.letter) + "\ry")}
	if res := declareWar(f, w); res != Stay {
		t.Fatalf("declareWar = %v, want Stay", res)
	}
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", f.out.String())
	}
	for _, r := range []pickRow{first, second} {
		if got := w.World.Relation(p, r.e); got != game.RelationEnemy {
			t.Errorf("%s should be %q after the declaration, got %q", r.name, game.RelationEnemy, got)
		}
	}
	if got := w.World.Relation(p, spared.e); got != "Free Trade Agreement" {
		t.Errorf("%s was not marked; relation = %q, want the standing pact", spared.name, got)
	}
}

// "*=All Allies" still narrows the send to the realms under treaty, and it
// composes with the list rather than standing outside it: the allies are marked
// and the same RETURN closes the list.
func TestNegotiateTreatyToAllAllies(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	p := w.Player()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	treatyWith(w, rows[0].e)
	treatyWith(w, rows[2].e)
	stranger := rows[1]

	action := negotiateTreaty("Technology Agreement")
	f := &fakeSession{keys: []rune("*\rn")}
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	for _, r := range []pickRow{rows[0], rows[2]} {
		if !offeredType(r.e, p.Name, "Technology Agreement") {
			t.Errorf("ally %s holds no pending offer, got %v", r.name, r.e.TreatyOffers)
		}
	}
	if len(stranger.e.TreatyOffers) != 0 {
		t.Errorf("%s holds no treaty with the player but got %v", stranger.name, stranger.e.TreatyOffers)
	}
}

// '?' at a Diplomacy prompt lists the "-*Relations*-" roster, not the score
// table Send Message lists — BRE's selection routine takes that choice as a flag
// and the Diplomacy menu passes 0 (docs/dev/bre-screens.md).
func TestDiplomacyPickerListsRelations(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	treatyWith(w, rows[0].e)

	action := negotiateTreaty("Protective Trade")
	f := &fakeSession{keys: []rune("?\r")} // list, then close the list with nothing marked
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	out := sgr.ReplaceAllString(f.out.String(), "")
	if !strings.Contains(out, "-*Relations*-") {
		t.Fatalf("'?' should list the Relations roster:\n%s", out)
	}
	if !strings.Contains(out, "Free Trade Agreement") {
		t.Errorf("the roster should name the pact held with %s:\n%s", rows[0].name, out)
	}
	if strings.Contains(out, "Net Worth") {
		t.Errorf("'?' listed the score table, not Relations:\n%s", out)
	}
	if len(rows[0].e.TreatyOffers) != 0 {
		t.Errorf("an empty list proposed something to %s: %v", rows[0].name, rows[0].e.TreatyOffers)
	}
}

// Selecting the treaty type a realm ALREADY holds with you says the pact stands
// and returns to the Diplomacy menu. It used to offer to break it, which put the
// one destructive diplomatic act behind the same key as the constructive one.
func TestNegotiateTreatyAlreadyHeldChangesNothing(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	p, target := w.Player(), rows[0].e
	treatyWith(w, target) // a standing Free Trade Agreement

	action := negotiateTreaty("Free Trade Agreement")
	f := &fakeSession{keys: []rune(string(rows[0].letter) + "\r")} // mark the realm, close the list
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	out := sgr.ReplaceAllString(f.out.String(), "")
	if !strings.Contains(out, "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", out)
	}
	if !strings.Contains(out, "holding strong") {
		t.Fatalf("want the pact-still-stands message:\n%s", out)
	}
	if strings.Contains(out, "Break this treaty") {
		t.Fatalf("selecting a held pact must not offer to break it:\n%s", out)
	}
	if got := w.World.Relation(p, target); got != "Free Trade Agreement" {
		t.Errorf("the standing pact should be untouched, got %q", got)
	}
	if len(target.TreatyOffers) != 0 {
		t.Errorf("nothing should have been proposed to %s, got %v", rows[0].name, target.TreatyOffers)
	}
}

// The regression guard for the case above: a DIFFERENT pact offered to a realm
// you already have relations with is a real proposal and still goes out. A pair
// holds one relation (#88), so the standing pact gives way only on acceptance.
func TestNegotiateTreatyProposesDifferentTypeToPartner(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	p, target := w.Player(), rows[0].e
	treatyWith(w, target) // a standing Free Trade Agreement

	action := negotiateTreaty("Tariff Trade Agreement")
	f := &fakeSession{keys: []rune(string(rows[0].letter) + "\rn")} // mark, close the list, decline the note
	if res := action(f, w); res != Stay {
		t.Fatalf("negotiateTreaty action = %v, want Stay", res)
	}
	out := sgr.ReplaceAllString(f.out.String(), "")
	if !strings.Contains(out, "Send to:") {
		t.Fatalf("never reached the Send to: picker:\n%s", out)
	}
	if strings.Contains(out, "holding strong") {
		t.Fatalf("a different pact is a real proposal, not a standing one:\n%s", out)
	}
	if !offeredType(target, p.Name, "Tariff Trade Agreement") {
		t.Errorf("want a pending Tariff Trade Agreement offer on %s, got %v", rows[0].name, target.TreatyOffers)
	}
	if got := w.World.Relation(p, target); got != "Free Trade Agreement" {
		t.Errorf("an unaccepted proposal must not change the standing relation, got %q", got)
	}
}
