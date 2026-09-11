package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A baron who cannot pay for a covert operation is refused in the original's
// words and then offered the bank, where the SpyGuy's stay already sends one.
// What they draw out pays for the op, and the target chosen before the trip
// still stands.
func TestCovertShortOfGoldIsOfferedTheBank(t *testing.T) {
	w := newWorld()
	var row covertRow
	for _, r := range covertRows {
		if r.Op == game.OpStirRevolts {
			row = r
		}
	}
	w.With(func() {
		p := w.Player()
		p.Protection = 0
		p.Agents, p.Gold, p.Bank = 5, 0, 1_000_000_000
		for _, e := range w.World.Empires {
			e.Protection = 0
		}
	})
	w.bank = BuildMenus().Bank
	// Target A, "y" to the bank, withdraw the lot, quit the bank, one agent, pause.
	// Target A, "y" to the bank, withdraw the lot, quit the bank, one agent, and
	// the acknowledgement's pause. The refusal itself does not pause: the bank
	// question follows it straight away.
	f := &fakeSession{keys: []rune("AyW1000000000\r01\r ")}
	sendAgents(f, w, row)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrCantAffordCovert.Error()) {
		t.Fatalf("the refusal was not shown first:\n%s", out)
	}
	if !strings.Contains(out, "Visit the bank?") {
		t.Fatalf("no bank offer:\n%s", out)
	}
	if !strings.Contains(out, "Sent out") {
		t.Errorf("the op did not go after the bank visit:\n%s", out)
	}
	var queued int
	w.Read(func() { queued = len(w.World.CovertQueue) })
	if queued != 1 {
		t.Errorf("covert queue holds %d, want the one op", queued)
	}
}

// Coming back from the bank no richer stops there, with the refusal already
// said and nothing sent.
func TestCovertStillShortSendsNothing(t *testing.T) {
	w := newWorld()
	var row covertRow
	for _, r := range covertRows {
		if r.Op == game.OpStirRevolts {
			row = r
		}
	}
	w.With(func() {
		p := w.Player()
		p.Protection = 0
		p.Agents, p.Gold, p.Bank = 5, 0, 0
		for _, e := range w.World.Empires {
			e.Protection = 0
		}
	})
	w.bank = BuildMenus().Bank
	// Target A, then "n" to the bank.
	f := &fakeSession{keys: []rune("An")}
	sendAgents(f, w, row)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrCantAffordCovert.Error()) {
		t.Fatalf("no refusal:\n%s", out)
	}
	if strings.Contains(out, "Sent out") {
		t.Errorf("an agent went out unpaid for:\n%s", out)
	}
	var queued int
	w.Read(func() { queued = len(w.World.CovertQueue) })
	if queued != 0 {
		t.Errorf("covert queue holds %d, want none", queued)
	}
}

// Walking into the bank and back out no richer is told why nothing happened.
// The first refusal is a screen of bank menu away by then, and an action that
// simply stops is what this whole path exists to avoid.
func TestBankVisitThatChangesNothingSaysSoAgain(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.Config.BombingOps = true
		w.World.ImportBoard(game.RemoteBoard{BoardID: "Mars"})
		p := w.Player()
		p.Protection = 0
		p.Bombers = game.BombingBombersRequired
		p.Gold, p.Bank = 0, 0
	})
	w.bank = BuildMenus().Bank
	f := &fakeSession{keys: []rune("Mars\ryy0 ")} // accept, visit the bank, quit it
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if n := strings.Count(out, game.ErrCantAffordOp.Error()); n != 2 {
		t.Errorf("the refusal appears %d times, want twice — once before the bank and once after:\n%s", n, out)
	}
	var sent int
	w.Read(func() { sent = len(w.World.Outbox) })
	if sent != 0 {
		t.Errorf("an op was sent unpaid for")
	}
}

// Declining the bank is told once, not twice: the refusal is still on screen
// directly above the question they just answered.
func TestDecliningTheBankIsNotToldTwice(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.Config.BombingOps = true
		w.World.ImportBoard(game.RemoteBoard{BoardID: "Mars"})
		p := w.Player()
		p.Protection = 0
		p.Bombers = game.BombingBombersRequired
		p.Gold, p.Bank = 0, 0
	})
	w.bank = BuildMenus().Bank
	f := &fakeSession{keys: []rune("Mars\ryn")} // accept, then decline the bank
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if n := strings.Count(out, game.ErrCantAffordOp.Error()); n != 1 {
		t.Errorf("the refusal appears %d times, want once:\n%s", n, out)
	}
}
