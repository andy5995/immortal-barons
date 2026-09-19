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
	// Target A, (1) Withdraw the shortfall, one agent, and the acknowledgment's
	// pause. The refusal itself does not pause: the offer follows it straight
	// away.
	f := &fakeSession{keys: []rune("A11\r ")}
	sendAgents(f, w, row)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrCantAfford.Error()) {
		t.Fatalf("the refusal was not shown first:\n%s", out)
	}
	if !strings.Contains(out, "Withdraw") || !strings.Contains(out, "Visit the bank") {
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
	if !strings.Contains(out, game.ErrCantAfford.Error()) {
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
	f := &fakeSession{keys: []rune("Mars\ry10 ")} // accept, (1) Visit the bank, quit it
	ipSpecialOp(game.OpBombFood)(f, w)

	out := stripANSI(f.out.String())
	if n := strings.Count(out, game.ErrCantAfford.Error()); n != 2 {
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
	if n := strings.Count(out, game.ErrCantAfford.Error()); n != 1 {
		t.Errorf("the refusal appears %d times, want once:\n%s", n, out)
	}
}

// A missile beyond the gold in hand is refused and the bank offered, and what
// is withdrawn pays for it. The original spends the bank without asking; see
// payArmsDealer on the divergence.
func TestMissileShortOfGoldIsOfferedTheBank(t *testing.T) {
	w := newWorld()
	w.bank = BuildMenus().Bank
	w.With(func() {
		p := w.Player()
		p.Protection, p.Gold, p.Bank = 0, 0, 1_000_000_000
		for _, e := range w.World.Empires {
			e.Protection = 0
		}
	})
	// Target A, buy it, (1) Withdraw the shortfall, pause.
	f := &fakeSession{keys: []rune("Ay1 ")}
	nuclearAttack(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrCantAfford.Error()) || !strings.Contains(out, "Withdraw") {
		t.Fatalf("the missile was not refused and offered the bank:\n%s", out)
	}
	var gold, bank int64
	w.Read(func() { p := w.Player(); gold, bank = p.Gold, p.Bank })
	if gold+bank >= 1_000_000_000 {
		t.Errorf("nothing was spent after the withdrawal: gold %d, bank %d", gold, bank)
	}
}

// The offer's second option opens the bank menu itself, for the baron who wants
// a loan rather than the withdrawal it suggests.
func TestShortOfGoldOfferOpensTheBankMenu(t *testing.T) {
	w := newWorld()
	w.bank = BuildMenus().Bank
	w.With(func() { p := w.Player(); p.Gold, p.Bank = 0, 5_000 })

	f := &fakeSession{keys: []rune("20")} // (2) Visit the bank, then quit it
	if !offerBank(f, w, 1_000) {
		t.Error("visiting the bank should report the visit")
	}
	if out := stripANSI(f.out.String()); !strings.Contains(out, "Goldie Luck's Bank") {
		t.Errorf("the bank menu was not drawn:\n%s", out)
	}
}
