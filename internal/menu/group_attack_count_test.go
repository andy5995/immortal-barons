package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Join Group Attack carries the number of parties still forming, and nothing
// when there are none.
func TestJoinGroupAttackShowsFormingCount(t *testing.T) {
	w := newWorld()
	w.With(func() { w.World.Config.IBBS = true })
	menus := BuildMenus()

	f := &fakeSession{}
	draw(f, w, menus.InterPlanetary)
	if out := stripANSI(f.out.String()); !strings.Contains(out, "Join Group Attack") || strings.Contains(out, "Join Group Attack (") {
		t.Fatalf("a count with no parties forming:\n%s", out)
	}

	w.With(func() {
		later := time.Now().Add(time.Hour)
		w.GroupAttacks = append(w.GroupAttacks,
			game.GroupAttack{ID: 1, DepartAt: later},
			game.GroupAttack{ID: 2, DepartAt: later},
			game.GroupAttack{ID: 3, DepartAt: time.Now().Add(-time.Hour)}) // already left
	})
	f = &fakeSession{}
	draw(f, w, menus.InterPlanetary)
	if out := stripANSI(f.out.String()); !strings.Contains(out, "Join Group Attack (2)") {
		t.Errorf("want the two forming parties counted:\n%s", out)
	}
}
