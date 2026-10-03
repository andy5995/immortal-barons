package menu

import (
	"strings"
	"testing"
)

// A staged attack must reach the screen, in order, and carry no running
// figures: the report after it is the only place the losses are told.
func TestRegularAttackStagesTheBattle(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	w.Player().Troopers = 1_000_000
	target := recipients(w)[0]
	target.Protection = 0
	troopersBefore := target.Troopers
	f := &fakeSession{keys: []rune("A\r\r\r\ry")}

	regularAttack(f, w)

	out := f.out.String()
	for _, want := range []string{"Attacking " + target.Name, "Advancing...", "Pushing..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("staged attack never printed %q; output was:\n%s", want, out)
		}
	}
	if target.Troopers >= troopersBefore {
		t.Errorf("the battle itself did not run: target troopers %d -> %d", troopersBefore, target.Troopers)
	}
	if i, j, k := strings.Index(out, "Attacking "), strings.Index(out, "Advancing..."), strings.Index(out, "Pushing..."); !(i < j && j < k) {
		t.Errorf("the stages are out of order: Attacking at %d, Advancing at %d, Pushing at %d", i, j, k)
	}
	if strings.Contains(out, "so far") {
		t.Error("a stage printed running losses")
	}
}
