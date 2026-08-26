package game

import "testing"

// The carrier requirement is sized to the CARGO, from the original's own
// per-good capacities (BRE.OVR ovr_050dfb_entry_0436). Golden literals, not the
// constants: a retune of a verified figure should fail here and force new
// evidence rather than following along.
func TestTradeDealCarriersMatchesTheOriginalsCapacities(t *testing.T) {
	cases := []struct {
		name string
		b    TradeBasket
		want int
	}{
		{"nothing", TradeBasket{}, 0},
		// One carrier holds 1000 troopers, so 1000 fit exactly and 1001 do not.
		{"exactly one carrier of troopers", TradeBasket{Troopers: 1000}, 1},
		{"one over", TradeBasket{Troopers: 1001}, 2},
		{"jets are bulkier", TradeBasket{Jets: 100}, 1},
		{"turrets ride with the troopers", TradeBasket{Turrets: 1000}, 1},
		{"tanks pack tightest", TradeBasket{Tanks: 5000}, 1},
		{"gold takes room too", TradeBasket{Gold: 100_000}, 1},
		// Food, bombers, agents and carriers need no carrier space at all, so a
		// shipment of nothing else travels free.
		{"food rides free", TradeBasket{Food: 10_000_000}, 0},
		{"bombers ride free", TradeBasket{Bombers: 50_000}, 0},
		{"agents ride free", TradeBasket{Agents: 50_000}, 0},
		{"carriers carry each other", TradeBasket{Carriers: 500}, 0},
		// The capacities are summed and the TOTAL rounded up, not each good
		// separately: half a carrier of troopers and half of jets is one carrier,
		// where rounding per good would charge two.
		{"halves share a carrier", TradeBasket{Troopers: 500, Jets: 50}, 1},
		{"a large mixed shipment", TradeBasket{Troopers: 20_000, Jets: 1_000, Tanks: 10_000}, 32},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TradeDealCarriers(c.b); got != c.want {
				t.Errorf("TradeDealCarriers(%+v) = %d, want %d", c.b, got, c.want)
			}
		})
	}
}

// The fee is the weighted sum over five plus a flat base (BRE.OVR 0x0513e7),
// then the sysop's Trade Deal Costs ladder. Golden literals again.
func TestIPTradeDealCostMatchesTheOriginalsFormula(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	// An empty basket is free: the original tests the weighted sum for zero
	// BEFORE adding its base, so nothing shipped costs nothing rather than the
	// base on its own.
	if got := w.IPTradeDealCost(TradeBasket{}); got != 0 {
		t.Errorf("an empty basket cost %d, want 0", got)
	}
	// 5,000 troopers weigh 1 each: 5000/5 = 1,000, plus the 100,000 base.
	if got := w.IPTradeDealCost(TradeBasket{Troopers: 5_000}); got != 101_000 {
		t.Errorf("5,000 troopers cost %d, want 101,000", got)
	}
	// A bomber weighs 3, so 5,000 of them weigh 15,000: 15000/5 = 3,000.
	if got := w.IPTradeDealCost(TradeBasket{Bombers: 5_000}); got != 103_000 {
		t.Errorf("5,000 bombers cost %d, want 103,000", got)
	}
	// Food weighs 0.05 and gold 0.01 — the two fractional weights, and the ones
	// IB holds as exact integers where the original holds Real48 approximations.
	if got := w.IPTradeDealCost(TradeBasket{Food: 100_000}); got != 101_000 {
		t.Errorf("100,000 food cost %d, want 101,000", got)
	}
	if got := w.IPTradeDealCost(TradeBasket{Gold: 1_000_000}); got != 102_000 {
		t.Errorf("1,000,000 gold cost %d, want 102,000", got)
	}
	// An agent weighs 0.5.
	if got := w.IPTradeDealCost(TradeBasket{Agents: 10_000}); got != 101_000 {
		t.Errorf("10,000 agents cost %d, want 101,000", got)
	}
}

// The Trade Deal Costs ladder scales the WHOLE fee, base included — its own
// spread, a sixth at Low and triple at High (BRE.OVR 0x5158F).
func TestIPTradeDealCostFollowsTheSysopsLadder(t *testing.T) {
	basket := TradeBasket{Troopers: 5_000} // 101,000 at the default
	for _, c := range []struct {
		level Level
		want  int64
	}{
		{Medium, 101_000},
		{Low, 101_000 / 6},
		{High, 101_000 * 3},
		{None, 0},
	} {
		cfg := DefaultConfig()
		cfg.TradeCosts = c.level
		w := NewWorldSeed(cfg, 1)
		if got := w.IPTradeDealCost(basket); got != c.want {
			t.Errorf("at %v the fee was %d, want %d", c.level, got, c.want)
		}
	}
}

