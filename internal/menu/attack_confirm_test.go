package menu

import (
	"strings"
	"testing"
)

// The local attack's confirmation is IB's own (the original asks it only on the
// interplanetary paths, where it follows a gold cost), and it exists because
// every force prompt above it defaults to the maximum. Declining must leave the
// target and the attacker's turn untouched. Asserts the prompt was REACHED as
// well as the state effect, so a flow change upstream cannot leave this green.
func TestLocalAttackConfirmRefusedAbortsTheAttack(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	w.Player().Troopers = 1_000_000
	target := recipients(w)[0]
	target.Protection = 0
	before, attacks := target.Troopers, w.Player().AttacksToday
	f := &fakeSession{keys: []rune("A\r\r\r\rn")}

	regularAttack(f, w)

	out := f.out.String()
	if !strings.Contains(out, "Send this Attack?") {
		t.Fatalf("the confirmation was never reached; got:\n%s", out)
	}
	if target.Troopers != before {
		t.Errorf("a declined attack still landed: target troopers %d -> %d", before, target.Troopers)
	}
	if w.Player().AttacksToday != attacks {
		t.Errorf("a declined attack still spent one: %d -> %d", attacks, w.Player().AttacksToday)
	}
}
