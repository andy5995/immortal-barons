package game

import "testing"

// Every computer baron carries the empty owner handle — that emptiness is what
// marks it as AI — so a market keyed by the handle gave the whole pool ONE
// position per good. The second baron to list overwrote the first, and the goods
// the overwritten row held had already left that baron's inventory, so they
// ceased to exist: 100 tanks off one realm's books and off the market at once.
func TestAIBaronsKeepSeparateMarketPositions(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.AddAIEmpires(2)
	ai := w.AIEmpires()
	if len(ai) < 2 {
		t.Fatalf("need two computer barons, got %d — nothing below is tested", len(ai))
	}
	a, b := ai[0], ai[1]
	a.Tanks, b.Tanks = 500, 700

	if err := w.SetMarketListing(a, "Tank", 100, 50); err != nil {
		t.Fatalf("A lists: %v", err)
	}
	if err := w.SetMarketListing(b, "Tank", 250, 60); err != nil {
		t.Fatalf("B lists: %v", err)
	}

	if a.Tanks != 400 || b.Tanks != 450 {
		t.Errorf("escrow left A with %d and B with %d tanks in stock, want 400 and 450", a.Tanks, b.Tanks)
	}
	if got := w.MarketForSale(a.Name, "Tank"); got != 100 {
		t.Errorf("A has %d tanks listed, want 100 — B's listing took its place", got)
	}
	if got := w.MarketForSale(b.Name, "Tank"); got != 250 {
		t.Errorf("B has %d tanks listed, want 250", got)
	}
	if got := w.MarketPrice(a.Name, "Tank"); got != 50 {
		t.Errorf("A's asking price is %d, want 50 — B set its own", got)
	}
	// The planet-wide pool is the conservation check: 750 tanks existed, 350 of
	// them are escrowed on the market and the other 400 are still in stock.
	if got := w.MarketTotalForSale("Tank"); got != 350 {
		t.Errorf("%d tanks on the market, want 350 (100 + 250) — the rest were destroyed", got)
	}
}

// The proceeds pool was keyed by the same handle, so every baron's sale gold
// piled into one entry and settleMarketProceeds paid the lot to whichever realm
// the lookup happened to return first.
func TestAIBaronsAreSettledTheirOwnMarketProceeds(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.AddAIEmpires(2)
	ai := w.AIEmpires()
	if len(ai) < 2 {
		t.Fatalf("need two computer barons, got %d — nothing below is tested", len(ai))
	}
	a, b := ai[0], ai[1]
	a.Tanks, b.Carriers = 500, 700
	buyer := w.AddHuman("buyer", "Buyer")
	buyer.Gold = 10_000_000

	if err := w.SetMarketListing(a, "Tank", 100, 50); err != nil {
		t.Fatalf("A lists: %v", err)
	}
	if err := w.SetMarketListing(b, "Carrier", 200, 70); err != nil {
		t.Fatalf("B lists: %v", err)
	}
	if err := w.BuyFromMarket(buyer, a.Name, "Tank", 10); err != nil {
		t.Fatalf("buy from A: %v", err)
	}
	if err := w.BuyFromMarket(buyer, b.Name, "Carrier", 10); err != nil {
		t.Fatalf("buy from B: %v", err)
	}
	if w.MarketProceeds[a.Name] != 500 || w.MarketProceeds[b.Name] != 700 {
		t.Fatalf("proceeds accrued as %v, want 500 to A and 700 to B", w.MarketProceeds)
	}

	goldA, goldB := a.Gold, b.Gold
	w.settleMarketProceeds()

	// Each is paid its own sale, less the market's commission.
	wantA := int64(500 - 500*MarketCommissionPct/100)
	wantB := int64(700 - 700*MarketCommissionPct/100)
	if got := a.Gold - goldA; got != wantA {
		t.Errorf("A was paid %d for its tanks, want %d", got, wantA)
	}
	if got := b.Gold - goldB; got != wantB {
		t.Errorf("B was paid %d for its carriers, want %d", got, wantB)
	}
}

// forgetMarketPosition wipes a removed realm's escrow and unpaid gold. It had to
// skip the empty owner handle to avoid taking the whole living AI pool with it,
// which left a dead baron's listing standing; with a per-baron key it wipes on
// every removal and touches nobody else.
func TestRemovingOneAIBaronLeavesTheOthersMarketPosition(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.AddAIEmpires(2)
	ai := w.AIEmpires()
	if len(ai) < 2 {
		t.Fatalf("need two computer barons, got %d — nothing below is tested", len(ai))
	}
	a, b := ai[0], ai[1]
	a.Tanks, b.Carriers = 500, 700
	buyer := w.AddHuman("buyer", "Buyer")
	buyer.Gold = 10_000_000
	if err := w.SetMarketListing(a, "Tank", 100, 50); err != nil {
		t.Fatalf("A lists: %v", err)
	}
	if err := w.SetMarketListing(b, "Carrier", 200, 70); err != nil {
		t.Fatalf("B lists: %v", err)
	}
	if err := w.BuyFromMarket(buyer, b.Name, "Carrier", 10); err != nil {
		t.Fatalf("buy from B: %v", err)
	}

	w.RemoveEmpire(a)

	if got := w.MarketTotalForSale("Tank"); got != 0 {
		t.Errorf("%d tanks of the removed baron are still on the market", got)
	}
	if got := w.MarketForSale(b.Name, "Carrier"); got != 190 {
		t.Errorf("B has %d carriers listed after A was removed, want 190", got)
	}
	if got := w.MarketProceeds[b.Name]; got != 700 {
		t.Errorf("B's unpaid sale gold is %d after A was removed, want 700", got)
	}
}
