package game

import (
	"strings"
	"testing"
)

func newAttackerAndTarget(t *testing.T) (*World, *Empire, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Protection = 0
	a.Gold = 10_000_000 // enough to afford covert/WMD op costs; gold-asserting tests reset it
	d := w.Empires[0]
	d.Protection = 0
	return w, a, d
}

func TestNuclearStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 10_000_000
	// A realm needs real size for the strike to bite: the damage is a percentage
	// of the target's regions, truncated, so a 15-region starter realm can shrug
	// off a warhead entirely (as it does in the original).
	d.Regions = defaultRegionMix(500)
	d.syncLand()
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 10_000_000-w.NukeCost(d) {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	// A nuke ruins land, it does not remove it: the total is untouched and the
	// difference shows up as waste.
	if d.Land != beforeLand {
		t.Errorf("land should be unchanged, before=%d after=%d", beforeLand, d.Land)
	}
	if d.Regions.Waste <= 0 {
		t.Errorf("expected waste regions, got %d", d.Regions.Waste)
	}
	if d.Regions.Total() != d.Land {
		t.Errorf("region mix out of sync: total=%d land=%d", d.Regions.Total(), d.Land)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
	if !strings.Contains(d.Events[len(d.Events)-1].Text, "nuclear") {
		t.Errorf("victim event should mention nuclear: %q", d.Events[len(d.Events)-1].Text)
	}
}

func TestNuclearStrikeCantAfford(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 0
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	_, err := w.NuclearStrike(a, d)
	if err != ErrCantAfford {
		t.Fatalf("expected ErrCantAfford, got %v", err)
	}
	if d.Land != beforeLand {
		t.Errorf("target land should be unchanged, before=%d after=%d", beforeLand, d.Land)
	}
	if len(d.Events) != beforeEvents {
		t.Errorf("target should have no new events")
	}
}

func TestChemicalStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 10_000_000
	beforePeople := d.People
	beforeTroopers := d.Troopers

	report, err := w.ChemicalStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 10_000_000-ChemCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.People >= beforePeople {
		t.Errorf("expected people reduced, before=%d after=%d", beforePeople, d.People)
	}
	if d.Troopers >= beforeTroopers {
		t.Errorf("expected troopers reduced, before=%d after=%d", beforeTroopers, d.Troopers)
	}
}

func TestBiologicalStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 10_000_000
	beforePeople := d.People
	beforeTroopers := d.Troopers
	beforeLand := d.Land

	report, err := w.BiologicalStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 10_000_000-BioCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.People >= beforePeople {
		t.Errorf("expected people reduced, before=%d after=%d", beforePeople, d.People)
	}
	if d.Troopers >= beforeTroopers {
		t.Errorf("expected troopers reduced, before=%d after=%d", beforeTroopers, d.Troopers)
	}
	if d.Land != beforeLand {
		t.Errorf("land should be unchanged, before=%d after=%d", beforeLand, d.Land)
	}
}

func TestRaidFactionWinLose(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)

	// An overwhelming force against the easiest faction (Humans, index 0) wins
	// against a landed faction: the captured land is DEFERRED (returned for the
	// menu picker), so the attacker's own land does not change here (#21).
	a.Troopers = 1_000_000
	a.Jets = 0
	a.Tanks = 0
	w.Pirates[0].Land = 400 // ensure the faction holds land to capture
	beforeLand := a.Land

	report, captured := w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if captured <= 0 {
		t.Errorf("expected a positive captured-land count on a win, got %d", captured)
	}
	if a.Land != beforeLand {
		t.Errorf("captured land must be deferred, not auto-added: before=%d after=%d", beforeLand, a.Land)
	}
	if a.Land != a.Regions.Total() {
		t.Errorf("Land/Regions invariant broken: Land=%d Regions.Total()=%d", a.Land, a.Regions.Total())
	}

	// A token force against the last faction (Ammonians, index 8) loses
	// and the committed troopers drop. Committed big enough that a 2-6% loss
	// does not round to zero.
	a.Troopers = 10_000
	beforeTroopers := a.Troopers

	w.Pirates[8].LootTanks = 1 << 20 // far beyond what is sent, so the loss is certain
	report, _ = w.RaidFaction(a, 8, 10_000, 0, 0)
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if a.Troopers >= beforeTroopers {
		t.Errorf("expected troopers lost on a loss, before=%d after=%d", beforeTroopers, a.Troopers)
	}
}

