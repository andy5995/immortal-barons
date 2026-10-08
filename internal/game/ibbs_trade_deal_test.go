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
func TestTradeOfferCostMatchesTheOriginalsFormula(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	// An empty basket is free: the original tests the weighted sum for zero
	// BEFORE adding its base, so nothing shipped costs nothing rather than the
	// base on its own.
	if got := w.TradeOfferCost(TradeBasket{}); got != 0 {
		t.Errorf("an empty basket cost %d, want 0", got)
	}
	// 5,000 troopers weigh 1 each: 5000/5 = 1,000, plus the 100,000 base.
	if got := w.TradeOfferCost(TradeBasket{Troopers: 5_000}); got != 101_000 {
		t.Errorf("5,000 troopers cost %d, want 101,000", got)
	}
	// A bomber weighs 3, so 5,000 of them weigh 15,000: 15000/5 = 3,000.
	if got := w.TradeOfferCost(TradeBasket{Bombers: 5_000}); got != 103_000 {
		t.Errorf("5,000 bombers cost %d, want 103,000", got)
	}
	// Food weighs 0.05 and gold 0.01 — the two fractional weights, and the ones
	// IB holds as exact integers where the original holds Real48 approximations.
	if got := w.TradeOfferCost(TradeBasket{Food: 100_000}); got != 101_000 {
		t.Errorf("100,000 food cost %d, want 101,000", got)
	}
	if got := w.TradeOfferCost(TradeBasket{Gold: 1_000_000}); got != 102_000 {
		t.Errorf("1,000,000 gold cost %d, want 102,000", got)
	}
	// An agent weighs 0.5.
	if got := w.TradeOfferCost(TradeBasket{Agents: 10_000}); got != 101_000 {
		t.Errorf("10,000 agents cost %d, want 101,000", got)
	}
}

