package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Land captured on another planet reaches the same type picker a Regular Attack
// opens, at the start of the owner's next turn — the second half of #107. The
// strike resolved on a board they were not logged in to, so the regions sat on
// Empire.PendingRegions until there was somebody to ask.
func TestSpoilsStageOffersThePicker(t *testing.T) {
	w := newWorld()
	var beforeLand, beforeRiver, beforeMtn int
	w.With(func() {
		p := w.Player()
		p.PendingRegions = 10
		beforeLand, beforeRiver, beforeMtn = p.Land, p.Regions.River, p.Regions.Mountain
	})
	f := &fakeSession{keys: []rune("R6\rM4\r")} // River 6, Mountain 4
	spoilsStage(f, w)

	// The screen this test is for, not just "some output": the spoils headline is
	// the picker's own and appears nowhere else.
	if out := f.out.String(); !strings.Contains(out, "brought home") {
		t.Fatalf("the spoils picker never ran; output was: %q", out)
	}
	p := w.Player()
	if p.Regions.River != beforeRiver+6 || p.Regions.Mountain != beforeMtn+4 {
		t.Errorf("split wrong: River %d (was %d), Mountain %d (was %d)",
			p.Regions.River, beforeRiver, p.Regions.Mountain, beforeMtn)
	}
	if p.Land != beforeLand+10 {
		t.Errorf("Land = %d, want %d", p.Land, beforeLand+10)
	}
	if p.PendingRegions != 0 {
		t.Errorf("PendingRegions = %d after the picker, want 0 — the land would be handed over twice", p.PendingRegions)
	}
}

// With nothing parked the stage is silent: a baron who sent no strike must not
// meet a picker at the top of every turn.
func TestSpoilsStageSilentWithNothingParked(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	spoilsStage(f, w)
	if out := f.out.String(); out != "" {
		t.Errorf("spoils stage printed %q with no land waiting", out)
	}
}

// Quitting the picker early still hands the land over (as Coastal) and still
// clears the parked count, so it cannot be claimed a second time next turn.
func TestSpoilsStageQuitStillClearsPending(t *testing.T) {
	w := newWorld()
	var beforeLand int
	w.With(func() {
		p := w.Player()
		p.PendingRegions = 10
		beforeLand = p.Land
	})
	f := &fakeSession{keys: []rune("0")}
	spoilsStage(f, w)
	p := w.Player()
	if p.Land != beforeLand+10 {
		t.Errorf("Land = %d, want %d — captured land was lost", p.Land, beforeLand+10)
	}
	if p.PendingRegions != 0 {
		t.Errorf("PendingRegions = %d, want 0", p.PendingRegions)
	}
}

// The group-attack picker asks for the delay in hours and holds the answer to
// BRE's window, so a baron cannot type 3 and get a three-hour strike (#124).
func TestCreateGroupAttackAsksHours(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.BoardID = "Alpha"
		w.World.ImportBoard(game.RemoteBoard{
			BoardID: "Mars",
			Scores:  []game.RemoteScore{{Empire: "Redlands"}},
		})
		p := w.Player()
		p.Protection = 0
		p.Troopers = 10_000
	})
	// planet 1, baron 2 (Redlands, past "the whole planet"), 3 hours, 500 troopers,
	// then zero for the other three types.
	f := &fakeSession{keys: []rune("1\r2\r3\r500\r\r\r\r")}
	createGroupAttack(f, w)

	out := f.out.String()
	if !strings.Contains(out, "Wait how many Hours") {
		t.Fatalf("the hours prompt never appeared; output was: %q", out)
	}
	var wait float64
	var count int
	w.With(func() {
		count = len(w.World.GroupAttacks)
		if count == 1 {
			wait = time.Until(w.World.GroupAttacks[0].DepartAt).Hours()
		}
	})
	if count != 1 {
		t.Fatalf("group attacks = %d, want 1", count)
	}
	if wait < float64(game.GroupAttackHoursMin)-1 {
		t.Errorf("a 3-hour delay was honoured (%.1fh); the floor is %d", wait, game.GroupAttackHoursMin)
	}
}
