package game

import "testing"

// The starting realm sits just UNDER its carrying capacity, so a new baron who
// changes nothing gains people rather than losing them. This is the whole point
// of PopBREUnitScale: BRE's capacity weights are per million and IB counts
// twenty people to that unit, so leaving the conversion out puts the same realm
// sixteen times over capacity and drains ~300 people on turn one.
//
// The figures are BRE's, from a live new-realm capture: the identical region
// mix reads "Population: 100 Million" against a capacity of 121, and BRE's own
// first turn moves it by one.
func TestANewRealmStartsBelowItsCapacity(t *testing.T) {
	e := &Empire{Support: 100, Tax: StartTax, People: StartPeople}
	e.Regions.Agricultural = StartAgricultural
	e.Regions.Desert = StartDesert
	e.Regions.Mountain = StartMountain
	e.Regions.Coastal = StartCoastal

	capacity := e.popCapacity()
	if capacity <= e.People {
		t.Errorf("a new realm holds %d people against a capacity of %d, so it drains from its first turn",
			e.People, capacity)
	}
	// BRE's figure for that realm is 121 in its own unit. Asserted as the golden
	// literal it converts to, so retuning a weight has to come with new evidence.
	if want := 2436; capacity != want {
		t.Errorf("capacity %d, want %d (BRE's 121 per million x %d)", capacity, want, PopBREUnitScale)
	}
}

// The population moves TOWARD the capacity, on every seed rather than a lucky
// one — a fixed seed is one trajectory and would not have caught the drain,
// which held across all of them.
func TestPopulationMovesTowardCapacity(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		cfg := DefaultConfig()
		cfg.AICount = 0
		w := NewWorldSeed(cfg, seed)
		e := w.AddHuman("tester", "Testland")
		before := e.People
		w.PlayTurn(e, "2026-08-14")
		// Support can fall and a random event can take people, so this asserts
		// the direction of migration itself rather than the turn's net figure.
		if e.LastPopGrowth <= 0 {
			t.Errorf("seed %d: a realm %d under capacity migrated by %d; it should be gaining",
				seed, e.popCapacity()-before, e.LastPopGrowth)
		}
	}
}

// An empty granary does not stop migration. BRE's end-of-turn routine
// (BRE.OVR 0xD219-0xD3CC) reads the region counts, population, support and tax
// and never touches the food field at +0x221 — a food shortfall costs support,
// morale and, at 50% severity, a civil war, and nothing else. IB gated growth
// on Food > 0 from its own pre-binary logistic model (6ace5fd) and kept the
// gate through the rewrite, so a realm that had fed its people in full at the
// maintenance prompts, spending the granary down to zero, silently lost its
// whole turn's growth.
func TestAnEmptyGranaryDoesNotStopGrowth(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		cfg := DefaultConfig()
		cfg.AICount = 0
		w := NewWorldSeed(cfg, seed)
		e := w.AddHuman("tester", "Testland")
		e.Food = 0
		w.PlayTurn(e, "2026-08-14")
		if e.LastPopGrowth <= 0 {
			t.Errorf("seed %d: a realm under capacity with no food migrated by %d; food does not gate growth",
				seed, e.LastPopGrowth)
		}
	}
}

// Waste houses nobody. BRE's capacity routine (BRE.OVR 0xD08A) loads eight
// region counts and never reads the ninth, Waste at +0xb6 — so ruined land
// contributes nothing to carrying capacity. Asserted as a golden literal rather
// than against PopCapWaste, so giving Waste a weight has to come with evidence.
func TestWasteHousesNobody(t *testing.T) {
	base := &Empire{Support: 90, Tax: 10}
	base.Regions.Urban = 10
	before := base.popCapacity()

	ruined := *base
	ruined.Regions.Waste = 500
	if got := ruined.popCapacity(); got != before {
		t.Errorf("500 waste regions changed capacity from %d to %d; waste must house nobody", before, got)
	}

	// And the contrast: any other type does move it, so the test above is not
	// passing because popCapacity ignores its input.
	moved := *base
	moved.Regions.Desert = 500
	if moved.popCapacity() == before {
		t.Error("desert regions should raise capacity; the comparison above proves nothing")
	}
}
