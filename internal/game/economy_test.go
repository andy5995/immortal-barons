package game

import "testing"

func TestLandPriceRisesWithHoldings(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0
	base := w.LandPrice(e)
	if base != w.Prices.Land {
		t.Errorf("LandPrice at Land=0: want %d, got %d", w.Prices.Land, base)
	}

	e.Land = 50
	higher := w.LandPrice(e)
	if higher <= base {
		t.Errorf("LandPrice should rise with holdings: base=%d higher=%d", base, higher)
	}
}

func TestBuyLandIncremental(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	// regionCost climbs with holdings, so buying 5 regions from Land=0 costs the
	// sum of the first five rising prices.
	e.Regions = RegionMix{}
	e.Land = 0
	e.Gold = 1_000_000
	startGold := e.Gold

	const n = 5
	total := 0
	for i := 0; i < n; i++ {
		total += w.regionCost(e.Land + i)
	}

	if err := w.BuyLand(e, n); err != nil {
		t.Fatalf("BuyLand: %v", err)
	}
	if e.Land != n {
		t.Errorf("Land: want %d, got %d", n, e.Land)
	}
	if want := startGold - total; e.Gold != want {
		t.Errorf("Gold: want %d, got %d", want, e.Gold)
	}
}

func TestBuyLandRejectsWhenBroke(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0

	const n = 5
	total := 0
	for i := 0; i < n; i++ {
		total += w.regionCost(e.Land + i)
	}
	e.Gold = total - 1
	startGold := e.Gold

	if err := w.BuyLand(e, n); err != ErrCantAfford {
		t.Errorf("BuyLand: want ErrCantAfford, got %v", err)
	}
	if e.Land != 0 {
		t.Errorf("Land should be unchanged, got %d", e.Land)
	}
	if e.Gold != startGold {
		t.Errorf("Gold should be unchanged: want %d, got %d", startGold, e.Gold)
	}
}

func TestSellLandRefundsHalf(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0
	e.Gold = 1_000_000
	startGold := e.Gold

	if err := w.BuyLand(e, 5); err != nil {
		t.Fatalf("BuyLand: %v", err)
	}
	if e.Land != 5 {
		t.Fatalf("Land: want 5, got %d", e.Land)
	}

	if err := w.SellLand(e, 5); err != nil {
		t.Fatalf("SellLand: %v", err)
	}
	if e.Land != 0 {
		t.Errorf("Land: want 0, got %d", e.Land)
	}
	if e.Gold >= startGold {
		t.Errorf("buy-then-sell should lose money: start=%d end=%d", startGold, e.Gold)
	}
}

func TestBuyFoodMarket(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 100_000
	e.Food = 0

	if err := w.BuyFoodMarket(e, 10); err != nil {
		t.Fatalf("BuyFoodMarket: %v", err)
	}
	if e.Gold != 100_000-10*w.FoodBuyPrice() {
		t.Errorf("Gold: want %d, got %d", 100_000-10*w.FoodBuyPrice(), e.Gold)
	}
	if e.Food != 10 {
		t.Errorf("Food: want 10, got %d", e.Food)
	}
}

func TestFoodMarketSupplyDepletesAndReplenishes(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	start := w.FoodMarketSupply
	if err := w.BuyFoodMarket(e, 100); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if w.FoodMarketSupply != start-100 {
		t.Errorf("supply after buy: want %d, got %d", start-100, w.FoodMarketSupply)
	}
	if err := w.SellFood(e, 40); err != nil {
		t.Fatalf("sell: %v", err)
	}
	if w.FoodMarketSupply != start-60 {
		t.Errorf("supply after sell: want %d, got %d", start-60, w.FoodMarketSupply)
	}
}

func TestFoodMarketOutOfFood(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	w.FoodMarketSupply = 0
	if err := w.BuyFoodMarket(e, 10); err != ErrNoFoodSupply {
		t.Errorf("want ErrNoFoodSupply, got %v", err)
	}
	w.FoodMarketSupply = 5 // buying clamps to what's left today
	if err := w.BuyFoodMarket(e, 10); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if e.Food != 5 || w.FoodMarketSupply != 0 {
		t.Errorf("should buy only the 5 available: food=%d supply=%d", e.Food, w.FoodMarketSupply)
	}
}