// The whole round trip: what leaves the sender, what rides the packet, and what
// the far realm ends up holding.
func TestIPTradeDealShipsGoodsAndChargesTheSender(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	w := NewWorldSeed(cfg, 1)
	from := w.AddHuman("sender", "Sender")
	from.Protection = 0
	from.Troopers, from.Carriers, from.Gold = 10_000, 50, 5_000_000

	goods := TradeBasket{Troopers: 5_000}
	cost := w.IPTradeDealCost(goods)
	need := TradeDealCarriers(goods)
	if need == 0 {
		t.Fatal("this shipment should need carriers, or the test proves nothing about them")
	}
	if err := w.SendIPTradeDeal(from, "there", "Receiver", goods); err != nil {
		t.Fatalf("send: %v", err)
	}
	if from.Troopers != 5_000 {
		t.Errorf("sender kept %d troopers, want 5,000", from.Troopers)
	}
	if from.Carriers != 50-need {
		t.Errorf("sender kept %d carriers, want %d", from.Carriers, 50-need)
	}
	if from.Gold != 5_000_000-cost {
		t.Errorf("sender kept %d gold, want %d", from.Gold, 5_000_000-cost)
	}

	// The shipment rides the outbox to the named board.
	var sent []IPTradeDeal
	for _, p := range w.Outbox {
		if p.ToBoard == "there" {
			sent = append(sent, p.TradeDeals...)
		}
	}
	if len(sent) != 1 {
		t.Fatalf("queued %d deals for that board, want 1", len(sent))
	}
	if sent[0].ToEmpire != "Receiver" || sent[0].Goods.Troopers != 5_000 {
		t.Errorf("queued the wrong shipment: %+v", sent[0])
	}

	// And it lands on the far board with no answer of any kind.
	far := NewWorldSeed(cfg, 1)
	far.Config.BoardID = "there"
	to := far.AddHuman("recv", "Receiver")
	to.Protection, to.Troopers = 0, 100
	before := len(far.NewsToday)
	far.deliverIPTradeDeal(sent[0])
	if to.Troopers != 5_100 {
		t.Errorf("receiver holds %d troopers, want 5,100", to.Troopers)
	}
	if len(far.NewsToday) != before {
		t.Errorf("a trade deal posted planet news; the original posts none: %v", far.NewsToday[before:])
	}
	if len(to.Events) == 0 {
		t.Error("the receiver was told nothing about the shipment")
	}
}

// Every refusal, so none of them is quietly lost.
func TestIPTradeDealRefusals(t *testing.T) {
	newSender := func() (*World, *Empire) {
		cfg := DefaultConfig()
		cfg.IBBS = true
		cfg.BoardID = "here"
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("sender", "Sender")
		e.Protection = 0
		e.Troopers, e.Carriers, e.Gold = 10_000, 50, 5_000_000
		return w, e
	}
	full := TradeBasket{Troopers: 5_000}

	t.Run("own planet", func(t *testing.T) {
		w, e := newSender()
		if err := w.SendIPTradeDeal(e, "here", "Someone", full); err != ErrTradeDealOwnPlanet {
			t.Errorf("got %v, want the own-planet refusal", err)
		}
	})
	t.Run("sender under protection", func(t *testing.T) {
		w, e := newSender()
		e.Protection = 3
		if err := w.SendIPTradeDeal(e, "there", "Someone", full); err != ErrInProtection {
			t.Errorf("got %v, want the protection refusal", err)
		}
	})
	t.Run("nothing in the basket", func(t *testing.T) {
		w, e := newSender()
		if err := w.SendIPTradeDeal(e, "there", "Someone", TradeBasket{}); err != ErrEmptyTradeDeal {
			t.Errorf("got %v, want the empty-deal refusal", err)
		}
	})
	t.Run("goods not held", func(t *testing.T) {
		w, e := newSender()
		if err := w.SendIPTradeDeal(e, "there", "Someone", TradeBasket{Troopers: 50_000}); err != ErrCantAfford {
			t.Errorf("got %v, want the cannot-afford refusal", err)
		}
	})
	t.Run("not enough carriers", func(t *testing.T) {
		w, e := newSender()
		e.Carriers = 1
		if err := w.SendIPTradeDeal(e, "there", "Someone", full); err == nil {
			t.Error("a shipment with no transport was accepted")
		}
	})
	t.Run("a deal cannot be funded by the gold it ships", func(t *testing.T) {
		w, e := newSender()
		// Everything held is in the basket, so nothing is left to pay the fee.
		e.Gold = 1_000_000
		b := TradeBasket{Gold: 1_000_000}
		if err := w.SendIPTradeDeal(e, "there", "Someone", b); err != ErrCantAfford {
			t.Errorf("got %v, want the cannot-afford refusal", err)
		}
		if e.Gold != 1_000_000 {
			t.Errorf("a refused deal still moved gold: %d", e.Gold)
		}
	})
	t.Run("nothing is spent on a refusal", func(t *testing.T) {
		w, e := newSender()
		e.Carriers = 0
		_ = w.SendIPTradeDeal(e, "there", "Someone", full)
		if e.Troopers != 10_000 || e.Gold != 5_000_000 {
			t.Errorf("a refused deal moved goods or gold: %d troopers, %d gold", e.Troopers, e.Gold)
		}
	})
}
