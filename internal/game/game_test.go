package game

import (
	"strings"
	"testing"
)

func testWorld() *World {
	cfg := DefaultConfig()
	cfg.AICount = 2
	return NewWorldSeed(cfg, 1)
}

// pastProtection clears New Realm Protection on every realm in w. Every trade
// path refuses a protected realm — BRE defines the setting as the turns for
// which a new empire "is unable to attack, trade, and be attacked" — so a
// fixture that exercises trading has to start past it, the same way the combat
// fixtures already zero it before attacking.
func pastProtection(w *World) *World {
	for _, e := range w.Empires {
		e.Protection = 0
	}
	return w
}

// pactAll puts ttype between every pair of realms in w. Sending a trade deal
// needs any pact (BRE), and buying from a listing needs one of
// MarketAccessTreaties (IB's own rule), so a fixture that trades has to
// establish one. Full Defense Alliance satisfies both.
func pactAll(w *World, ttype string) *World {
	for i, a := range w.Empires {
		for _, b := range w.Empires[i+1:] {
			w.setRelation(a.Name, b.Name, ttype)
		}
	}
	return w
}

func TestNewWorldSeedsAIOnly(t *testing.T) {
	w := testWorld()
	if len(w.Empires) != 2 {
		t.Fatalf("want 2 AI empires, got %d", len(w.Empires))
	}
	for _, e := range w.Empires {
		if e.Owner != "" {
			t.Errorf("AI empire should have empty Owner, got %q", e.Owner)
		}
	}
}

func TestAddHumanAndFindByOwner(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("Khan", "New Barony")
	if e.Owner != "khan" {
		t.Errorf("owner should be normalized to lowercase, got %q", e.Owner)
	}
	if e.Name != "New Barony" {
		t.Errorf("realm name: got %q", e.Name)
	}
	if w.FindByOwner("KHAN") != e {
		t.Error("FindByOwner should match case-insensitively")
	}
	if w.FindByOwner("nobody") != nil {
		t.Error("FindByOwner should return nil for unknown handle")
	}
}

func TestAddAIEmpires(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	if got := w.AddAIEmpires(3); got != 3 {
		t.Fatalf("AddAIEmpires(3) added %d, want 3", got)
	}
	if len(w.Empires) != 3 {
		t.Fatalf("want 3 empires, got %d", len(w.Empires))
	}
	seen := map[string]bool{}
	for _, e := range w.Empires {
		if e.Owner != "" {
			t.Errorf("injected empire should be AI (empty Owner), got %q", e.Owner)
		}
		if e.Jets != 5 || e.Turrets != 40 {
			t.Errorf("AI setup: Jets=%d Turrets=%d, want 5/40", e.Jets, e.Turrets)
		}
		key := strings.ToLower(e.Name)
		if seen[key] {
			t.Errorf("duplicate AI name %q", e.Name)
		}
		seen[key] = true
	}

	// A second call skips names already used and keeps names distinct.
	if got := w.AddAIEmpires(2); got != 2 {
		t.Fatalf("second AddAIEmpires(2) added %d, want 2", got)
	}
	names := map[string]bool{}
	for _, e := range w.Empires {
		key := strings.ToLower(e.Name)
		if names[key] {
			t.Errorf("second call duplicated name %q", e.Name)
		}
		names[key] = true
	}
	if len(w.Empires) != 5 {
		t.Fatalf("want 5 empires after two calls, got %d", len(w.Empires))
	}
}

// The name pool (576 combinations) is far larger than the planet, so what stops
// AddAIEmpires is the slot count, not the names. Barons share the roster the
// pickers letter, so a board cannot be given more of them than can be addressed.
func TestAddAIEmpiresStopsAtPlanetSlots(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	if got := w.AddAIEmpires(100); got != 25 {
		t.Fatalf("requesting 100 barons added %d, want 25 (the planet's slots)", got)
	}
	if len(w.Empires) != 25 {
		t.Fatalf("want 25 empires, got %d", len(w.Empires))
	}
	if got := w.AddAIEmpires(1); got != 0 {
		t.Errorf("a full planet should add 0, got %d", got)
	}
	if !w.PlanetFull() {
		t.Error("PlanetFull() = false with every slot held")
	}
}

