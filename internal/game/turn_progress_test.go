package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TurnProgress must survive a JSON round-trip: the whole point of the resume
// feature is that a mid-turn boot's progress persists across a process restart
// (a fresh Load unmarshals world.json). A stray `json:"-"` would silently break
// resume, so guard the serialization here.
func TestTurnProgressPersistsJSON(t *testing.T) {
	e := &Empire{}
	e.TurnProgress = TurnProgress{
		IncomeCollected: true, MaintPaid: true, AttackDone: true,
		CovertOpsUsed: map[CovertOp]bool{OpBribery: true},
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Empire
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back.TurnProgress, e.TurnProgress) {
		t.Errorf("TurnProgress did not survive JSON round-trip: got %+v, want %+v", back.TurnProgress, e.TurnProgress)
	}
}

// A committed turn starts the next one clean: PlayTurn (the turn-commit point)
// clears TurnProgress alongside the other per-turn state it already resets.
func TestPlayTurnClearsTurnProgress(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "Testland")
	e.TurnProgress = TurnProgress{IncomeCollected: true, MaintPaid: true, Fed: true, SpendingDone: true}

	w.PlayTurn(e, "2026-07-03")

	if !reflect.DeepEqual(e.TurnProgress, TurnProgress{}) {
		t.Errorf("PlayTurn should clear TurnProgress, got %+v", e.TurnProgress)
	}
}

// A new day abandons an uncommitted turn: DailyMaintenance resets TurnsLeft and
// AttacksToday per empire on rollover, and must clear TurnProgress too. Without
// it, a turn booted mid-way on day N leaves stale flags that would make the
// player's FIRST turn of day N+1 skip income collection.
func TestDailyMaintenanceClearsTurnProgress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameStartDate = "" // started immediately
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("t", "Testland")
	w.LastMaintDate = "2026-07-03"
	e.TurnProgress = TurnProgress{IncomeCollected: true}

	w.DailyMaintenance("2026-07-04")

	if !reflect.DeepEqual(e.TurnProgress, TurnProgress{}) {
		t.Errorf("new day should clear TurnProgress, got %+v", e.TurnProgress)
	}
}
