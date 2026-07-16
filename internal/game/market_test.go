package game

import "testing"

// listing goods escrows them out of inventory, and inventory + listed is
// conserved through a re-list and a full delist.
func TestMarketEscrowConserves(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers = 100

	if err := w.SetMarketListing(e, "Trooper", 40, 500); err != nil {
		t.Fatalf("list: %v", err)
	}
	if e.Troopers != 60 {
		t.Errorf("owned after listing 40: got %d, want 60", e.Troopers)
	}
	if got := w.MarketForSale("alice", "Trooper"); got != 40 {
		t.Errorf("for sale: got %d, want 40", got)
	}
	if got := w.MarketTotalForSale("Trooper"); got != 40 {
		t.Errorf("total for sale: got %d, want 40", got)
	}

	// Re-list a smaller amount returns the difference to inventory.
	if err := w.SetMarketListing(e, "Trooper", 10, 500); err != nil {
		t.Fatalf("relist: %v", err)
	}
	if e.Troopers != 90 || w.MarketForSale("alice", "Trooper") != 10 {
		t.Errorf("after relist to 10: owned=%d listed=%d, want 90 and 10", e.Troopers, w.MarketForSale("alice", "Trooper"))
	}

	// Delist (0) returns everything and removes the listing.
	if err := w.SetMarketListing(e, "Trooper", 0, 0); err != nil {
		t.Fatalf("delist: %v", err)
	}
	if e.Troopers != 100 {
		t.Errorf("owned after delist: got %d, want 100", e.Troopers)
	}
	if w.marketListing("alice", "Trooper") != nil {
		t.Error("empty listing should be removed")
	}
}

// listing units does not reduce force maintenance — you still pay upkeep on
// escrowed units, so the market can't be used to dodge maintenance.
func TestMarketListedGoodsStillCostMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers = 100
	before := w.ForcesDue(e)
	if err := w.SetMarketListing(e, "Trooper", 40, 500); err != nil {
		t.Fatalf("list: %v", err)
	}
	if after := w.ForcesDue(e); after != before {
		t.Errorf("listing dodged maintenance: before=%d after=%d", before, after)
	}
}

// listing food does not protect it from spoilage: two identical empires, one
// holding all its food in the granary and one with half listed, end a turn with
// the same total food.
func TestMarketListedFoodSpoilsLikeGranary(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alethia")
	b := w.AddHuman("bob", "Bobland")
	a.Food, b.Food = 5000, 5000
	if err := w.SetMarketListing(b, "Food", 2000, 10); err != nil {
		t.Fatalf("list: %v", err)
	}
	w.processEconomy(a)
	w.processEconomy(b)
	totalA := a.Food
	totalB := b.Food + w.MarketForSale("bob", "Food")
	if totalA != totalB {
		t.Errorf("listed food spoiled differently: granary-only total=%d, half-listed total=%d", totalA, totalB)
	}
}

// listing more than owned clamps to what's available.
func TestMarketListingClamps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	e.Jets = 5
	if err := w.SetMarketListing(e, "Jet", 999, 100); err != nil {
		t.Fatalf("list: %v", err)
	}
	if w.MarketForSale("alice", "Jet") != 5 || e.Jets != 0 {
		t.Errorf("clamp: listed=%d owned=%d, want 5 and 0", w.MarketForSale("alice", "Jet"), e.Jets)
	}
}

// a buy transfers goods and gold, credits the seller's proceeds, and the seller
// is paid exactly once — on the next turn it plays.
func TestMarketBuyAndSettle(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	seller := w.AddHuman("alice", "Alethia")
	buyer := w.AddHuman("bob", "Bobland")
	seller.Tanks = 20
	buyer.Gold = 100000
	buyer.Tanks = 0
	if err := w.SetMarketListing(seller, "Tank", 10, 1000); err != nil {
		t.Fatalf("list: %v", err)
	}
	sellerGoldBefore := seller.Gold

	if err := w.BuyFromMarket(buyer, "alice", "Tank", 4); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if buyer.Tanks != 4 || buyer.Gold != 100000-4*1000 {
		t.Errorf("buyer: tanks=%d gold=%d", buyer.Tanks, buyer.Gold)
	}
	if w.MarketForSale("alice", "Tank") != 6 {
		t.Errorf("listing after buy: %d, want 6", w.MarketForSale("alice", "Tank"))
	}
	// Seller not paid until day-end settlement.
	if seller.Gold != sellerGoldBefore {
		t.Errorf("seller paid too early: %d", seller.Gold)
	}
	w.settleMarketProceeds()
	if seller.Gold != sellerGoldBefore+4*1000 {
		t.Errorf("seller after settle: %d, want +%d", seller.Gold, 4*1000)
	}
	// Pool cleared — a second settlement doesn't double-pay.
	w.settleMarketProceeds()
	if seller.Gold != sellerGoldBefore+4*1000 {
		t.Errorf("seller paid twice: %d", seller.Gold)
	}
}

// you cannot buy from your own listing, and you cannot buy more than is for sale
// or more than you can afford.
func TestMarketBuyGuards(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alethia")
	b := w.AddHuman("bob", "Bobland")
	a.Carriers = 10
	if err := w.SetMarketListing(a, "Carrier", 10, 1000); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := w.BuyFromMarket(a, "alice", "Carrier", 1); err != ErrOwnListing {
		t.Errorf("self-buy: want ErrOwnListing, got %v", err)
	}
	// Can't afford: bob has too little gold.
	b.Gold = 500
	if err := w.BuyFromMarket(b, "alice", "Carrier", 1); err != ErrCantAfford {
		t.Errorf("afford: want ErrCantAfford, got %v", err)
	}
	// Over-buy clamps to what's for sale.
	b.Gold = 100000
	if err := w.BuyFromMarket(b, "alice", "Carrier", 999); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if b.Carriers != 10 || w.marketListing("alice", "Carrier") != nil {
		t.Errorf("over-buy: bought=%d, listing still present=%v", b.Carriers, w.marketListing("alice", "Carrier") != nil)
	}
}
