package game

import "testing"

// A sent trade deal escrows the Send goods off the sender and records a pending
// deal on the recipient. Accepting delivers the Send goods to the recipient and
// pays the Demand goods from the recipient to the sender.
func TestSendTradeDealEscrowsAndAcceptTransfersBothBaskets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Gold = 500, 0
	to.Gold, to.Tanks = 100_000, 0

	send := TradeBasket{Tanks: 100}
	demand := TradeBasket{Gold: 5_000}
	if err := w.SendTradeDeal(from, to, send, demand); err != nil {
		t.Fatalf("send: %v", err)
	}
	if from.Tanks != 400 {
		t.Errorf("send should escrow tanks off the sender: Tanks = %d, want 400", from.Tanks)
	}
	if len(to.TradeDeals) != 1 {
		t.Fatalf("recipient should have 1 pending deal, got %d", len(to.TradeDeals))
	}

	if err := w.AcceptTradeDeal(to, "Fromland"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if to.Tanks != 100 {
		t.Errorf("accept should deliver the offered tanks: to.Tanks = %d, want 100", to.Tanks)
	}
	if to.Gold != 95_000 {
		t.Errorf("accept should take the demanded gold from the recipient: to.Gold = %d, want 95000", to.Gold)
	}
	if from.Gold != 5_000 {
		t.Errorf("accept should pay the demanded gold to the sender: from.Gold = %d, want 5000", from.Gold)
	}
	if len(to.TradeDeals) != 0 {
		t.Errorf("accepted deal should be removed, %d left", len(to.TradeDeals))
	}
}

// Declining returns the escrowed Send goods to the sender and drops the deal;
// nothing else moves.
func TestDeclineTradeDealReturnsEscrow(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks = 500
	toGoldBefore := to.Gold

	w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{Gold: 5_000})
	if !w.DeclineTradeDeal(to, "Fromland") {
		t.Fatal("decline should find and drop the deal")
	}
	if from.Tanks != 500 {
		t.Errorf("decline should return escrowed tanks: from.Tanks = %d, want 500", from.Tanks)
	}
	if to.Gold != toGoldBefore {
		t.Errorf("decline should not move the recipient's gold: %d, want %d", to.Gold, toGoldBefore)
	}
	if len(to.TradeDeals) != 0 {
		t.Error("declined deal should be removed")
	}
}

// Accepting fails (and leaves the deal pending) when the recipient can't cover
// the Demand.
func TestAcceptTradeDealFailsWhenRecipientCantPay(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks = 500
	to.Gold = 100 // can't cover a 5,000 demand

	w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{Gold: 5_000})
	if err := w.AcceptTradeDeal(to, "Fromland"); err != ErrCantAfford {
		t.Fatalf("accept with too little gold should return ErrCantAfford, got %v", err)
	}
	if len(to.TradeDeals) != 1 {
		t.Error("a failed accept should leave the deal pending")
	}
	if to.Tanks != 0 {
		t.Error("a failed accept must not deliver the offered goods")
	}
}

// An all-empty deal is rejected.
func TestSendTradeDealRejectsEmpty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	if err := w.SendTradeDeal(from, to, TradeBasket{}, TradeBasket{}); err == nil {
		t.Error("an empty trade deal should be rejected")
	}
}

// Delivered gold clamps to the money cap (excess lost), like any gold overflow.
func TestAcceptTradeDealClampsGoldToMoneyCap(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Gold = 1_000_000
	to.Gold = MoneyCap - 50 // near the cap

	w.SendTradeDeal(from, to, TradeBasket{Gold: 1_000}, TradeBasket{})
	if err := w.AcceptTradeDeal(to, "Fromland"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if to.Gold != MoneyCap {
		t.Errorf("delivered gold should clamp to MoneyCap: %d, want %d", to.Gold, MoneyCap)
	}
}
