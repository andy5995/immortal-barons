package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// newIPDealCtx sets up a two-planet league with one clear baron and one under
// New Realm Protection on the far planet.
func newIPDealCtx(t *testing.T) *ctx {
	t.Helper()
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		w.Config.BoardID = "Home BBS"
		w.ImportBoard(game.RemoteBoard{BoardID: "Far BBS", Scores: []game.RemoteScore{
			{Empire: "Open Realm", Land: 100, NetWorth: 5000, Score: 900},
			{Empire: "Fresh Realm", Land: 20, NetWorth: 500, Score: 100, Protected: true},
		}})
		p := w.Player()
		p.Protection = 0
		p.Troopers, p.Carriers, p.Gold = 100_000, 500, 50_000_000
	})
	return w
}

// The interplanetary item must run the INTERPLANETARY mechanic, not the local
// one it was wired to until #195. The tell is unmistakable: the local action
// picks its recipient from the realms on THIS board and asks for a Request
// basket and a day span, none of which this flow has.
func TestInterplanetarySendTradeDealShipsToAnotherPlanet(t *testing.T) {
	w := newIPDealCtx(t)
	before := 0
	w.With(func() { before = w.Player().Troopers })

	// Planet, baron, then the goods: key 1 is Troopers in the basket builder,
	// 5000, done, then confirm.
	f := &fakeSession{keys: []rune("1\r" + "A" + "1" + "5000\r" + "0" + "y" + " ")}
	sendIPTradeDeal(f, w)
	out := stripANSI(f.out.String())

	// Reached the screen: the one-way warning belongs to this flow and to no
	// other, and a day-span prompt would mean the local action ran.
	if !strings.Contains(out, "cannot refuse them") {
		t.Fatalf("never reached the interplanetary deal screen:\n%s", out)
	}
	if strings.Contains(out, "days to send it for") {
		t.Fatalf("the LOCAL trade action ran — that is #195:\n%s", out)
	}

	// And the state effect: the goods left, and a shipment is queued for the far
	// board addressed to the realm that was chosen.
	var queued []game.IPTradeDeal
	var after int
	w.With(func() {
		after = w.Player().Troopers
		for _, p := range w.Outbox {
			if p.ToBoard == "Far BBS" {
				queued = append(queued, p.TradeDeals...)
			}
		}
	})
	if after != before-5_000 {
		t.Errorf("sender holds %d troopers, want %d", after, before-5_000)
	}
	if len(queued) != 1 {
		t.Fatalf("queued %d shipments, want 1:\n%s", len(queued), out)
	}
	if queued[0].ToEmpire != "Open Realm" {
		t.Errorf("addressed to %q, want the baron that was picked", queued[0].ToEmpire)
	}
	if queued[0].Goods.Troopers != 5_000 {
		t.Errorf("shipped %+v, want 5,000 troopers", queued[0].Goods)
	}
}

// A realm the last scores packet had under protection is LISTED — #214 stopped
// hiding them — flagged, and refused when picked. Andy's call; the original
// instead lets the deal go and destroys it on arrival.
func TestInterplanetaryDealRefusesAProtectedRealm(t *testing.T) {
	w := newIPDealCtx(t)
	before := 0
	w.With(func() { before = w.Player().Troopers })

	f := &fakeSession{keys: []rune("1\r" + "B" + " ")}
	sendIPTradeDeal(f, w)
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "Fresh Realm") {
		t.Fatalf("the protected realm was not even listed:\n%s", out)
	}
	if !strings.Contains(out, "New Realm Protection") {
		t.Fatalf("picking a protected realm was not refused:\n%s", out)
	}
	var queued int
	var after int
	w.With(func() {
		after = w.Player().Troopers
		for _, p := range w.Outbox {
			queued += len(p.TradeDeals)
		}
	})
	if queued != 0 {
		t.Errorf("a refused deal was queued anyway (%d)", queued)
	}
	if after != before {
		t.Errorf("a refused deal still moved goods: %d troopers, was %d", after, before)
	}
}

