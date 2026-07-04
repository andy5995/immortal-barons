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

	// Land=0, Prices.Land=100, LandPriceStep=50: price(Land) = 100 + 2*Land,
	// so buying 5 regions costs 100+102+104+106+108 = 520 exactly.
	e.Regions = RegionMix{}
	e.Land = 0
	e.Gold = 1000
	startGold := e.Gold

	const n = 5
	total := 0
	for i := 0; i < n; i++ {
		total += w.Prices.Land + (e.Land+i)*w.Prices.Land/LandPriceStep
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
		total += w.Prices.Land + (e.Land+i)*w.Prices.Land/LandPriceStep
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
	e.Gold = 10000
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
	e.Gold = 1000
	e.Food = 0

	if err := w.BuyFoodMarket(e, 10); err != nil {
		t.Fatalf("BuyFoodMarket: %v", err)
	}
	if e.Gold != 1000-10*FoodBuyPrice {
		t.Errorf("Gold: want %d, got %d", 1000-10*FoodBuyPrice, e.Gold)
	}
	if e.Food != 10 {
		t.Errorf("Food: want 10, got %d", e.Food)
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
	if e.Gold != 30*FoodSellPrice {
		t.Errorf("Gold: want %d, got %d", 30*FoodSellPrice, e.Gold)
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
	if e.Gold != 5*FoodSellPrice {
		t.Errorf("Gold: want %d, got %d", 5*FoodSellPrice, e.Gold)
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

func TestSellUnitsHalfPrice(t *testing.T) {
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
	wantGold := 10 * w.Prices.Trooper / 2
	if e.Gold != wantGold {
		t.Errorf("Gold: want %d, got %d", wantGold, e.Gold)
	}

	// Selling a partial amount only removes n and pays n*price/2.
	e.Jets = 8
	e.Gold = 0
	if err := w.SellJets(e, 3); err != nil {
		t.Fatalf("SellJets: %v", err)
	}
	if e.Jets != 5 {
		t.Errorf("Jets: want 5, got %d", e.Jets)
	}
	wantGold = 3 * w.Prices.Jet / 2
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

	wBase.processEconomy(base)
	wTech.processEconomy(tech)

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
	e.Gold = 12000
	max := w.MaxAffordableRegions(e)
	if max <= 0 {
		t.Fatalf("expected to afford some regions with 12000 gold, got %d", max)
	}

	// Buying exactly the reported max must succeed against a fresh identical
	// empire; one more must fail — that was the bug (prompt offered a max the
	// rising price made unaffordable).
	ok := w.AddHuman("ok", "OK")
	ok.Gold = 12000
	if err := w.BuyRegions(ok, &ok.Regions.Coastal, max); err != nil {
		t.Errorf("buying the affordable max (%d) should succeed, got %v", max, err)
	}
	over := w.AddHuman("over", "Over")
	over.Gold = 12000
	if err := w.BuyRegions(over, &over.Regions.Coastal, max+1); err == nil {
		t.Errorf("buying one more than the max (%d+1) should have failed", max)
	}
}

func TestFoodNeededNextTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.People = 100
	e.Troopers = 10
	e.Jets = 5
	e.Tanks = 3

	want := 100 + 10 + 5*2 + 3*2
	if got := w.FoodNeededNextTurn(e); got != want {
		t.Errorf("FoodNeededNextTurn: want %d, got %d", want, got)
	}
}
