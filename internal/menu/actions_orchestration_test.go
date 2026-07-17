package menu

import (
	"strings"
	"testing"
)

// These tests exercise the menu handlers' orchestration branches — the
// prompt -> validate -> reject-or-mutate flow — using the scripted fakeSession.
// The underlying game rules (Attack, SendGold, BuyRegions) are covered in the
// game package; here we check that a handler rejects bad input *without*
// mutating state, and applies the mutation on good input.

// --- regularAttack ------------------------------------------------------

func TestRegularAttackBlockedByProtection(t *testing.T) {
	w := newWorld()
	// Under New Realm Protection w.Targets() is empty, so reach the rival empire
	// via recipients() (which protection does not gate) to check it is untouched.
	target := recipients(w)[0]
	before := target.Land
	f := &fakeSession{} // returns before prompting; pause() reads EOF harmlessly

	regularAttack(f, w)

	if !strings.Contains(f.out.String(), "New Realm Protection") {
		t.Errorf("expected New Realm Protection notice; got:\n%s", f.out.String())
	}
	if target.Land != before {
		t.Errorf("target Land changed under protection: %d -> %d", before, target.Land)
	}
}

func TestRegularAttackCancelDoesNotAttack(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	target := recipients(w)[0]
	target.Protection = 0 // clear the rival's new-realm protection so it is attackable
	before := target.Land
	f := &fakeSession{keys: []rune("0\r")} // 0 = cancel at the target prompt

	regularAttack(f, w)

	if target.Land != before {
		t.Errorf("cancel still attacked: target Land %d -> %d", before, target.Land)
	}
}

func TestRegularAttackInvalidIndexDoesNotAttack(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	target := recipients(w)[0]
	target.Protection = 0 // clear the rival's new-realm protection so it is attackable
	before := target.Land
	f := &fakeSession{keys: []rune("9\r")} // only one target exists; 9 is out of range

	regularAttack(f, w)

	if target.Land != before {
		t.Errorf("out-of-range index still attacked: target Land %d -> %d", before, target.Land)
	}
}

func TestRegularAttackHitsTarget(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	w.Player().Troopers = 1_000_000 // overwhelming offense: the player wins and captures regions
	target := recipients(w)[0]
	target.Protection = 0 // clear the rival's new-realm protection so it is attackable
	before := target.Land
	f := &fakeSession{keys: []rune("A")} // target A; full-force defaults via EOF

	regularAttack(f, w)

	if target.Land >= before {
		t.Errorf("winning attack captured no regions: target Land %d -> %d", before, target.Land)
	}
}

// --- sendTradeDeal ------------------------------------------------------

func TestSendTradeDealCancelRecipient(t *testing.T) {
	w := newWorld()
	p := w.Player()
	to := recipients(w)[0]
	pGold, toGold := p.Gold, to.Gold
	f := &fakeSession{keys: []rune("0")} // 0 = cancel at recipient picker

	sendTradeDeal(f, w)

	if p.Gold != pGold || to.Gold != toGold {
		t.Errorf("cancel moved gold: player %d->%d, target %d->%d", pGold, p.Gold, toGold, to.Gold)
	}
}

func TestSendTradeDealZeroAmountSendsNothing(t *testing.T) {
	w := newWorld()
	p := w.Player()
	to := recipients(w)[0]
	pGold, toGold := p.Gold, to.Gold
	f := &fakeSession{keys: []rune("A0\r")} // pick A, then amount 0

	sendTradeDeal(f, w)

	if p.Gold != pGold || to.Gold != toGold {
		t.Errorf("zero amount moved gold: player %d->%d, target %d->%d", pGold, p.Gold, toGold, to.Gold)
	}
}

func TestSendTradeDealSuccess(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 1000
	to := recipients(w)[0]
	toGold := to.Gold
	f := &fakeSession{keys: []rune("A100\r")} // pick A, send 100

	sendTradeDeal(f, w)

	if p.Gold != 900 {
		t.Errorf("player Gold = %d, want 900", p.Gold)
	}
	if to.Gold != toGold+100 {
		t.Errorf("target Gold = %d, want %d", to.Gold, toGold+100)
	}
}

func TestSendTradeDealCantAffordSendsNothing(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 50
	to := recipients(w)[0]
	toGold := to.Gold
	f := &fakeSession{keys: []rune("A100\r")} // wants to send 100 with only 50

	sendTradeDeal(f, w)

	if p.Gold != 50 || to.Gold != toGold {
		t.Errorf("insufficient funds still moved gold: player 50->%d, target %d->%d", p.Gold, toGold, to.Gold)
	}
}

// --- buyLand ------------------------------------------------------------

func TestBuyLandBuysRegion(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 1_000_000 // ensure at least one region is affordable
	beforeCoastal := p.Regions.Coastal
	beforeGold := p.Gold
	f := &fakeSession{keys: []rune("C1\r0")} // Coastal, buy 1, then 0 to leave

	buyLand(f, w)

	if p.Regions.Coastal != beforeCoastal+1 {
		t.Errorf("Coastal = %d, want %d", p.Regions.Coastal, beforeCoastal+1)
	}
	if p.Gold >= beforeGold {
		t.Errorf("gold not spent: %d -> %d", beforeGold, p.Gold)
	}
}

func TestBuyLandZeroQuantityBuysNothing(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 1_000_000
	beforeCoastal := p.Regions.Coastal
	beforeGold := p.Gold
	f := &fakeSession{keys: []rune("C0\r0")} // Coastal, quantity 0, then leave

	buyLand(f, w)

	if p.Regions.Coastal != beforeCoastal || p.Gold != beforeGold {
		t.Errorf("zero quantity changed state: Coastal %d->%d, Gold %d->%d",
			beforeCoastal, p.Regions.Coastal, beforeGold, p.Gold)
	}
}

func TestBuyLandCancelImmediately(t *testing.T) {
	w := newWorld()
	p := w.Player()
	beforeCoastal := p.Regions.Coastal
	beforeGold := p.Gold
	f := &fakeSession{keys: []rune("0")} // 0 at the region picker leaves at once

	buyLand(f, w)

	if p.Regions.Coastal != beforeCoastal || p.Gold != beforeGold {
		t.Errorf("cancel changed state: Coastal %d->%d, Gold %d->%d",
			beforeCoastal, p.Regions.Coastal, beforeGold, p.Gold)
	}
}

func TestBuyLandRedisplayRedrawsMenu(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("?0")} // ? redraws the list, then 0 leaves

	buyLand(f, w)

	if n := strings.Count(f.out.String(), "Buy Regions"); n < 2 {
		t.Errorf("expected the region menu drawn at least twice (initial + redisplay), got %d", n)
	}
}

func TestBuyLandAdvisorsThenReturns(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("* 0")} // * = Advisors, space clears its pause, 0 leaves

	buyLand(f, w)

	// The Advisors branch shows the screen and then redraws the region menu, so
	// "Buy Regions" appears at least twice (initial draw + post-Advisors draw).
	if n := strings.Count(f.out.String(), "Buy Regions"); n < 2 {
		t.Errorf("expected the region menu redrawn after Advisors, got %d draws", n)
	}
}
