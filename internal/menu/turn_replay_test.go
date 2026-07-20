package menu

import (
	"io"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A stage that completed this turn is skipped when the turn is replayed after a
// boot — the player is not walked back through the Attack/Spending menu they
// already exited (#10).
func TestRunStageOnceSkipsCompleted(t *testing.T) {
	w := newWorld()
	calls := 0
	fn := func() error { calls++; return nil }
	get := func(tp game.TurnProgress) bool { return tp.SpendingDone }
	set := func(tp *game.TurnProgress) { tp.SpendingDone = true }

	runStageOnce(w, get, set, fn)
	if calls != 1 {
		t.Fatalf("stage should run on first pass, calls=%d", calls)
	}
	if !w.Player().TurnProgress.SpendingDone {
		t.Fatal("a completed stage should be marked done")
	}

	runStageOnce(w, get, set, fn) // replay
	if calls != 1 {
		t.Errorf("a completed stage should be skipped on replay, calls=%d", calls)
	}
}

// A stage interrupted before it finishes (fn errors, or a boot unwinds it) stays
// unmarked, so the replay re-runs it — the player resumes where the boot hit.
func TestRunStageOnceLeavesUnmarkedOnError(t *testing.T) {
	w := newWorld()
	fn := func() error { return io.EOF }
	get := func(tp game.TurnProgress) bool { return tp.AttackDone }
	set := func(tp *game.TurnProgress) { tp.AttackDone = true }

	if err := runStageOnce(w, get, set, fn); err != io.EOF {
		t.Fatalf("runStageOnce should propagate the stage error, got %v", err)
	}
	if w.Player().TurnProgress.AttackDone {
		t.Error("an interrupted stage must stay unmarked so the replay re-runs it")
	}
}

// A turn replayed after an idle-boot must not collect income twice. collectTurnIncome
// credits the turn's gold once and marks IncomeCollected; a second call (the replay)
// is a no-op.
func TestCollectTurnIncomeOnceOnReplay(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 0

	collectTurnIncome(w)
	if !p.TurnProgress.IncomeCollected {
		t.Fatal("collectTurnIncome should mark IncomeCollected")
	}
	after := p.Gold
	if after == 0 {
		t.Fatal("collectTurnIncome should credit some gold for a populated realm")
	}

	collectTurnIncome(w) // replay
	if p.Gold != after {
		t.Errorf("replay double-collected income: gold %d -> %d", after, p.Gold)
	}
}

// Maintenance must not be charged twice. After paymentStage pays (auto-pay), a
// replay (MaintPaid already set) leaves gold untouched.
func TestPaymentStageNotRechargedOnReplay(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 5_000_000
	p.Troopers, p.Land = 700, 100
	w.AutoPayMaint = true

	paymentStage(&fakeSession{}, w, BuildMenus().Bank)
	if !p.TurnProgress.MaintPaid {
		t.Fatal("paymentStage should mark MaintPaid after paying")
	}
	after := p.Gold
	if after == 5_000_000 {
		t.Fatal("paymentStage should have charged maintenance on the first pass")
	}

	paymentStage(&fakeSession{}, w, BuildMenus().Bank) // replay
	if p.Gold != after {
		t.Errorf("replay re-charged maintenance: gold %d -> %d", after, p.Gold)
	}
}
