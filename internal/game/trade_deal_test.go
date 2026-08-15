package game

import "testing"

// Sending escrows the offered goods, consumes a transport carrier, and charges
// the per-day fee. Accepting delivers the offered goods to the recipient and
// pays the demanded goods from the recipient to the sender.
func TestSendTradeDealChargesAndAcceptTransfersBaskets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	pastProtection(w)
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
	pastProtection(w)
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
	pastProtection(w)
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
	pastProtection(w)
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
	pastProtection(w)
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
	pastProtection(w)
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
	pastProtection(w)
	from.Carriers, from.Gold = 1, w.MoneyCap()
	to.Gold = w.MoneyCap() - 50

	w.SendTradeDeal(from, to, TradeBasket{Gold: 1_000}, TradeBasket{}, TradeDealMinDays)
	if err := w.AcceptTradeDeal(to, "Fromland"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if to.Gold != w.MoneyCap() {
		t.Errorf("delivered gold should clamp to the money cap: %d, want %d", to.Gold, w.MoneyCap())
	}
}

// A Protective Trade agreement puts guards on the route and makes the deal
// cheaper to send: BRE divides the PER-DAY rate by three, so a two-day deal
// costs 2 x 33,333 rather than 2 x 100,000. Golden literals — these are the
// binary's figures, not a playtest knob (see ProtectiveTradeCostDivisor).
func TestProtectiveTradeMakesDealsCheaper(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	pastProtection(w)
	from.Carriers, from.Gold = 2, 1_000_000

	if got := w.TradeDealCostBetween(from, to, 2); got != 200_000 {
		t.Fatalf("undiscounted 2-day cost = %d, want 200000", got)
	}
	w.setRelation(from.Name, to.Name, protectiveTrade)
	if got := w.TradeDealGoldPerDayBetween(from, to); got != 33_333 {
		t.Errorf("guarded per-day rate = %d, want 33333", got)
	}
	if got := w.TradeDealCostBetween(from, to, 2); got != 66_666 {
		t.Errorf("guarded 2-day cost = %d, want 66666", got)
	}

	before := from.Gold
	if err := w.SendTradeDeal(from, to, TradeBasket{}, TradeBasket{Food: 1}, 2); err != nil {
		t.Fatalf("send: %v", err)
	}
	if before-from.Gold != 66_666 {
		t.Errorf("send charged %d, want the guarded 66666", before-from.Gold)
	}
}

// The sysop's Trade Deal Costs setting has to reach the price. It is one of the
// original's five cost knobs and IB stored and broadcast it while it changed
// nothing at all (#56).
func TestTradeDealCostFollowsTheCostSetting(t *testing.T) {
	rate := func(l Level) int64 {
		cfg := DefaultConfig()
		cfg.TradeCosts = l
		w := NewWorldSeed(cfg, 1)
		a, b := w.AddHuman("a", "Alpha"), w.AddHuman("b", "Bravo")
		return w.TradeDealGoldPerDayBetween(a, b)
	}
	med := rate(Medium)
	if med != TradeDealGoldPerDay {
		t.Fatalf("Medium should be the unscaled rate: got %d, want %d", med, TradeDealGoldPerDay)
	}
	// Golden literals, not the constants: this ladder is the original's own and
	// differs from BOTH the others — Low divides by six where the attack knobs
	// divide by five, and the generic presets halve.
	if got := rate(Low); got != 16_666 {
		t.Errorf("Low = %d, want 100,000/6 = 16,666", got)
	}
	if got := rate(High); got != 300_000 {
		t.Errorf("High = %d, want 100,000x3 = 300,000", got)
	}
	if got := rate(None); got != 0 {
		t.Errorf("None = %d, want a free deal", got)
	}
}

// A Protective Trade pact discounts what the setting leaves, not the raw rate —
// so at None there is nothing to discount and the deal is still free.
func TestProtectiveTradeDiscountsTheScaledRate(t *testing.T) {
	for _, c := range []struct {
		level Level
		want  int64
	}{
		{Medium, TradeDealGoldPerDay / ProtectiveTradeCostDivisor},
		{High, TradeDealGoldPerDay * TradeCostHighMultiple / ProtectiveTradeCostDivisor},
		{None, 0},
	} {
		cfg := DefaultConfig()
		cfg.TradeCosts = c.level
		w := NewWorldSeed(cfg, 1)
		a, b := w.AddHuman("a", "Alpha"), w.AddHuman("b", "Bravo")
		w.setRelation(a.Name, b.Name, protectiveTrade)
		if got := w.TradeDealGoldPerDayBetween(a, b); got != c.want {
			t.Errorf("%v with Protective Trade: got %d, want %d", c.level, got, c.want)
		}
	}
}
