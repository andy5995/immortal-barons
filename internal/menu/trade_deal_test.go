package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A pending trade deal is surfaced to the recipient at turn start; accepting
// completes the barter (they receive the offered goods and pay the demanded ones).
func TestReviewTradeDealsAcceptCompletesBarter(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	p.Gold, p.Tanks = 100_000, 0
	otherGoldBefore := other.Gold
	p.TradeDeals = []game.TradeDeal{{
		From:   other.Name,
		Send:   game.TradeBasket{Tanks: 100},
		Demand: game.TradeBasket{Gold: 5_000},
	}}

	f := &fakeSession{keys: []rune("y")}
	reviewTradeDeals(f, w)
	out := f.out.String()

	if !strings.Contains(out, other.Name) || !strings.Contains(out, "Tanks") {
		t.Errorf("review should show the offer, got:\n%s", out)
	}
	if p.Tanks != 100 {
		t.Errorf("accept should deliver the offered tanks: %d, want 100", p.Tanks)
	}
	if p.Gold != 95_000 {
		t.Errorf("accept should take the demanded gold: %d, want 95000", p.Gold)
	}
	if other.Gold != otherGoldBefore+5_000 {
		t.Errorf("sender should receive the demanded gold: %d, want %d", other.Gold, otherGoldBefore+5_000)
	}
	if len(w.Player().TradeDeals) != 0 {
		t.Error("accepted deal should be removed")
	}
}

// Declining leaves the recipient's goods untouched and drops the deal.
func TestReviewTradeDealsDeclineDropsIt(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	p.Gold, p.Tanks = 100_000, 0
	p.TradeDeals = []game.TradeDeal{{
		From:   other.Name,
		Send:   game.TradeBasket{Tanks: 100},
		Demand: game.TradeBasket{Gold: 5_000},
	}}

	f := &fakeSession{keys: []rune("n")}
	reviewTradeDeals(f, w)

	if p.Gold != 100_000 || p.Tanks != 0 {
		t.Errorf("decline must not move goods: Gold=%d Tanks=%d", p.Gold, p.Tanks)
	}
	if len(w.Player().TradeDeals) != 0 {
		t.Error("declined deal should be removed")
	}
}

func TestBasketSummary(t *testing.T) {
	f := &fakeSession{}
	got := basketSummary(f, game.TradeBasket{Tanks: 100, Gold: 5_000})
	if !strings.Contains(got, "100 Tanks") || !strings.Contains(got, "5,000 Gold") {
		t.Errorf("basketSummary = %q, want it to list Tanks and Gold", got)
	}
	if basketSummary(f, game.TradeBasket{}) != "nothing" {
		t.Errorf("empty basket should summarize as %q", "nothing")
	}
}

// An ignored deal stays pending and is put to the player again the next time
// they enter the game, however often that is in a day (#175). Nothing expires it
// and nothing marks it seen, which is BRE's behaviour: run_player_turn calls
// process_trade_offer on every entry that has a turn to play (BRE.EXE 0x3855),
// with no per-day gate anywhere on the path.
func TestIgnoredTradeDealComesBackOnTheNextEntry(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	goldBefore, otherGoldBefore := p.Gold, other.Gold
	p.TradeDeals = []game.TradeDeal{{
		From:   other.Name,
		Send:   game.TradeBasket{Tanks: 100},
		Demand: game.TradeBasket{Gold: 5_000},
	}}

	f := &fakeSession{keys: []rune("i")}
	reviewTradeDeals(f, w)
	if !strings.Contains(f.out.String(), "offers you a trade deal") {
		t.Fatalf("first entry should present the deal, got:\n%s", f.out.String())
	}
	if len(w.Player().TradeDeals) != 1 {
		t.Fatalf("ignore should leave the deal pending, got %d", len(w.Player().TradeDeals))
	}
	if p.Gold != goldBefore || other.Gold != otherGoldBefore {
		t.Errorf("ignore should move nothing: player %d->%d, sender %d->%d",
			goldBefore, p.Gold, otherGoldBefore, other.Gold)
	}

	f2 := &fakeSession{keys: []rune("i")}
	reviewTradeDeals(f2, w)
	if !strings.Contains(f2.out.String(), "offers you a trade deal") {
		t.Errorf("the next entry should present it again, got:\n%s", f2.out.String())
	}
}
