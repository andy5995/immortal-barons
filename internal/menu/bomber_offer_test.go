package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// bomberOpWorld is a board where a Special Operation can be sent but the caller
// holds too few bombers to deliver it.
func bomberOpWorld(t *testing.T, gold, bank int64) *ctx {
	t.Helper()
	w := newWorld()
	w.bank = BuildMenus().Bank
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.Config.BombingOps = true
		w.World.ImportBoard(game.RemoteBoard{BoardID: "Mars"})
		p := w.Player()
		p.Protection = 0
		p.Bombers = 100
		p.Gold, p.Bank = gold, bank
	})
	return w
}

func bombers(w *ctx) int {
	var n int
	w.Read(func() { n = w.Player().Bombers })
	return n
}

func sent(w *ctx) int {
	var n int
	w.Read(func() { n = len(w.World.Outbox) })
	return n
}

// Short of bombers with the gold for them and the op, the caller picks the
// target, hears the price of both, buys the bombers, and the op goes.
func TestTooFewBombersOffersToBuyThem(t *testing.T) {
	w := bomberOpWorld(t, 1_000_000_000, 0)
	f := &fakeSession{keys: []rune("Mars\ry1 ")}
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrNeedBombers.Error()) || !strings.Contains(out, "Bombers Needed") {
		t.Fatalf("no refusal and offer:\n%s", out)
	}
	if strings.Index(out, "against which planet") > strings.Index(out, "Bombers Needed") {
		t.Errorf("the bombers were offered before the target was picked:\n%s", out)
	}
	if !strings.Contains(out, "Buy 400 Bombers") || !strings.Contains(out, "with the operation") {
		t.Errorf("the offer does not name the shortfall and the total:\n%s", out)
	}
	if n := sent(w); n != 1 {
		t.Fatalf("%d ops sent, want the one paid for:\n%s", n, out)
	}
	if got := bombers(w); got != 0 {
		t.Errorf("bombers %d after buying 400 and launching, want 0", got)
	}
}

// Short of the gold too, the bank is offered first for the bombers and the op
// together; what is withdrawn pays for both.
func TestTooFewBombersAndGoldOffersTheBankFirst(t *testing.T) {
	w := bomberOpWorld(t, 0, 1_000_000_000)
	f := &fakeSession{keys: []rune("Mars\ry11 ")} // (1) Withdraw, then (1) Buy
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Gold Needed") || !strings.Contains(out, "Withdraw") {
		t.Fatalf("no bank offer:\n%s", out)
	}
	if strings.Index(out, "Gold Needed") > strings.Index(out, "Bombers Needed") {
		t.Errorf("the bomber offer came before the bank:\n%s", out)
	}
	if n := sent(w); n != 1 {
		t.Errorf("%d ops sent after the withdrawal, want 1:\n%s", n, out)
	}
}

// Back from the bank no richer, the caller is told so and nothing is bought.
func TestTooFewBombersStillShortAfterTheBank(t *testing.T) {
	w := bomberOpWorld(t, 0, 0)
	f := &fakeSession{keys: []rune("Mars\ry10 ")} // (1) Visit the bank, quit it, pause
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrCantAfford.Error()) {
		t.Fatalf("no refusal after the bank:\n%s", out)
	}
	if strings.Contains(out, "Bombers Needed") || bombers(w) != 100 || sent(w) != 0 {
		t.Errorf("bombers were offered or bought without the gold:\n%s", out)
	}
}

// A single missing bomber reads in the singular.
func TestOneMissingBomberIsSingular(t *testing.T) {
	w := bomberOpWorld(t, 1_000_000_000, 0)
	w.With(func() { w.Player().Bombers = 499 })
	f := &fakeSession{keys: []rune("Mars\ry1 ")}
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Buy 1 Bomber for") || !strings.Contains(out, "1 Bomber purchased.") {
		t.Errorf("want the singular:\n%s", out)
	}
}

// Quitting the offer buys nothing and sends nothing.
func TestQuittingTheBomberOfferBuysNothing(t *testing.T) {
	w := bomberOpWorld(t, 1_000_000_000, 0)
	f := &fakeSession{keys: []rune("Mars\ry0")}
	ipSpecialOp(game.OpBombFood)(f, w)

	if out := stripANSI(f.out.String()); !strings.Contains(out, "Bombers Needed") {
		t.Fatalf("the offer was never drawn:\n%s", out)
	}
	if got := bombers(w); got != 100 || sent(w) != 0 {
		t.Errorf("bombers %d and %d ops sent, want the 100 held before and none", got, sent(w))
	}
}

// A league that sells no military gets the plain refusal, before a target is
// asked for.
func TestTooFewBombersWithoutAMarketIsRefused(t *testing.T) {
	w := bomberOpWorld(t, 1_000_000_000, 0)
	w.With(func() { w.World.Config.BuyMilitary = game.BuyNo })
	f := &fakeSession{keys: []rune(" ")}
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrNeedBombers.Error()) {
		t.Fatalf("no refusal:\n%s", out)
	}
	if strings.Contains(out, "Bombers Needed") || strings.Contains(out, "against which planet") {
		t.Errorf("walked on past a refusal that cannot be answered:\n%s", out)
	}
}
