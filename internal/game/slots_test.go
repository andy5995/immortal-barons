package game

import "testing"

// A realm's letter is its identity for the whole game, so a neighbour falling
// and being swept must not renumber it. Before slots the letter was the empire's
// position in the Empires slice, and a removal moved every realm below the gap
// up one — the key that mailed a realm yesterday reached a different one today.
func TestSlotLetterSurvivesANeighboursRemoval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	first := w.AddHuman("a", "Alpha")
	second := w.AddHuman("b", "Bravo")
	third := w.AddHuman("c", "Charlie")

	// Golden literals: slot 1 is A, and the third realm founded is C.
	if got := w.EmpireLetter(first); got != "A" {
		t.Fatalf("first realm lettered %q, want A", got)
	}
	if got := w.EmpireLetter(third); got != "C" {
		t.Fatalf("third realm lettered %q, want C", got)
	}

	w.RemoveEmpire(second)

	if got := w.EmpireLetter(third); got != "C" {
		t.Errorf("after B was removed the third realm is lettered %q, want C — a letter must not move", got)
	}
	if got := w.EmpireLetter(first); got != "A" {
		t.Errorf("after B was removed the first realm is lettered %q, want A", got)
	}
}

// A planet holds 25 realms and no more: BRE keeps a fixed 25-entry array and
// addresses each entry by the letter that indexes it, so a 26th realm could be
// named by nobody (#144).
func TestPlanetHoldsTwentyFiveRealms(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	for i := 0; i < 25; i++ {
		if e := w.AddHuman(handleFor(i), realmFor(i)); e == nil {
			t.Fatalf("realm %d of 25 was refused a slot", i+1)
		}
	}
	if !w.PlanetFull() {
		t.Fatal("PlanetFull() = false with 25 realms")
	}
	if e := w.AddHuman("surplus", "Surplusia"); e != nil {
		t.Fatalf("a 26th realm was created in slot %d; the planet holds 25", e.Slot)
	}
	if len(w.Empires) != 25 {
		t.Errorf("world holds %d empires, want 25", len(w.Empires))
	}
	if !w.BoardFull() {
		t.Error("BoardFull() = false on a full planet")
	}
	// Every letter A..Y is used exactly once, and none strays past Y into the
	// Z the pickers reserve for "All".
	seen := map[string]bool{}
	for _, e := range w.Empires {
		l := e.Letter()
		if l < "A" || l > "Y" {
			t.Errorf("realm %q lettered %q, outside A..Y", e.Name, l)
		}
		if seen[l] {
			t.Errorf("two realms answer to %q", l)
		}
		seen[l] = true
	}
}

// Max Players Per BBS counts callers only, so it never bounded the roster the
// pickers letter: computer barons sit in the same slice with no owner. 0 used to
// mean "unlimited" and bounded nothing at all. Both now stop at the slots.
func TestBaronsAndCallersShareThePlanetsSlots(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	cfg.MaxPlayers = 0 // "no cap of my own"
	w := NewWorldSeed(cfg, 1)

	if got := w.AddAIEmpires(20); got != 20 {
		t.Fatalf("seeded %d barons, want 20", got)
	}
	for i := 0; i < 5; i++ {
		if e := w.AddHuman(handleFor(i), realmFor(i)); e == nil {
			t.Fatalf("caller %d was refused a slot with %d free", i+1, 5-i)
		}
	}
	if e := w.AddHuman("surplus", "Surplusia"); e != nil {
		t.Fatalf("20 barons plus 6 callers is 26 realms; the sixth caller took slot %d", e.Slot)
	}
}

