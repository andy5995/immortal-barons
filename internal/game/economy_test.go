package game

import "testing"

func TestLandPriceRisesWithHoldings(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

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