// The decontamination allowance is the original's: a fifth of the pile, floored
// at WasteDecontamFloor, and never more waste than the realm actually holds.
func TestDecontaminateAllowance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")

	for _, tc := range []struct{ waste, want int }{
		{0, 0},   // nothing ruined, nothing to clean
		{4, 4},   // below the floor: capped by what is held
		{40, 10}, // a fifth would be 8, so the floor of 10 wins
		{500, 100},
	} {
		e.Regions = RegionMix{Coastal: 1000, Waste: tc.waste}
		e.syncLand()
		if got := w.DecontaminateAllowance(e); got != tc.want {
			t.Errorf("allowance with %d waste = %d, want %d", tc.waste, got, tc.want)
		}
	}
}

func TestDecontaminateRestoresLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")
	e.Regions = RegionMix{Coastal: 1000, Waste: 500}
	e.syncLand()
	e.Gold = 1_000_000_000

	before := e.Land
	cost := w.DecontaminateCost(e)
	cleaned := w.Decontaminate(e, cost)

	if cleaned != 100 {
		t.Errorf("cleaned %d regions, want the whole 100-region allowance", cleaned)
	}
	if e.Regions.Waste != 400 {
		t.Errorf("waste = %d, want 400", e.Regions.Waste)
	}
	if e.Land != before {
		t.Errorf("land changed during decontamination: %d -> %d", before, e.Land)
	}
	if e.Gold != 1_000_000_000-cost {
		t.Errorf("gold = %d, want %d", e.Gold, 1_000_000_000-cost)
	}
}

// Paying part of the bill cleans proportionally less rather than nothing.
func TestDecontaminatePartialPayment(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")
	e.Regions = RegionMix{Coastal: 1000, Waste: 500}
	e.syncLand()
	e.Gold = 1_000_000_000

	price := w.DecontaminatePrice(e)
	if cleaned := w.Decontaminate(e, price*7); cleaned != 7 {
		t.Errorf("cleaned %d regions for seven regions' worth of gold, want 7", cleaned)
	}
}

// A realm holding no waste is never billed and never prompted.
func TestDecontaminateWithoutWaste(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")
	e.Gold = 1_000_000

	if got := w.DecontaminateCost(e); got != 0 {
		t.Errorf("cost with no waste = %d, want 0", got)
	}
	if got := w.Decontaminate(e, 1_000_000); got != 0 {
		t.Errorf("cleaned %d regions with no waste, want 0", got)
	}
	if e.Gold != 1_000_000 {
		t.Errorf("gold was spent with nothing to clean: %d", e.Gold)
	}
}

// The warhead is priced off the target, and the price stops climbing at the cap.
func TestNukeCostScalesWithTarget(t *testing.T) {
	if got, want := NukeCostForLand(1000), int64(1000*NukeCostPerRegion); got != want {
		t.Errorf("price for 1000 regions = %d, want %d", got, want)
	}
	if got := NukeCostForLand(1_000_000); got != NukeCostCap {
		t.Errorf("price for a huge realm = %d, want the cap %d", got, NukeCostCap)
	}
}

// The damage band is 5-9% of the target's regions, centred on 7 — assert the
// band as golden literals rather than as the constants, so a retune has to
// produce new evidence rather than following the code.
func TestNuclearStrikeDamageBand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	for seed := int64(1); seed <= 40; seed++ {
		w := NewWorldSeed(cfg, seed)
		a := w.AddHuman("att", "Attacker")
		a.Protection, a.Gold = 0, 1_000_000_000
		d := w.Empires[0]
		d.Protection = 0
		d.Regions = RegionMix{Agricultural: 1000}
		d.syncLand()

		if _, err := w.NuclearStrike(a, d); err != nil {
			t.Fatalf("seed %d: unexpected error: %v", seed, err)
		}
		if d.Regions.Waste < 50 || d.Regions.Waste > 90 {
			t.Errorf("seed %d: ruined %d of 1000 regions, want 50-90", seed, d.Regions.Waste)
		}
	}
}

// A struck AI realm cleans itself up. Without this the damage is permanent
// against an AI and temporary against a human, which is the wrong way round.
func TestAIDecontaminatesAfterAStrike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 7)
	ai := w.Empires[0]
	ai.Regions = RegionMix{Coastal: 800, Agricultural: 200, Waste: 200}
	ai.syncLand()
	ai.Gold = 1_000_000_000

	before := ai.Regions.Waste
	w.aiDecontaminate(ai)

	if ai.Regions.Waste >= before {
		t.Errorf("AI did not decontaminate: waste %d -> %d", before, ai.Regions.Waste)
	}
	if ai.Land != ai.Regions.Total() {
		t.Errorf("Land/Regions invariant broken: Land=%d total=%d", ai.Land, ai.Regions.Total())
	}
}

// A broke AI leaves the waste alone rather than spending its food reserve on it.
func TestAIKeepsItsReserveOverDecontamination(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 7)
	ai := w.Empires[0]
	ai.Regions = RegionMix{Coastal: 800, Waste: 200}
	ai.syncLand()
	ai.Gold = 0

	w.aiDecontaminate(ai)

	if ai.Regions.Waste != 200 {
		t.Errorf("a broke AI cleaned waste anyway: %d left of 200", ai.Regions.Waste)
	}
	if ai.Gold != 0 {
		t.Errorf("a broke AI spent %d gold it did not have", -ai.Gold)
	}
}

