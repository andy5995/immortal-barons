package menu

import (
	"strings"
	"testing"
)

// BRE applies the turn's pending morale penalty, and runs desertion on the
// result, in resolve_civil_unrest — stage 6 of run_player_turn (BRE.EXE 0x3d40),
// straight after food and BEFORE the Covert, Bank, Spending and Attack menus.
// IB applied it at rollover until 2026-09-25, so a turn's attacks fought at the
// morale the previous turn left.
//
// The penalty is seeded the way a boot after the payment stage leaves it —
// maintenance and food committed, the turn not yet past them — and the script
// runs dry at the first menu, so anything that happened happened before it.
func TestCivilUnrestLandsBeforeTheMenus(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Agents = 0 // no covert stage
	p.TurnsLeft = 3
	p.Morale = 100
	p.TurnProgress.IncomeCollected = true
	p.TurnProgress.MaintPaid = true
	p.TurnProgress.Fed = true
	p.PendingMoralePenalty = 30

	f := &fakeSession{keys: []rune(" ")} // the status pause, then dry at the Bank
	runTurn(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Goldie Luck's Bank]") {
		t.Fatalf("the script never reached the Bank menu:\n%s", out)
	}
	if p.TurnsLeft != 3 {
		t.Fatalf("TurnsLeft = %d: the turn was committed, so this proves nothing", p.TurnsLeft)
	}
	if p.Morale != 70 || p.PendingMoralePenalty != 0 || !p.TurnProgress.UnrestResolved {
		t.Errorf("before the menus: morale %d pending %d resolved %v, want 70, 0, true",
			p.Morale, p.PendingMoralePenalty, p.TurnProgress.UnrestResolved)
	}
}

// A civil war is reported by the civil-unrest step itself, before the turn's
// first menu, as the original's resolve_civil_unrest prints it, and it pauses
// there once.
func TestCivilWarIsReportedBeforeTheMenus(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Agents = 0 // no covert stage
	p.TurnsLeft = 3
	p.TurnProgress.IncomeCollected = true
	p.TurnProgress.MaintPaid = true
	p.TurnProgress.Fed = true
	p.CivilWarSeverity = 10

	f := &fakeSession{keys: []rune("  ")} // the unrest pause, the status pause, then dry at the Bank
	runTurn(f, w)

	out := stripANSI(f.out.String())
	bank := strings.Index(out, "Goldie Luck's Bank]")
	if bank < 0 {
		t.Fatalf("the script never reached the Bank menu:\n%s", out)
	}
	war := strings.Index(out, "Civil war!")
	if war < 0 || war > bank {
		t.Errorf("the civil war should be reported before the Bank menu (at %d, bank at %d):\n%s", war, bank, out)
	}
	if p.LastCivilWar != 10 {
		t.Errorf("LastCivilWar = %d, want 10", p.LastCivilWar)
	}
}
