package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A staged attack must reach the screen and carry the battle's own figures. It
// asserts the marker text AND that the second snapshot never exceeds the report
// it is a share of, because a stage that overstates the losses would be telling
// the player something the battle did not do.
func TestRegularAttackStagesTheBattle(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	w.Player().Troopers = 1_000_000
	target := recipients(w)[0]
	target.Protection = 0
	troopersBefore := target.Troopers
	f := &fakeSession{keys: []rune("A")}

	regularAttack(f, w)

	out := f.out.String()
	for _, want := range []string{"Attacking " + target.Name, "Pushing...", "Your losses so far", "Their losses so far"} {
		if !strings.Contains(out, want) {
			t.Fatalf("staged attack never printed %q; output was:\n%s", want, out)
		}
	}
	if target.Troopers >= troopersBefore {
		t.Errorf("the battle itself did not run: target troopers %d -> %d", troopersBefore, target.Troopers)
	}
	if i, j := strings.Index(out, "Attacking "), strings.Index(out, "Pushing..."); i > j {
		t.Errorf("the stages are out of order: Attacking at %d, Pushing at %d", i, j)
	}
}

func TestPartialLossNeverExceedsTheWhole(t *testing.T) {
	u := game.UnitLoss{Troopers: 1000, Jets: 7, Turrets: 2, Tanks: 1, Bombers: 0}
	first, second := partialLoss(u, 1), partialLoss(u, 2)
	if first.Troopers != 333 || second.Troopers != 666 {
		t.Errorf("thirds of 1000 = %d then %d; want 333 then 666", first.Troopers, second.Troopers)
	}
	for _, s := range []game.UnitLoss{first, second} {
		if s.Jets > u.Jets || s.Turrets > u.Turrets || s.Tanks > u.Tanks || s.Bombers > u.Bombers {
			t.Errorf("a stage overstates the losses: %+v against %+v", s, u)
		}
	}
	if whole := partialLoss(u, battleStages); whole != u {
		t.Errorf("the last stage is not the whole: %+v want %+v", whole, u)
	}
}