// The name pool itself still has an exact end, tested where it can be reached:
// through newAIName, which AddAIEmpires can no longer drain now that the planet
// runs out of slots first.
func TestNewAINameExhaustsPool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	pool := len(aiNameModifiers) * len(aiNameNouns) // every modifier x noun combination
	used := map[string]bool{}
	for i := 0; i < pool; i++ {
		name, ok := w.newAIName(used)
		if !ok {
			t.Fatalf("pool ran dry after %d names, want %d", i, pool)
		}
		if used[strings.ToLower(name)] {
			t.Fatalf("newAIName repeated %q", name)
		}
		used[strings.ToLower(name)] = true
	}
	if _, ok := w.newAIName(used); ok {
		t.Error("an exhausted pool still offered a name")
	}
}

func TestTargetsExcludeSelfAndProtected(t *testing.T) {
	w := testWorld()
	me := w.AddHuman("me", "Mine")
	me.Protection = 0
	w.Empires[0].Protection = 5 // protected AI
	w.Empires[1].Protection = 0
	got := w.Targets(me)
	for _, e := range got {
		if e == me {
			t.Error("Targets must not include the attacker")
		}
		if e.Protection > 0 {
			t.Error("Targets must exclude protected empires")
		}
	}
	if len(got) != 1 {
		t.Errorf("want 1 targetable empire, got %d", len(got))
	}
}

func TestHQBoostsTanks(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("tester", "Testland")
	e.Tanks = 10

	e.HQ = 0
	baseOffense := e.Offense()
	baseDefense := e.Defense()

	e.HQ = 100
	boostedOffense := e.Offense()
	boostedDefense := e.Defense()

	if boostedOffense <= baseOffense {
		t.Errorf("Offense should rise with HQ=100: base=%d boosted=%d", baseOffense, boostedOffense)
	}
	if boostedDefense <= baseDefense {
		t.Errorf("Defense should rise with HQ=100: base=%d boosted=%d", baseDefense, boostedDefense)
	}

	// A finished HeadQuarters adds one trooper's worth per tank: the binary's
	// tank term is (1.75 + HQ/200) against a trooper's 0.5, so 0.5 of a tank
	// over the full range, which is 1 trooper once both are doubled.
	wantDelta := e.Tanks * 1
	if boostedOffense-baseOffense != wantDelta {
		t.Errorf("Offense delta: want %d, got %d", wantDelta, boostedOffense-baseOffense)
	}
	if boostedDefense-baseDefense != wantDelta {
		t.Errorf("Defense delta: want %d, got %d", wantDelta, boostedDefense-baseDefense)
	}
}

// TestTankStrengthMatchesBRE pins the curve read out of the regular-attack
// routine (BRE.OVR 0xF2E4): the tank term is (1.75 + HQ/200) against a
// trooper's 0.5, so in whole troopers a tank is worth 3.5 at HQ 0, 4 at 50
// (breins.txt's "equivalent of four Troopers"), and 4.5 at 100. Fidelity
// contract — a failure means IB has stopped matching, not that the numbers need
// updating. The earlier 3..5 curve came from reading the manual's "four
// Troopers" as a base rather than as the mid-point of the HQ term.
func TestTankStrengthMatchesBRE(t *testing.T) {
	for _, c := range []struct{ hq, want int }{{0, 350}, {25, 375}, {50, 400}, {75, 425}, {100, 450}} {
		if got := tankStrength(100, c.hq); got != c.want {
			t.Errorf("tankStrength(100 tanks, HQ %d) = %d troopers, want %d", c.hq, got, c.want)
		}
	}
}

