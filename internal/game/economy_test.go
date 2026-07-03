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
	// so buying 5 regions costs 100+102+104+106+108 = 520 exactly, leaving
	// no gold for a 6th region (which would cost 110).
	e.Land = 0
	e.Gold = 520

	if err := w.BuyLand(e, 10); err != nil {
		t.Fatalf("BuyLand: %v", err)
	}
	if e.Land != 5 {
		t.Errorf("Land: want 5, got %d", e.Land)
	}
	if e.Gold != 0 {
		t.Errorf("Gold: want 0, got %d", e.Gold)
	}
}

func TestBuyLandRejectsWhenBroke(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Land = 0
	e.Gold = w.Prices.Land - 1

	if err := w.BuyLand(e, 1); err != ErrCantAfford {
		t.Errorf("BuyLand: want ErrCantAfford, got %v", err)
	}
	if e.Land != 0 {
		t.Errorf("Land should be unchanged, got %d", e.Land)
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
