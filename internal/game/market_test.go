package game

import "testing"

// listing goods escrows them out of inventory, and inventory + listed is
// conserved through a re-list and a full delist.
func TestMarketEscrowConserves(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	e.Troopers = 100

	if err := w.SetMarketListing(e, "Trooper", 40, 500); err != nil {
		t.Fatalf("list: %v", err)
	}
	if e.Troopers != 60 {
		t.Errorf("owned after listing 40: got %d, want 60", e.Troopers)
	}
	if got := w.MarketForSale("Alethia", "Trooper"); got != 40 {
		t.Errorf("for sale: got %d, want 40", got)
	}
	if got := w.MarketTotalForSale("Trooper", e); got != 40 {
		t.Errorf("total for sale: got %d, want 40", got)
	}

	// Re-list a smaller amount returns the difference to inventory.
	if err := w.SetMarketListing(e, "Trooper", 10, 500); err != nil {
		t.Fatalf("relist: %v", err)
	}
	if e.Troopers != 90 || w.MarketForSale("Alethia", "Trooper") != 10 {
		t.Errorf("after relist to 10: owned=%d listed=%d, want 90 and 10", e.Troopers, w.MarketForSale("Alethia", "Trooper"))
	}

	// Delist (0) returns everything and removes the listing.
	if err := w.SetMarketListing(e, "Trooper", 0, 0); err != nil {
		t.Fatalf("delist: %v", err)
	}
	if e.Troopers != 100 {
		t.Errorf("owned after delist: got %d, want 100", e.Troopers)
	}
	if w.marketListing("Alethia", "Trooper") != nil {
		t.Error("empty listing should be removed")
	}
}

// listing units does not reduce force maintenance — you still pay upkeep on
// escrowed units, so the market can't be used to dodge maintenance.
func TestMarketListedGoodsStillCostMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
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
// holding all its food in hand and one with half listed, end a turn with
// the same total food.
func TestMarketListedFoodSpoilsLikeFoodInHand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alethia")
	b := w.AddHuman("bob", "Bobland")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	a.Food, b.Food = 5000, 5000
	if err := w.SetMarketListing(b, "Food", 2000, 10); err != nil {
		t.Fatalf("list: %v", err)
	}
	w.processEconomy(a)
	w.processEconomy(b)
	totalA := a.Food
	totalB := b.Food + w.MarketForSale("Bobland", "Food")
	if totalA != totalB {
		t.Errorf("listed food spoiled differently: all-in-hand total=%d, half-listed total=%d", totalA, totalB)
	}
}

// listing more than owned clamps to what's available.
func TestMarketListingClamps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	e.Jets = 5
	if err := w.SetMarketListing(e, "Jet", 999, 100); err != nil {
		t.Fatalf("list: %v", err)
	}
	if w.MarketForSale("Alethia", "Jet") != 5 || e.Jets != 0 {
		t.Errorf("clamp: listed=%d owned=%d, want 5 and 0", w.MarketForSale("Alethia", "Jet"), e.Jets)
	}
}

