package menu

import (
	"strings"
	"testing"

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