// Deals Out lists a shipped deal, what it carries and when it comes home.
func TestDealsOutListsAShippedDeal(t *testing.T) {
	w := newIPDealCtx(t)
	empty := &fakeSession{keys: []rune(" ")}
	ipPendingDeals(empty, w)
	if !strings.Contains(stripANSI(empty.out.String()), "You have no trade deals out.") {
		t.Fatalf("an empty list should say so:\n%s", empty.out.String())
	}

	w.With(func() {
		w.Config.LostForcesDays = 3
		if err := w.World.SendIPTradeDeal(w.Player(), "Far BBS", "Open Realm", game.TradeBasket{Troopers: 5_000}); err != nil {
			t.Fatalf("send: %v", err)
		}
	})
	f := &fakeSession{keys: []rune(" ")}
	ipPendingDeals(f, w)
	out := stripANSI(f.out.String())
	for _, want := range []string{"Trade Deals Awaiting a Receipt", "  Open Realm of Far BBS\n",
		// Sent today: the launch day does not count, so a wait of 3 ends at the
		// fourth day change.
		"    Cargo: 5,000 Troopers\n", "    Returns in: 4 days\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// With the world on the host's date, the line names the moment instead: the
	// host's midnight four days on, in the reader's zone (UTC by default).
	now := game.Now()
	w.With(func() { w.Today = now.Format("2006-01-02") })
	f = &fakeSession{keys: []rune(" ")}
	ipPendingDeals(f, w)
	due := time.Date(now.Year(), now.Month(), now.Day()+4, 0, 0, 0, 0, now.Location())
	// Wrapping folds the stamp's double space, so compare word by word.
	got := strings.Join(strings.Fields(stripANSI(f.out.String())), " ")
	if want := "Returns after: " + strings.Join(strings.Fields(game.Stamp(due, time.UTC)), " "); !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, f.out.String())
	}
}

// The day turns at the host's midnight, counted across a month's end.
func TestBoardMidnight(t *testing.T) {
	now := time.Date(2026, 10, 30, 21, 15, 0, 0, time.UTC)
	got, ok := boardMidnight(now, "2026-10-30", 4)
	if want := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC); !ok || !got.Equal(want) {
		t.Errorf("boardMidnight = %v, %v; want %v", got, ok, want)
	}
	if _, ok := boardMidnight(now, "2026-10-29", 4); ok {
		t.Error("boardMidnight counted from a date the world is not on")
	}
}

// While its board is held a trade's countdown is the backstop, the latest it can
// come home, so the line says "by" rather than "after".
func TestTradeReturnWhileHeld(t *testing.T) {
	w := newIPDealCtx(t)
	f := &fakeSession{}
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	if got, want := tradeReturn(f, w, now, "2026-10-13", 15, true), "Returns by: 10/28/2026  00:00:00 UTC"; got != want {
		t.Errorf("held, dated: %q, want %q", got, want)
	}
	if got, want := tradeReturn(f, w, now, "2026-10-13", 4, false), "Returns after: 10/17/2026  00:00:00 UTC"; got != want {
		t.Errorf("dated: %q, want %q", got, want)
	}
	if got, want := tradeReturn(f, w, now, "2026-10-12", 15, true), "Returns within: 15 days"; got != want {
		t.Errorf("held, off the world's date: %q, want %q", got, want)
	}
	if got, want := tradeReturn(f, w, now, "2026-10-13", 0, true), "Returns in: next planetary run"; got != want {
		t.Errorf("wait up: %q, want %q", got, want)
	}
}

// Bids Out says when each bid's gold comes home, as Deals Out does for a deal.
func TestBidsOutSaysWhenTheGoldComesHome(t *testing.T) {
	w := newIPDealCtx(t)
	w.With(func() {
		w.World.InFlight = append(w.World.InFlight, game.InFlightStrike{ID: 1, Kind: "trade", Owner: w.Player().Owner,
			TargetBoard: "Far BBS", TargetEmpire: "Seller", Good: "Tank", Qty: 40, Gold: 20_000, LaunchedDay: w.GameDay})
	})
	now := game.Now()
	w.With(func() { w.Today = now.Format("2006-01-02") })
	f := &fakeSession{keys: []rune(" ")}
	ipPendingBids(f, w)
	// Sent today, a bid waits game.IPTradeWaitDays full days: home after the
	// host's midnight four days on. Wrapping folds the stamp's double space.
	due := time.Date(now.Year(), now.Month(), now.Day()+4, 0, 0, 0, 0, now.Location())
	out := strings.Join(strings.Fields(stripANSI(f.out.String())), " ")
	for _, want := range []string{"Bids Awaiting an Answer", "40 Tank from Seller of Far BBS",
		"Returns after: " + strings.Join(strings.Fields(game.Stamp(due, time.UTC)), " ")} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, f.out.String())
		}
	}
}