// A nuclear strike scores for the attacker, on a flat draw rather than a share
// of the damage — so even a strike that ruins nothing pays. The award band is
// asserted as golden literals, not as the constant.
func TestNuclearStrikeScoresTheAttacker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	for seed := int64(1); seed <= 25; seed++ {
		w := NewWorldSeed(cfg, seed)
		a := w.AddHuman("att", "Attacker")
		a.Protection, a.Gold, a.Score = 0, 1_000_000_000, 0
		d := w.Empires[0]
		d.Protection = 0
		d.Regions = RegionMix{Agricultural: 1000}
		d.syncLand()

		if _, err := w.NuclearStrike(a, d); err != nil {
			t.Fatalf("seed %d: unexpected error: %v", seed, err)
		}
		if a.Score < 0 || a.Score > 899 {
			t.Errorf("seed %d: scored %d, want 0-899", seed, a.Score)
		}
	}
}

// Cleaned land is restored before the owner is asked what to hold it as: the
// gold is already spent, so it must never sit in limbo waiting on a picker that
// a dropped session would never run.
func TestDecontaminateRestoresBeforeTheOwnerChooses(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")
	e.Regions = RegionMix{Coastal: 1000, Waste: 500}
	e.syncLand()
	e.Gold = 1_000_000_000

	cleaned := w.Decontaminate(e, w.DecontaminateCost(e))

	if cleaned != 100 {
		t.Fatalf("cleaned %d regions, want 100", cleaned)
	}
	if e.Regions.Coastal != 1100 {
		t.Errorf("Coastal = %d, want the 100 cleaned regions restored on top of 1000", e.Regions.Coastal)
	}
	if e.Regions.Waste != 400 {
		t.Errorf("waste = %d, want 400", e.Regions.Waste)
	}
	if e.Land != 1500 {
		t.Errorf("land = %d, want 1500 — cleaning moves land between types, it does not create or destroy it", e.Land)
	}
}

// The price scales off FOOD technology, not maintenance technology: the
// original divides by technology_factor(2.0, slot 0), the agricultural pair.
func TestDecontaminatePriceUsesFoodTechnology(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Ashfields")
	e.Regions = RegionMix{Coastal: 1000, Waste: 500}
	e.syncLand()

	plain := w.DecontaminatePrice(e)
	if want := int64(w.LandPrice(e)) / 2; plain != want {
		t.Errorf("unteched price = %d, want half the land price (%d)", plain, want)
	}

	// Research into the shared slot 0 must move the price; the maintenance slot
	// must not. Asserting both is what would catch the wrong helper being wired
	// back in, since either one alone looks plausible.
	e.TechSlots[TechSlotMaint] = 5000
	if got := w.DecontaminatePrice(e); got != plain {
		t.Errorf("maintenance research changed the price: %d -> %d", plain, got)
	}
	e.TechSlots[TechSlotGold] = 5000
	if got := w.DecontaminatePrice(e); got >= plain {
		t.Errorf("food research did not cut the price: %d -> %d", plain, got)
	}
}

// Capturing land from a realm that is part waste must not create land. The
// waste transfers as waste, and only the clean remainder is handed to the
// picker — if the returned count included the waste, the picker would grant it
// a second time as a type of the winner's choosing.
func TestCaptureFromARuinedRealmDoesNotCreateLand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 3)
	a := w.AddHuman("att", "Attacker")
	a.Protection = 0
	a.Troopers, a.Tanks = 500_000, 50_000
	d := w.Empires[0]
	d.Protection = 0
	d.Regions = RegionMix{Coastal: 500, Waste: 500}
	d.syncLand()
	d.Troopers, d.Tanks, d.Turrets = 1, 0, 0

	beforeTotal := a.Land + d.Land
	_, captured := w.Attack(a, d, AttackForce{Troopers: 500_000, Tanks: 50_000}, false)

	// The picker's share plus the waste that moved on its own equals the whole
	// transfer, and the two realms still hold every region between them.
	if got := a.Land + captured + d.Land; got != beforeTotal {
		t.Errorf("land total = %d after the picker's %d are placed, want %d", got, captured, beforeTotal)
	}
	if a.Regions.Waste <= 0 {
		t.Errorf("the attacker took no waste from a half-ruined realm (waste=%d)", a.Regions.Waste)
	}
	if a.Land != a.Regions.Total() {
		t.Errorf("Land/Regions invariant broken: Land=%d total=%d", a.Land, a.Regions.Total())
	}
}
