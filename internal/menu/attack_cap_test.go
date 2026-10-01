package menu

import (
	"strings"
	"testing"
)

// The per-day allowance is the original's "InterBBS: Max Individual Attacks"
// (game/reset.hlp), so the local Attack menu must not consult it: a baron who has
// spent every interplanetary strike can still fight the realm next door. IB
// refused them here until 2026-09-18, which on an eight-turn board cost a player
// five attacks a day. The turn is what paces a local attack — hence the Back.
func TestLocalAttackIgnoresTheInterplanetaryAllowance(t *testing.T) {
	w := newWorld()
	w.Config.MaxIndividualAttacks = 1
	w.Player().Protection = 0
	w.Player().Troopers = 1_000_000
	w.Player().AttacksToday = 99 // far past the allowance
	target := recipients(w)[0]
	target.Protection = 0
	before := target.Troopers
	f := &fakeSession{keys: []rune("A\r\r\r\ry\r\r\r\r\r\r\r\r")}

	got := regularAttack(f, w)

	out := f.out.String()
	if !strings.Contains(out, "Send this Attack?") {
		t.Fatalf("the attack was refused before the confirmation; got:\n%s", out)
	}
	if target.Troopers >= before {
		t.Errorf("the attack never landed: target troopers %d -> %d", before, target.Troopers)
	}
	if w.Player().AttacksToday != 99 {
		t.Errorf("AttacksToday = %d: a local attack spent the interplanetary allowance", w.Player().AttacksToday)
	}
	if got != Back {
		t.Errorf("result = %v, want Back — one attack ends the turn's visit to the menu", got)
	}
}

// Max Local Attacks/Day is IB's own limit on top of the turn's one attack. At its
// default of 0 nothing is refused; once set, an attack past it is refused before
// the target is asked for, and an attack that lands is counted.
func TestLocalAttackAllowance(t *testing.T) {
	setup := func(limit, made int) (*ctx, *fakeSession) {
		w := newWorld()
		w.Config.MaxLocalAttacks = limit
		w.Player().Protection = 0
		w.Player().Troopers = 1_000_000
		w.Player().LocalAttacksToday = made
		recipients(w)[0].Protection = 0
		return w, &fakeSession{keys: []rune("A\r\r\r\ry\r\r\r\r\r\r\r\r")}
	}

	w, f := setup(0, 50)
	regularAttack(f, w)
	if !strings.Contains(f.out.String(), "Send this Attack?") {
		t.Fatalf("a limit of 0 refused an attack:\n%s", f.out.String())
	}
	if got := w.Player().LocalAttacksToday; got != 51 {
		t.Errorf("LocalAttacksToday = %d after an attack, want 51", got)
	}

	w, f = setup(2, 2)
	regularAttack(f, w)
	out := f.out.String()
	if !strings.Contains(out, "You have already made all 2 of your attacks for today.") {
		t.Errorf("the spent allowance was not refused:\n%s", out)
	}
	if strings.Contains(out, "Send this Attack?") {
		t.Errorf("the refusal came after the attack was set up:\n%s", out)
	}
}