func TestFoodUnlimitedIgnoresSupply(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FoodUnlimited = true
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	w.FoodMarketSupply = 0 // empty pool, but unlimited mode ignores it
	if err := w.BuyFoodMarket(e, 100); err != nil {
		t.Fatalf("unlimited buy: %v", err)
	}
	if e.Food != 100 || w.FoodMarketSupply != 0 {
		t.Errorf("unlimited: want 100 food and untouched pool, got food=%d supply=%d", e.Food, w.FoodMarketSupply)
	}
}

func TestFoodMarketRefillsDaily(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.FoodMarketSupply = 123
	w.DailyMaintenance("2026-07-04")
	if w.FoodMarketSupply != FoodMarketDailySupply {
		t.Errorf("supply should refill to %d, got %d", FoodMarketDailySupply, w.FoodMarketSupply)
	}
}

func TestBuyFoodMarketCantAfford(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 5
	e.Food = 3

	if err := w.BuyFoodMarket(e, 10); err != ErrCantAfford {
		t.Fatalf("BuyFoodMarket: want ErrCantAfford, got %v", err)
	}
	if e.Gold != 5 || e.Food != 3 {
		t.Errorf("state should not mutate on failed buy: gold=%d food=%d", e.Gold, e.Food)
	}
}

func TestSellFood(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 0
	e.Food = 100

	if err := w.SellFood(e, 30); err != nil {
		t.Fatalf("SellFood: %v", err)
	}
	if e.Gold != 30*w.FoodSellPrice() {
		t.Errorf("Gold: want %d, got %d", 30*w.FoodSellPrice(), e.Gold)
	}
	if e.Food != 70 {
		t.Errorf("Food: want 70, got %d", e.Food)
	}
}

func TestSellFoodClampedToOwned(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 0
	e.Food = 5

	if err := w.SellFood(e, 100); err != nil {
		t.Fatalf("SellFood: %v", err)
	}
	if e.Food != 0 {
		t.Errorf("Food: want 0, got %d", e.Food)
	}
	if e.Gold != 5*w.FoodSellPrice() {
		t.Errorf("Gold: want %d, got %d", 5*w.FoodSellPrice(), e.Gold)
	}
}

func TestStartHQ(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = HQCost

	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	if e.HQ != 5 {
		t.Errorf("HQ: want 5, got %d", e.HQ)
	}
	if e.Gold != 0 {
		t.Errorf("Gold: want 0, got %d", e.Gold)
	}
}

func TestStartHQAlreadyStarted(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = HQCost * 2
	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	goldBefore := e.Gold
	if err := w.StartHQ(e); err != ErrHQExists {
		t.Errorf("second StartHQ: want ErrHQExists, got %v", err)
	}
	if e.Gold != goldBefore {
		t.Errorf("second StartHQ should not charge again: gold %d -> %d", goldBefore, e.Gold)
	}
}

func TestStartHQCantAfford(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = HQCost - 1

	if err := w.StartHQ(e); err != ErrCantAfford {
		t.Errorf("StartHQ: want ErrCantAfford, got %v", err)
	}
	if e.HQ != 0 {
		t.Errorf("HQ should remain 0, got %d", e.HQ)
	}
	if e.Gold != HQCost-1 {
		t.Errorf("Gold should be unchanged, got %d", e.Gold)
	}
}

func TestSellUnitsThirdPrice(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Troopers = 10
	e.Gold = 0

	// Clamped to owned: selling more than owned only sells what's owned.
	if err := w.SellTroopers(e, 15); err != nil {
		t.Fatalf("SellTroopers: %v", err)
	}
	if e.Troopers != 0 {
		t.Errorf("Troopers: want 0, got %d", e.Troopers)
	}
	wantGold := 10 * w.Prices.Trooper / 3
	if e.Gold != wantGold {
		t.Errorf("Gold: want %d, got %d", wantGold, e.Gold)
	}

	// Selling a partial amount only removes n and pays n*price/3.
	e.Jets = 8
	e.Gold = 0
	if err := w.SellJets(e, 3); err != nil {
		t.Fatalf("SellJets: %v", err)
	}
	if e.Jets != 5 {
		t.Errorf("Jets: want 5, got %d", e.Jets)
	}
	wantGold = 3 * w.Prices.Jet / 3
	if e.Gold != wantGold {
		t.Errorf("Gold: want %d, got %d", wantGold, e.Gold)
	}
}

