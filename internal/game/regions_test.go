package game

import "testing"

func TestRegionMixTotalIncomeFood(t *testing.T) {
	r := RegionMix{Coastal: 2, Mountain: 3, Desert: 1, River: 1, Agricultural: 4, Urban: 5, Industrial: 1, Technology: 1}
	if got, want := r.Total(), 18; got != want {
		t.Errorf("Total: want %d, got %d", want, got)
	}
	wantFood := 4 * FoodPerAgri // Agricultural only; river fishing is conditional (World.riverFood, #29)
	if got := r.foodProduced(); got != wantFood {
		t.Errorf("foodProduced: want %d, got %d", wantFood, got)
	}
}

func TestRegionMixRemoveNeverGoesNegative(t *testing.T) {
	r := RegionMix{Coastal: 3, Mountain: 2, Desert: 0, River: 1, Agricultural: 0, Urban: 0, Industrial: 0, Technology: 0}
	total := r.Total()
	removed := r.remove(4)

	if got := removed.Total(); got != 4 {
		t.Errorf("removed.Total(): want 4, got %d", got)
	}
	if got := r.Total(); got != total-4 {
		t.Errorf("remaining.Total(): want %d, got %d", total-4, got)
	}
	if r.Coastal < 0 || r.Mountain < 0 || r.Desert < 0 || r.River < 0 ||
		r.Agricultural < 0 || r.Urban < 0 || r.Industrial < 0 || r.Technology < 0 {
		t.Errorf("remove drove a field negative: %+v", r)
	}
}

func TestRegionMixRemoveAllWhenNGreaterThanTotal(t *testing.T) {
	r := RegionMix{Coastal: 3, Mountain: 2}
	total := r.Total()
	removed := r.remove(total + 100)

	if got := removed.Total(); got != total {
		t.Errorf("removed.Total(): want %d, got %d", total, got)
	}
	if got := r.Total(); got != 0 {
		t.Errorf("remaining.Total(): want 0, got %d", got)
	}
}

func TestRegionMixAddMix(t *testing.T) {
	r := RegionMix{Coastal: 1, Urban: 2}
	r.addMix(RegionMix{Coastal: 4, Mountain: 5})
	if r.Coastal != 5 || r.Mountain != 5 || r.Urban != 2 {
		t.Errorf("addMix: got %+v", r)
	}
}

func TestDefaultRegionMixSumsToLand(t *testing.T) {
	for _, land := range []int{0, 100, 137} {
		m := defaultRegionMix(land)
		if got := m.Total(); got != land {
			t.Errorf("land=%d: Total()=%d, want %d", land, got, land)
		}
		for _, f := range (&m).fields() {
			if *f < 0 {
				t.Errorf("land=%d: negative field in %+v", land, m)
			}
		}
	}
}

func TestEnsureRegionsRepairsLegacy(t *testing.T) {
	e := &Empire{Land: 100, Regions: RegionMix{}}
	e.EnsureRegions()
	if e.Regions.Total() != 100 {
		t.Errorf("Regions.Total()=%d, want 100", e.Regions.Total())
	}
	if e.Land != 100 {
		t.Errorf("Land=%d, want 100", e.Land)
	}

	consistent := &Empire{Land: 50, Regions: RegionMix{Coastal: 50}}
	consistent.EnsureRegions()
	if consistent.Regions != (RegionMix{Coastal: 50}) {
		t.Errorf("already-consistent empire changed: %+v", consistent.Regions)
	}
	if consistent.Land != 50 {
		t.Errorf("Land changed: %d", consistent.Land)
	}
}

func assertRegionInvariant(t *testing.T, label string, e *Empire) {
	t.Helper()
	if e.Land != e.Regions.Total() {
		t.Errorf("%s: invariant broken for %s: Land=%d Regions.Total()=%d", label, e.Name, e.Land, e.Regions.Total())
	}
}

func TestNewEmpireStartsWithRegionsSummingToLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	if e.Regions.Total() != e.Land {
		t.Fatalf("Regions.Total()=%d, Land=%d", e.Regions.Total(), e.Land)
	}
	if e.Land != 15 {
		t.Errorf("expected starting Land 15, got %d", e.Land)
	}
}

func TestRegionInvariantAfterBuySellAttackNuke(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 20_000_000
	d.Troopers = 0 // make the defender an easy, deterministic loser

	if err := w.BuyRegions(a, &a.Regions.Mountain, 10); err != nil {
		t.Fatalf("BuyRegions: %v", err)
	}
	assertRegionInvariant(t, "after BuyRegions", a)

	if err := w.SellRegions(a, &a.Regions.Mountain, 3); err != nil {
		t.Fatalf("SellRegions: %v", err)
	}
	assertRegionInvariant(t, "after SellRegions", a)

	w.Attack(a, d, FullForce(a))
	assertRegionInvariant(t, "after Attack (attacker)", a)
	assertRegionInvariant(t, "after Attack (defender)", d)

	if _, err := w.NuclearStrike(a, d); err != nil {
		t.Fatalf("NuclearStrike: %v", err)
	}
	assertRegionInvariant(t, "after NuclearStrike", d)
}

func TestProcessEconomyReflectsRegionMix(t *testing.T) {
	cfg := DefaultConfig()

	wCoastal := NewWorldSeed(cfg, 1)
	coastal := wCoastal.AddHuman("c", "Coastal Realm")
	coastal.Regions = RegionMix{Coastal: 100}
	coastal.syncLand()
	coastal.Gold, coastal.Food = 0, 0

	wAg := NewWorldSeed(cfg, 1)
	ag := wAg.AddHuman("a", "Ag Realm")
	ag.Regions = RegionMix{Agricultural: 100}
	ag.syncLand()
	ag.Gold, ag.Food = 0, 0

	wCoastal.CollectIncome(coastal)  // gold income (turn start)
	wCoastal.GrowFood(coastal)       // food yield (turn start)
	wCoastal.processEconomy(coastal) // food consumption/spoilage (turn end)
	wAg.CollectIncome(ag)
	wAg.GrowFood(ag)
	wAg.processEconomy(ag)

	if coastal.Gold <= ag.Gold {
		t.Errorf("all-Coastal empire should earn more gold than all-Agricultural: coastal=%d ag=%d", coastal.Gold, ag.Gold)
	}
	if ag.Food <= coastal.Food {
		t.Errorf("all-Agricultural empire should produce more food than all-Coastal: ag=%d coastal=%d", ag.Food, coastal.Food)
	}
}
