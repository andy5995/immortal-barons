package game

import "testing"

// Sending escrows the offered goods, consumes a transport carrier, and charges
// the per-day fee. Accepting delivers the offered goods to the recipient and
// pays the demanded goods from the recipient to the sender.
func TestSendTradeDealChargesAndAcceptTransfersBaskets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Carriers, from.Gold = 500, 1, 300_000
	to.Gold, to.Tanks = 100_000, 0

	send := TradeBasket{Tanks: 100}
	demand := TradeBasket{Gold: 5_000}
	fee := TradeDealCost(TradeDealMinDays) // 2 * 100,000
	if err := w.SendTradeDeal(from, to, send, demand, TradeDealMinDays); err != nil {
		t.Fatalf("send: %v", err)
	}
	if from.Tanks != 400 {
		t.Errorf("send should escrow the offered tanks: %d, want 400", from.Tanks)
	}
	if from.Carriers != 0 {
		t.Errorf("send should consume the transport carrier: %d, want 0", from.Carriers)
	}
	if from.Gold != 300_000-fee {
		t.Errorf("send should charge the fee: Gold = %d, want %d", from.Gold, 300_000-fee)
	}
	if len(to.TradeDeals) != 1 {
		t.Fatalf("recipient should have 1 pending deal, got %d", len(to.TradeDeals))
	}

	if err := w.AcceptTradeDeal(to, "Fromland"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if to.Tanks != 100 || to.Gold != 95_000 {
		t.Errorf("accept should deliver 100 tanks and take 5000 gold: Tanks=%d Gold=%d", to.Tanks, to.Gold)
	}
	if from.Gold != 300_000-fee+5_000 {
		t.Errorf("accept should pay the demanded gold to the sender: %d", from.Gold)
	}
}

// Sending without a carrier fails and moves nothing.
func TestSendTradeDealNeedsCarrier(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Carriers, from.Gold = 500, 0, 1_000_000

	if err := w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{}, 2); err != ErrTradeNeedsCarrier {
		t.Fatalf("send without a carrier should return ErrTradeNeedsCarrier, got %v", err)
	}
	if from.Tanks != 500 || len(to.TradeDeals) != 0 {
		t.Error("a failed send must move nothing")
	}
}

// Sending without enough gold for the fee fails.
func TestSendTradeDealNeedsFee(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Carriers, from.Gold = 500, 1, 100 // can't cover a 200,000 fee

	if err := w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{}, 2); err != ErrCantAfford {
		t.Fatalf("send without the fee should return ErrCantAfford, got %v", err)
	}
	if from.Carriers != 1 || from.Tanks != 500 {
		t.Error("a failed send must move nothing")
	}
}

// Declining returns the escrowed offered goods to the sender (but not the spent
// carrier or fee) and drops the deal.
func TestDeclineTradeDealReturnsEscrow(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Carriers, from.Gold = 500, 1, 300_000
	toGoldBefore := to.Gold

	w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{Gold: 5_000}, TradeDealMinDays)
	if !w.DeclineTradeDeal(to, "Fromland") {
		t.Fatal("decline should find and drop the deal")
	}
	if from.Tanks != 500 {
		t.Errorf("decline should return escrowed tanks: %d, want 500", from.Tanks)
	}
	if from.Carriers != 0 {
		t.Errorf("the spent carrier is not returned on decline: %d, want 0", from.Carriers)
	}
	if to.Gold != toGoldBefore {
		t.Errorf("decline should not move the recipient's gold: %d, want %d", to.Gold, toGoldBefore)
	}
	if len(to.TradeDeals) != 0 {
		t.Error("declined deal should be removed")
	}
}

// Accepting fails (deal stays pending) when the recipient can't cover the demand.
func TestAcceptTradeDealFailsWhenRecipientCantPay(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Tanks, from.Carriers, from.Gold = 500, 1, 300_000
	to.Gold = 100 // can't cover a 5,000 demand

	w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{Gold: 5_000}, TradeDealMinDays)
	if err := w.AcceptTradeDeal(to, "Fromland"); err != ErrCantAfford {
		t.Fatalf("accept with too little gold should return ErrCantAfford, got %v", err)
	}
	if len(to.TradeDeals) != 1 || to.Tanks != 0 {
		t.Error("a failed accept should leave the deal pending and deliver nothing")
	}
}

// An all-empty deal is rejected.
func TestSendTradeDealRejectsEmpty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Carriers, from.Gold = 1, 1_000_000
	if err := w.SendTradeDeal(from, to, TradeBasket{}, TradeBasket{}, 2); err == nil {
		t.Error("an empty trade deal should be rejected")
	}
}

// Delivered gold clamps to the money cap (excess lost).
func TestAcceptTradeDealClampsGoldToMoneyCap(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	from.Carriers, from.Gold = 1, MoneyCap
	to.Gold = MoneyCap - 50

	w.SendTradeDeal(from, to, TradeBasket{Gold: 1_000}, TradeBasket{}, TradeDealMinDays)
	if err := w.AcceptTradeDeal(to, "Fromland"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if to.Gold != MoneyCap {
		t.Errorf("delivered gold should clamp to MoneyCap: %d, want %d", to.Gold, MoneyCap)
	}
}