func TestTechBoostsIncomeAndCutsMaintenance(t *testing.T) {
	cfg := DefaultConfig()

	setup := func(regions RegionMix) (*World, *Empire) {
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("me", "Mine")
		e.Regions = regions
		e.syncLand()
		e.Gold = 0
		e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Carriers = 100, 20, 30, 10, 5
		return w, e
	}

	wBase, base := setup(RegionMix{Coastal: 100})
	wTech, tech := setup(RegionMix{Coastal: 60, Technology: 40})

	wBase.CollectIncome(base)
	wTech.CollectIncome(tech)

	if tech.Gold <= base.Gold {
		t.Errorf("Technology empire should net more gold: base=%d tech=%d", base.Gold, tech.Gold)
	}
	if tech.ForcesUpkeep() >= base.ForcesUpkeep() {
		t.Errorf("Technology empire should have lower upkeep: base=%d tech=%d", base.ForcesUpkeep(), tech.ForcesUpkeep())
	}
}

func TestMaxAffordableRegionsIsTrulyAffordable(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 1_000_000
	max := w.MaxAffordableRegions(e)
	if max <= 0 {
		t.Fatalf("expected to afford some regions with 1,000,000 gold, got %d", max)
	}

	// Buying exactly the reported max must succeed against a fresh identical
	// empire; one more must fail — that was the bug (prompt offered a max the
	// rising price made unaffordable).
	ok := w.AddHuman("ok", "OK")
	ok.Gold = 1_000_000
	if err := w.BuyRegions(ok, &ok.Regions.Coastal, max); err != nil {
		t.Errorf("buying the affordable max (%d) should succeed, got %v", max, err)
	}
	over := w.AddHuman("over", "Over")
	over.Gold = 1_000_000
	if err := w.BuyRegions(over, &over.Regions.Coastal, max+1); err == nil {
		t.Errorf("buying one more than the max (%d+1) should have failed", max)
	}
}

func TestRegionPurchaseCapIsCumulativePerTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.MaxRegions = 5
	e := w.AddHuman("tester", "Testland")
	e.Gold = 1_000_000
	startLand := e.Land

	// Buying up to the cap works.
	if err := w.BuyLand(e, 3); err != nil {
		t.Fatalf("BuyLand(3): %v", err)
	}
	if err := w.BuyLand(e, 2); err != nil {
		t.Fatalf("BuyLand(2): %v", err)
	}
	if e.RegionsBoughtThisTurn != 5 {
		t.Fatalf("RegionsBoughtThisTurn: want 5, got %d", e.RegionsBoughtThisTurn)
	}

	// A further purchase in the same turn — even a single region, even after
	// returning to the Spending menu — must be blocked at the cap, not
	// allowed to reset per action.
	if err := w.BuyLand(e, 1); err != ErrRegionCap {
		t.Errorf("BuyLand(1) over the per-turn cap: want ErrRegionCap, got %v", err)
	}
	if e.Land != startLand+5 {
		t.Errorf("Land should be unchanged by the rejected purchase, want %d, got %d", startLand+5, e.Land)
	}

	// Once the turn advances, the counter resets and the cap is available
	// again (mirrors the reset in runTurn, internal/menu/gameflow.go).
	e.RegionsBoughtThisTurn = 0
	if err := w.BuyLand(e, 5); err != nil {
		t.Fatalf("BuyLand(5) after turn reset: %v", err)
	}
	if e.Land != startLand+10 {
		t.Errorf("Land: want %d, got %d", startLand+10, e.Land)
	}
}

func TestFoodNeededNextTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.People = 100
	e.Troopers = 10
	e.Jets = 5
	e.Tanks = 3

	want := 100*PeopleFoodPerThousand/1000 + 10 + 5*2 + 3*2
	if got := w.FoodNeededNextTurn(e); got != want {
		t.Errorf("FoodNeededNextTurn: want %d, got %d", want, got)
	}
}

func TestBuildAndSellBombers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 1_000_000

	if err := w.BuildBombers(e, 10); err != nil {
		t.Fatal(err)
	}
	if e.Bombers != 10 {
		t.Errorf("expected 10 bombers, got %d", e.Bombers)
	}
	before := e.Gold
	if err := w.SellBombers(e, 5); err != nil {
		t.Fatal(err)
	}
	if e.Bombers != 5 {
		t.Errorf("expected 5 bombers after selling, got %d", e.Bombers)
	}
	if e.Gold <= before {
		t.Error("selling bombers should add gold")
	}
}