// TestHQPriceRisesWithTurnsPlayed pins the price curve read from the original:
// base + 75 per lifetime turn, plus a draw under 300, capped near 100,000. The
// captured prices are checked against exactly this, so the window is the
// fidelity contract.
func TestHQPriceRisesWithTurnsPlayed(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	for _, turns := range []int{0, 1, 30, 91, 500} {
		e.TurnsPlayed = turns
		got := w.HQPrice(e)
		lo := HQPriceBase + HQPricePerTurn*turns
		if got < lo || got >= lo+HQPriceJitter {
			t.Errorf("HQPrice at %d turns = %d, want [%d, %d)", turns, got, lo, lo+HQPriceJitter)
		}
	}

	// The cap keeps a very old realm from being priced out entirely.
	e.TurnsPlayed = 100_000
	if got := w.HQPrice(e); got > HQPriceCap || got <= HQPriceCap-HQPriceCapJitter {
		t.Errorf("HQPrice for an ancient realm = %d, want just under the %d cap", got, HQPriceCap)
	}

	// Shown == charged: the price is stable across reads within one turn.
	e.TurnsPlayed = 40
	if a, b := w.HQPrice(e), w.HQPrice(e); a != b {
		t.Errorf("HQPrice is not stable within a turn: %d then %d", a, b)
	}

	// And it actually moves as the empire plays on.
	e.TurnsPlayed = 0
	young := w.HQPrice(e)
	e.TurnsPlayed = 100
	if old := w.HQPrice(e); old <= young {
		t.Errorf("an old realm should pay more: %d turns-0 vs %d turns-100", young, old)
	}
}

// TestHQBuildRate: BRE starts a bought HeadQuarters at 5% and adds 5 at the end
// of every turn while it sits in 1..99. The purchase happens during the spending
// phase, so the buying turn's own end-of-turn advance counts — 5 goes to 10 that
// same turn, and the build finishes 19 turns after the purchase.
func TestHQBuildRate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = int64(w.HQPrice(e))
	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	if e.HQ != HQBuildStart {
		t.Fatalf("a new HeadQuarters starts at %d%%, got %d", HQBuildStart, e.HQ)
	}
	turns := 0
	for e.HQ < 100 {
		w.PlayTurn(e, "2026-07-03")
		turns++
		if turns > 50 {
			t.Fatalf("HeadQuarters stalled at %d%%", e.HQ)
		}
	}
	if turns != 19 {
		t.Errorf("HeadQuarters took %d turns to finish, want 19", turns)
	}

	// A finished HeadQuarters stays finished — no advance past 100.
	w.PlayTurn(e, "2026-07-03")
	if e.HQ != 100 {
		t.Errorf("HQ should cap at 100, got %d", e.HQ)
	}
}

func TestTechFactorShape(t *testing.T) {
	// The benefit comes from banked research, not from holding regions: an empire
	// with Technology regions but no research yet reads exactly 100%.
	e := &Empire{Land: 100, Regions: RegionMix{Coastal: 80, Technology: 20}}
	if got := e.TechGoldFactor(); got != TechFactorUnit {
		t.Errorf("un-researched Technology: want %d, got %d", TechFactorUnit, got)
	}

	// It rises with the level and approaches, but never reaches, the ceiling.
	e.TechSlots[TechSlotGold] = 50
	mid := e.TechGoldFactor()
	if mid <= TechFactorUnit {
		t.Errorf("research should raise the factor, got %d", mid)
	}
	e.TechSlots[TechSlotGold] = 500 // more research, still short of saturation
	high := e.TechGoldFactor()
	ceiling := TechCapGold * TechFactorUnit / 100
	if high <= mid {
		t.Errorf("more research should not lower the factor: %d -> %d", mid, high)
	}
	if high >= ceiling {
		t.Errorf("factor %d should still be under its %d ceiling at this level", high, ceiling)
	}
	// The exponent is clamped, as in the original, so absurd research saturates
	// exactly at the ceiling rather than growing without bound.
	e.TechSlots[TechSlotGold] = 100_000_000
	if got := e.TechGoldFactor(); got != ceiling {
		t.Errorf("saturated factor = %d, want the %d ceiling", got, ceiling)
	}

	// The same slot drives three effects with different ceilings.
	if e.TechFoodFactor() <= e.TechGoldFactor() {
		t.Error("food production has a higher ceiling than gold, so it must read higher")
	}
	if e.TechUnitFactor() >= e.TechGoldFactor() {
		t.Error("unit production has a lower ceiling than gold, so it must read lower")
	}
}

