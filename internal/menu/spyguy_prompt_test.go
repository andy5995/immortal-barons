package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// spyGuyWorld is a board that knows one other planet, with the caller's gold set
// to whatever the case needs.
func spyGuyWorld(t *testing.T, gold, bank int64) *ctx {
	t.Helper()
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.ImportBoard(game.RemoteBoard{BoardID: "Mars"})
		p := w.Player()
		p.Protection = 0
		p.Gold, p.Bank = gold, bank
	})
	w.bank = BuildMenus().Bank
	return w
}

// The prompt offers the whole stay a SpyGuy can be paid for, not the part this
// baron's gold covers: the ceiling is the office's, and the money is dealt with
// afterwards.
func TestSpyGuyOffersTheFullStay(t *testing.T) {
	w := spyGuyWorld(t, 1_000_000_000, 0)
	f := &fakeSession{keys: []rune("1\r\r\r")} // planet 1, default days, pause
	sendSpyGuy(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "(3; 15)") {
		t.Errorf("the prompt should offer up to %d days:\n%s", game.SpyGuyMaxDays, out)
	}
}

// A baron who cannot pay is offered the bank where they stand, and what they
// draw out pays for the stay.
func TestSpyGuyShortOfGoldIsOfferedTheBank(t *testing.T) {
	w := spyGuyWorld(t, 0, 1_000_000_000)
	var perDay int64
	w.Read(func() { perDay = w.SpyGuyCostPerDay() })
	if perDay == 0 {
		t.Fatal("this test needs a non-zero daily rate to prove anything")
	}
	// planet 1, 15 days, "y" to the bank, (W)ithdraw everything, quit the bank.
	f := &fakeSession{keys: []rune("1\r15\ryW1000000000\r0\r\r")}
	sendSpyGuy(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Visit the bank?") {
		t.Fatalf("no bank offer:\n%s", out)
	}
	if !strings.Contains(out, "will watch it for 15 days") {
		t.Errorf("the stay was not paid for after the bank visit:\n%s", out)
	}
	var sent bool
	w.Read(func() { sent = len(w.World.Outbox) > 0 })
	if !sent {
		t.Errorf("no watcher left the board:\n%s", out)
	}
}

// A baron who still cannot pay when they come back is told so, and nothing is
// sent or charged.
func TestSpyGuyStillShortIsRefused(t *testing.T) {
	w := spyGuyWorld(t, 0, 0)
	f := &fakeSession{keys: []rune("1\r15\rn\r")} // planet 1, 15 days, no bank
	sendSpyGuy(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "gold more than you have in hand") {
		t.Fatalf("the shortfall was not named:\n%s", out)
	}
	if !strings.Contains(out, game.ErrCantAfford.Error()) {
		t.Errorf("the send was not refused:\n%s", out)
	}
	var sent bool
	w.Read(func() { sent = len(w.World.Outbox) > 0 })
	if sent {
		t.Errorf("a watcher left despite the refusal:\n%s", out)
	}
}