// The Trade Deal Costs ladder scales the WHOLE fee, base included — its own
// spread, a sixth at Low and triple at High (BRE.OVR 0x5158F).
func TestTradeOfferCostFollowsTheSysopsLadder(t *testing.T) {
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
		if got := w.TradeOfferCost(basket); got != c.want {
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
	cost := w.TradeOfferCost(goods)
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

	// And it lands on the far board with no planet news.
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
	t.Run("no realm named", func(t *testing.T) {
		w, e := newSender()
		if err := w.SendIPTradeDeal(e, "there", "", full); err != ErrTradeDealNoRealm {
			t.Errorf("got %v, want the no-realm refusal", err)
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

// The sender hears back either way: delivered, or lost to a realm that is gone.
// Driven through ApplyPacket on both boards, so the receipt really rides the
// reply packet rather than being handed across by the test.
func TestIPTradeDealReceiptReachesTheSender(t *testing.T) {
	for _, alive := range []bool{true, false} {
		cfg := DefaultConfig()
		cfg.IBBS = true
		cfg.BoardID = "here"
		home := NewWorldSeed(cfg, 1)
		from := home.AddHuman("sender", "Sender")
		from.Protection = 0
		from.Troopers, from.Carriers, from.Gold = 10_000, 50, 5_000_000
		if err := home.SendIPTradeDeal(from, "there", "Receiver", TradeBasket{Troopers: 5_000}); err != nil {
			t.Fatalf("send: %v", err)
		}
		var out Packet
		for _, p := range home.Outbox {
			if p.ToBoard == "there" {
				out = p
			}
		}

		far := NewWorldSeed(cfg, 1)
		far.Config.BoardID = "there"
		to := far.AddHuman("recv", "Receiver")
		to.Protection = 0
		to.Alive = alive
		reply := far.receive(out)
		if len(reply.TradeReceipts) != 1 || reply.TradeReceipts[0].Delivered != alive {
			t.Fatalf("alive=%v: reply carried %+v, want one receipt with Delivered=%v",
				alive, reply.TradeReceipts, alive)
		}

		before := len(from.Events)
		home.receive(reply)
		// 5,000 troopers need five carriers, and they come home either way; the
		// goods come home only when nobody was there to take them.
		wantTroopers := 5_000
		if !alive {
			wantTroopers = 10_000
		}
		if from.Carriers != 50 || from.Troopers != wantTroopers {
			t.Errorf("alive=%v: sender holds %d carriers, %d troopers; want 50, %d",
				alive, from.Carriers, from.Troopers, wantTroopers)
		}
		if len(from.Events) != before+1 {
			t.Fatalf("alive=%v: sender got %d new events, want 1", alive, len(from.Events)-before)
		}
		got := from.Events[before].Text
		want := "Your trade deal reached Receiver of there: 5000 Troopers."
		if !alive {
			want = "Your trade deal to Receiver of there found no such realm there, and your trade fleet has brought the goods home."
		}
		if got != want {
			t.Errorf("alive=%v: sender was told %q, want %q", alive, got, want)
		}
	}
}

// A deal whose packet goes missing comes home on the lost-forces timer, goods
// and carriers both, and a receipt that turns up after that pays nothing more.
func TestIPTradeDealLostPacketComesHome(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	cfg.LostForcesDays = 3
	home := NewWorldSeed(cfg, 1)
	from := home.AddHuman("sender", "Sender")
	from.Protection = 0
	from.Troopers, from.Carriers, from.Gold = 10_000, 50, 5_000_000
	if err := home.SendIPTradeDeal(from, "there", "Receiver", TradeBasket{Troopers: 5_000, Gold: 1_000}); err != nil {
		t.Fatalf("send: %v", err)
	}
	gold := from.Gold
	var out Packet
	for _, p := range home.Outbox {
		if p.ToBoard == "there" {
			out = p
		}
	}

	home.GameDay += 2
	if home.ReturnLostForces(nil) != 0 || from.Troopers != 5_000 {
		t.Fatal("the deal came home before its wait ran out")
	}
	home.GameDay++
	if n := home.ReturnLostForces(nil); n != 1 {
		t.Fatalf("recovered %d items, want 1", n)
	}
	// 5,000 troopers need five carriers; the fee is not refunded.
	if from.Troopers != 10_000 || from.Carriers != 50 || from.Gold != gold+1_000 {
		t.Errorf("sender holds %d troopers, %d carriers, %d gold; want 10000, 50, %d",
			from.Troopers, from.Carriers, from.Gold, gold+1_000)
	}
	want := "No word came back from there. Your trade deal to Receiver has come home: 1000 Gold and 5000 Troopers."
	if got := from.Events[len(from.Events)-1].Text; got != want {
		t.Errorf("sender was told %q, want %q", got, want)
	}

	// The packet was only late: its receipt must not return the carriers again,
	// but the sender learns the goods arrived after all.
	far := NewWorldSeed(cfg, 1)
	far.Config.BoardID = "there"
	far.AddHuman("recv", "Receiver").Protection = 0
	before := len(from.Events)
	home.receive(far.receive(out))
	if from.Carriers != 50 || from.Troopers != 10_000 {
		t.Errorf("a late receipt paid out: %d carriers, %d troopers", from.Carriers, from.Troopers)
	}
	if len(from.Events) != before+1 {
		t.Fatalf("a late receipt filed %d events, want 1", len(from.Events)-before)
	}
	want = "Word came late from there: your trade deal reached Receiver after all."
	if got := from.Events[before].Text; got != want {
		t.Errorf("sender was told %q, want %q", got, want)
	}
}

// A delivered deal leaves nothing for the timer to hand back.
func TestIPTradeDealReceiptClearsInFlight(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	cfg.LostForcesDays = 3
	home := NewWorldSeed(cfg, 1)
	from := home.AddHuman("sender", "Sender")
	from.Protection = 0
	from.Troopers, from.Carriers, from.Gold = 10_000, 50, 5_000_000
	if err := home.SendIPTradeDeal(from, "there", "Receiver", TradeBasket{Troopers: 5_000}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(home.InFlight) != 1 || home.InFlight[0].Kind != "deal" {
		t.Fatalf("in flight: %+v, want one deal", home.InFlight)
	}
	var out Packet
	for _, p := range home.Outbox {
		if p.ToBoard == "there" {
			out = p
		}
	}
	far := NewWorldSeed(cfg, 1)
	far.Config.BoardID = "there"
	far.AddHuman("recv", "Receiver").Protection = 0
	home.receive(far.receive(out))
	if len(home.InFlight) != 0 {
		t.Fatalf("the receipt left %+v in flight", home.InFlight)
	}
	home.GameDay += 10
	if home.ReturnLostForces(nil) != 0 || from.Troopers != 5_000 {
		t.Errorf("a delivered deal came home as well: %d troopers", from.Troopers)
	}
}
