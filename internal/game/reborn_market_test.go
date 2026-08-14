package game

import "testing"

// The trading market is keyed by realm NAME, like the treaty rows, so it leaks
// the same way: a name freed by one realm's death can be claimed by the next
// caller to onboard, who would find the dead realm's escrowed goods and unpaid
// sale gold waiting under it.
func TestRebornRealmHoldsNoMarketPositionOfItsPredecessor(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-02"
	doomed := w.AddHuman("owner", "Doomed")
	buyer := w.AddHuman("buyer", "Buyer")
	for _, e := range []*Empire{doomed, buyer} {
		// Played today, so the never-played and idle sweeps leave them alone and the
		// husk is the only realm this maintenance collects.
		e.LastPlayed = "2026-07-03"
	}
	doomed.Troopers = 1_000
	buyer.Gold = 1_000_000
	// Priced far above the shop so no AI baron shops the listing out from under
	// the test while maintenance runs.
	if err := w.SetMarketListing(doomed, "Trooper", 1_000, 1_000); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := w.BuyFromMarket(buyer, "Doomed", "Trooper", 400); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if n, gold := w.MarketForSale("Doomed", "Trooper"), w.MarketProceeds["Doomed"]; n != 600 || gold != 400_000 {
		t.Fatalf("the listing and sale did not take (listed=%d, proceeds=%d), so nothing below is tested", n, gold)
	}

	doomed.Land = 0 // ground to nothing: dies as the day rolls
	w.DailyMaintenance("2026-07-03")

	if w.FindByName("Doomed") != nil {
		t.Fatal("the husk was not collected, so nothing below tests the reborn case")
	}
	if n := w.MarketForSale("Doomed", "Trooper"); n != 0 {
		t.Errorf("%d escrowed troopers outlived the realm that listed them", n)
	}
	// A listing left standing is still sellable and the gold still accrues to the
	// name, so try to buy off it before the name is claimed again. The sale must be
	// refused; the error is deliberately ignored, since being refused is the point.
	buyer = w.FindByOwner("buyer")
	if buyer == nil {
		t.Fatal("the buyer did not survive maintenance, so the orphan sale is untested")
	}
	buyer.Gold = 1_000_000 // fund the attempt outright, so what follows turns on the listing
	_ = w.BuyFromMarket(buyer, "Doomed", "Trooper", 100)

	// The freed name is claimed by the next caller to onboard.
	reborn := w.AddHuman("newcomer", "Doomed")
	troopers, gold := reborn.Troopers, reborn.Gold
	if err := w.SetMarketListing(reborn, "Trooper", 0, 0); err != nil {
		t.Fatalf("delist: %v", err)
	}
	if reborn.Troopers != troopers {
		t.Errorf("the reborn realm delisted %d of its predecessor's escrowed troopers", reborn.Troopers-troopers)
	}
	w.settleMarketProceeds()
	if reborn.Gold != gold {
		t.Errorf("the reborn realm was paid %d gold for its predecessor's sales", reborn.Gold-gold)
	}
}

// Abdication frees the realm name immediately — another caller can claim it the
// same day — so it must take the market position with it too.
func TestAbdicationDropsItsMarketPosition(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	quitter := w.AddHuman("q", "Quitter")
	buyer := w.AddHuman("b", "Buyer")
	quitter.Tanks = 10
	buyer.Gold = 100_000
	if err := w.SetMarketListing(quitter, "Tank", 10, 1_000); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := w.BuyFromMarket(buyer, "Quitter", "Tank", 4); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if n, gold := w.MarketForSale("Quitter", "Tank"), w.MarketProceeds["Quitter"]; n != 6 || gold != 4_000 {
		t.Fatalf("the listing and sale did not take (listed=%d, proceeds=%d), so nothing below is tested", n, gold)
	}

	w.RemoveEmpire(quitter)

	if w.FindByName("Quitter") != nil {
		t.Fatal("Quitter was not removed, so nothing below tests the abdication case")
	}
	if n := w.MarketForSale("Quitter", "Tank"); n != 0 {
		t.Errorf("%d escrowed tanks outlived the realm that listed them", n)
	}
	reborn := w.AddHuman("second", "Quitter")
	tanks, gold := reborn.Tanks, reborn.Gold
	if err := w.SetMarketListing(reborn, "Tank", 0, 0); err != nil {
		t.Fatalf("delist: %v", err)
	}
	if reborn.Tanks != tanks {
		t.Errorf("the reborn realm delisted %d of its predecessor's escrowed tanks", reborn.Tanks-tanks)
	}
	w.settleMarketProceeds()
	if reborn.Gold != gold {
		t.Errorf("the reborn realm was paid %d gold for its predecessor's sale", reborn.Gold-gold)
	}
}