// A slot freed by a prune is available again at once — otherwise a long-running
// board fills up for good — and its new occupant inherits nothing from the realm
// that held it. Everything a realm leaves behind keys on its NAME, and
// dropEmpires forgets all of it.
func TestFreedSlotIsReusedAndInheritsNothing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	ally := w.AddHuman("ally", "Allyria")
	doomed := w.AddHuman("doomed", "Doomedia")
	for i := 0; i < 23; i++ { // fill the rest of the planet
		w.AddHuman(handleFor(i), realmFor(i))
	}
	if !w.PlanetFull() {
		t.Fatal("the planet should be full before a slot is freed")
	}
	w.ProposeTreaty(ally, doomed, fullDefenseAlliance)
	if !w.AcceptTreaty(doomed, ally.Name, fullDefenseAlliance) {
		t.Fatal("the pact under test was never formed")
	}
	doomed.Mail = append(doomed.Mail, Message{From: "Allyria", Body: "hold the line"})

	// Doomedia falls and daily maintenance sweeps the husk.
	doomed.Alive = false
	doomed.DiedDay = w.GameDay
	w.GameDay++
	w.removeDeadHusks()

	if w.PlanetFull() {
		t.Fatal("the freed slot did not come back")
	}
	heir := w.AddHuman("heir", "Heiria")
	if heir == nil {
		t.Fatal("no realm could be founded in the freed slot")
	}
	if heir.Slot != 2 {
		t.Errorf("the heir took slot %d, want 2 — the freed one", heir.Slot)
	}
	if len(heir.Mail) != 0 {
		t.Errorf("the heir inherited %d messages", len(heir.Mail))
	}
	if held := w.TreatiesBetween(ally, heir); len(held) != 0 {
		t.Errorf("the heir inherited its predecessor's pacts: %v", held)
	}
}

// EnsureSlots settles a world saved before realms had a slot, and runs on EVERY
// reload — so it must renumber nobody the second time.
func TestEnsureSlotsBackfillsOnceAndIsIdempotent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	for i := 0; i < 4; i++ {
		w.AddHuman(handleFor(i), realmFor(i))
	}
	for _, e := range w.Empires { // as a pre-slot save loads: the field is absent
		e.Slot = 0
	}

	w.EnsureSlots()
	first := map[string]int{}
	for i, e := range w.Empires {
		if e.Slot != i+1 {
			t.Errorf("realm %d backfilled to slot %d, want %d (saved order)", i, e.Slot, i+1)
		}
		first[e.Name] = e.Slot
	}

	w.EnsureSlots()
	for _, e := range w.Empires {
		if e.Slot != first[e.Name] {
			t.Errorf("a second EnsureSlots moved %q from slot %d to %d", e.Name, first[e.Name], e.Slot)
		}
	}
}

// A world saved when nothing bounded the roster can hold more realms than the
// planet has slots. The surplus was addressable by nobody, which is the defect
// slots exist to end, so it is removed rather than left in place — and the
// callers keep their slots ahead of the computer barons.
func TestEnsureSlotsDropsAnOverFullWorldsSurplus(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(24)
	late := w.AddHuman("late", "Lateland") // the 25th
	for _, e := range w.Empires {
		e.Slot = 0
	}
	// Two more realms than the planet holds, as an unbounded save could carry.
	w.Empires = append(w.Empires, newEmpire("Surplus One", "", w.Config, w.GameDay))
	w.Empires = append(w.Empires, newEmpire("Surplus Two", "", w.Config, w.GameDay))

	w.EnsureSlots()

	if len(w.Empires) != 25 {
		t.Fatalf("world holds %d realms after the migration, want 25", len(w.Empires))
	}
	if w.FindByOwner("late") == nil {
		t.Error("a caller's realm was dropped while computer barons kept their slots")
	}
	if late.Slot < 1 || late.Slot > 25 {
		t.Errorf("the caller's realm holds slot %d, outside 1..25", late.Slot)
	}
	seen := map[int]bool{}
	for _, e := range w.Empires {
		if e.Slot < 1 || e.Slot > 25 {
			t.Errorf("realm %q left with slot %d", e.Name, e.Slot)
		}
		if seen[e.Slot] {
			t.Errorf("slot %d is held twice", e.Slot)
		}
		seen[e.Slot] = true
	}
}

func handleFor(i int) string { return string(rune('a'+i)) + "handle" }
func realmFor(i int) string  { return "Realm" + string(rune('A'+i)) }