// a buy transfers goods and gold, credits the seller's proceeds, and the seller
// is paid exactly once — on the next turn it plays.
func TestMarketBuyAndSettle(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	seller := w.AddHuman("alice", "Alethia")
	buyer := w.AddHuman("bob", "Bobland")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	seller.Tanks = 20
	buyer.Gold = 100000
	buyer.Tanks = 0
	if err := w.SetMarketListing(seller, "Tank", 10, 1000); err != nil {
		t.Fatalf("list: %v", err)
	}
	sellerGoldBefore := seller.Gold

	if err := w.BuyFromMarket(buyer, "Alethia", "Tank", 4); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if buyer.Tanks != 4 || buyer.Gold != 100000-4*1000 {
		t.Errorf("buyer: tanks=%d gold=%d", buyer.Tanks, buyer.Gold)
	}
	if w.MarketForSale("Alethia", "Tank") != 6 {
		t.Errorf("listing after buy: %d, want 6", w.MarketForSale("Alethia", "Tank"))
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
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	a.Carriers = 10
	if err := w.SetMarketListing(a, "Carrier", 10, 1000); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := w.BuyFromMarket(a, "Alethia", "Carrier", 1); err != ErrOwnListing {
		t.Errorf("self-buy: want ErrOwnListing, got %v", err)
	}
	// Can't afford: bob has too little gold.
	b.Gold = 500
	if err := w.BuyFromMarket(b, "Alethia", "Carrier", 1); err != ErrCantAfford {
		t.Errorf("afford: want ErrCantAfford, got %v", err)
	}
	// Over-buy clamps to what's for sale.
	b.Gold = 100000
	if err := w.BuyFromMarket(b, "Alethia", "Carrier", 999); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if b.Carriers != 10 || w.marketListing("Alethia", "Carrier") != nil {
		t.Errorf("over-buy: bought=%d, listing still present=%v", b.Carriers, w.marketListing("Alethia", "Carrier") != nil)
	}
}

// Buying from a listing needs one of MarketAccessTreaties — IB's own rule, so
// that a seller can aim a cheap listing at chosen realms instead of whoever
// reaches it first. The three trade pacts are deliberately NOT among them: they
// already pay a per-turn income, and only one pact can stand between a pair.
func TestMarketBuyingNeedsTheRightPact(t *testing.T) {
	seller, buyer := "Alethia", "Bobland"
	for _, tc := range []struct {
		pact string
		want bool
	}{
		{"", false},
		{RelationEnemy, false},
		{tariffTradeAgreement, false},
		{freeTradeAgreement, false},
		{protectiveTrade, false},
		{terroristPrevention, true},
		{intelligenceAlliance, true},
		{technologyAgreement, true},
		{fullDefenseAlliance, true},
	} {
		w := NewWorldSeed(DefaultConfig(), 1)
		a := w.AddHuman("alice", seller)
		b := w.AddHuman("bob", buyer)
		pastProtection(w)
		a.Tanks, b.Gold = 100, 10_000_000
		if err := w.SetMarketListing(a, "Tank", 100, 10); err != nil {
			t.Fatalf("%s: list: %v", tc.pact, err)
		}
		w.setRelation(seller, buyer, tc.pact)

		err := w.BuyFromMarket(b, seller, "Tank", 10)
		if got := err == nil; got != tc.want {
			t.Errorf("pact %q: bought=%v, want %v (err %v)", tc.pact, got, tc.want, err)
		}
		if tc.want && b.Tanks != 10 {
			t.Errorf("pact %q: bought %d tanks, want 10", tc.pact, b.Tanks)
		}
		if !tc.want && b.Tanks != 0 {
			t.Errorf("pact %q: bought %d tanks with no market pact", tc.pact, b.Tanks)
		}
	}
}

// Listing stays open to everyone: the gate is on buying, so a realm with no
// pacts at all can still offer goods and wait for a partner.
func TestListingNeedsNoPact(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alethia")
	pastProtection(w)
	a.Tanks = 100
	if err := w.SetMarketListing(a, "Tank", 100, 10); err != nil {
		t.Fatalf("a realm with no pacts should still be able to list: %v", err)
	}
	if n := w.MarketForSale("Alethia", "Tank"); n != 100 {
		t.Errorf("listed %d, want 100", n)
	}
}

// The pact gate is a single-board rule. On a league board the planet is one
// team, so the market stays open — and the interplanetary market, which has its
// own alliance check, never comes through BuyFromMarket at all.
func TestMarketPactGateIsLocalOnly(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "Alpha"
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("alice", "Alethia")
	b := w.AddHuman("bob", "Bobland")
	pastProtection(w)
	a.Tanks, b.Gold = 100, 10_000_000
	if err := w.SetMarketListing(a, "Tank", 100, 10); err != nil {
		t.Fatalf("list: %v", err)
	}
	// No treaty of any kind between them.
	if err := w.BuyFromMarket(b, "Alethia", "Tank", 10); err != nil {
		t.Fatalf("a league board should not gate the local market: %v", err)
	}
	if b.Tanks != 10 {
		t.Errorf("bought %d tanks, want 10", b.Tanks)
	}
}

// A realm sees only what it could buy. Showing a listing it has no pact for is
// noise, and the quantity would leak what a rival is holding back.
func TestMarketHidesListingsYouCannotBuy(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	seller := w.AddHuman("alice", "Alethia")
	viewer := w.AddHuman("bob", "Bobland")
	pastProtection(w)
	seller.Tanks, viewer.Tanks = 100, 7
	if err := w.SetMarketListing(seller, "Tank", 100, 10); err != nil {
		t.Fatalf("seller lists: %v", err)
	}
	if err := w.SetMarketListing(viewer, "Tank", 7, 99); err != nil {
		t.Fatalf("viewer lists: %v", err)
	}

	// No pact: the seller's row is not offered, and its 100 are not counted.
	if got := w.MarketSellers("Tank", viewer); len(got) != 0 {
		t.Errorf("with no pact the browse list showed %d row(s)", len(got))
	}
	if got := w.MarketTotalForSale("Tank", viewer); got != 7 {
		t.Errorf("total = %d, want 7 — only the viewer's own listing is visible", got)
	}

	// A market pact reveals it.
	w.setRelation("Alethia", "Bobland", technologyAgreement)
	if got := w.MarketSellers("Tank", viewer); len(got) != 1 || got[0].Realm != "Alethia" {
		t.Errorf("with a pact the browse list showed %d row(s), want Alethia", len(got))
	}
	if got := w.MarketTotalForSale("Tank", viewer); got != 107 {
		t.Errorf("total = %d, want 107 (100 revealed + the viewer's own 7)", got)
	}

	// A trade pact does not: it pays income instead of opening the market.
	w.setRelation("Alethia", "Bobland", freeTradeAgreement)
	if got := w.MarketSellers("Tank", viewer); len(got) != 0 {
		t.Errorf("a Free Trade Agreement revealed %d listing(s)", len(got))
	}
}