// The benefit divides by total regions, so expanding dilutes banked research.
func TestTechDilutesAsTheRealmGrows(t *testing.T) {
	small := &Empire{Regions: RegionMix{Coastal: 80, Technology: 20}}
	small.TechSlots[TechSlotDecay] = 30
	big := &Empire{Regions: RegionMix{Coastal: 980, Technology: 20}}
	big.TechSlots[TechSlotDecay] = 30 // same research, larger realm

	if big.TechDecayFactor() >= small.TechDecayFactor() {
		t.Errorf("a larger realm should get less from the same research: small=%d big=%d",
			small.TechDecayFactor(), big.TechDecayFactor())
	}
}

// Research accumulates while Technology regions are held, and FREEZES when they
// are sold — it is never decremented.
func TestTechAccumulatesThenFreezes(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 40, Technology: 60}
	e.Land = e.Regions.Total()

	total := func() int {
		n := 0
		for _, v := range e.TechSlots {
			n += v
		}
		return n
	}
	if total() != 0 {
		t.Fatalf("fresh empire should have no research, got %d", total())
	}
	prev := 0
	for range 10 {
		w.advanceTech(e)
		if total() < prev {
			t.Fatalf("research must never decrease: %d -> %d", prev, total())
		}
		prev = total()
	}
	if prev <= 0 {
		t.Fatal("ten turns of Technology regions should have banked research")
	}

	// Sell every Technology region: research stops, the bank is untouched.
	e.Regions.Technology = 0
	e.Land = e.Regions.Total()
	for range 10 {
		w.advanceTech(e)
	}
	if total() != prev {
		t.Errorf("selling Technology must freeze research, not lose it: %d -> %d", prev, total())
	}
}

// A denser tech block out-researches a sparse one: points are quadratic in
// Technology regions and only inverse-linear in realm size.
func TestTechDenserBlockResearchesFaster(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	lo := w.AddHuman("lo", "Lo")
	lo.Regions = RegionMix{Coastal: 74, Technology: 26}
	lo.Land = lo.Regions.Total()
	hi := w.AddHuman("hi", "Hi")
	hi.Regions = RegionMix{Coastal: 40, Technology: 60}
	hi.Land = hi.Regions.Total()
	sum := func(e *Empire) int {
		n := 0
		for _, v := range e.TechSlots {
			n += v
		}
		return n
	}
	for range 10 {
		w.advanceTech(lo)
		w.advanceTech(hi)
	}
	if sum(hi) <= sum(lo) {
		t.Errorf("denser tech should research faster: lo(26%%)=%d hi(60%%)=%d", sum(lo), sum(hi))
	}
}

func TestTechBoostsMilitary(t *testing.T) {
	base := &Empire{Land: 100, Regions: RegionMix{Coastal: 100},
		Troopers: 100, Jets: 20, Carriers: 1, Turrets: 30, Tanks: 10}
	tech := &Empire{Land: 100, Regions: RegionMix{Technology: 40},
		Troopers: 100, Jets: 20, Carriers: 1, Turrets: 30, Tanks: 10}
	tech.TechSlots[TechSlotMilitary] = 200 // well-researched military

	if tech.Offense() <= base.Offense() {
		t.Errorf("Offense should rise with Technology regions: base=%d tech=%d", base.Offense(), tech.Offense())
	}
	if tech.Defense() <= base.Defense() {
		t.Errorf("Defense should rise with Technology regions: base=%d tech=%d", base.Defense(), tech.Defense())
	}
}

func TestRemoveEmpireAbdicate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	before := len(w.Empires)
	e := w.AddHuman("bob", "Bobland")
	if w.FindByOwner("bob") == nil {
		t.Fatal("empire not registered")
	}
	w.RemoveEmpire(e)
	if got := len(w.Empires); got != before {
		t.Errorf("empire count = %d, want %d after abdication", got, before)
	}
	if w.FindByOwner("bob") != nil {
		t.Error("abdicated empire still findable by owner")
	}
}

func TestRealmNameTaken(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.AddHuman("alice", "Khanate")
	if !w.RealmNameTaken("Khanate") {
		t.Error("exact name should be taken")
	}
	if !w.RealmNameTaken("  khanate  ") {
		t.Error("case-insensitive, trimmed name should be taken")
	}
	if w.RealmNameTaken("Empire") {
		t.Error("unused name should be free")
	}
}
