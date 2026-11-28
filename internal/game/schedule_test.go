package game

import (
	"fmt"
	"testing"
)

func TestGameStartedAndJoinOpen(t *testing.T) {
	var c Config // empty dates
	if !c.GameStarted("2026-07-05") || !c.JoinOpen("2026-07-05") {
		t.Fatal("empty dates should mean started and open")
	}
	c.GameStartDate = "2026-07-10"
	if c.GameStarted("2026-07-09") {
		t.Error("should not be started the day before the start date")
	}
	if !c.GameStarted("2026-07-10") {
		t.Error("should be started on the start date")
	}
	c.JoinDate = "2026-07-12"
	if !c.JoinOpen("2026-07-12") {
		t.Error("joining should be open through the cutoff date")
	}
	if c.JoinOpen("2026-07-13") {
		t.Error("joining should be closed after the cutoff date")
	}
}

func TestMaintenanceFrozenBeforeStart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameStartDate = "2026-07-10"
	w := NewWorldSeed(cfg, 1)

	w.DailyMaintenance("2026-07-05") // before start: no advance
	w.DailyMaintenance("2026-07-09")
	if w.GameDay != 0 {
		t.Fatalf("game advanced before start date: GameDay=%d", w.GameDay)
	}
	// From the start date the game runs normally (one day per calendar day).
	w.DailyMaintenance("2026-07-10")
	w.DailyMaintenance("2026-07-11")
	if w.GameDay != 2 {
		t.Fatalf("game should advance after the start date, GameDay=%d want 2", w.GameDay)
	}
}

func TestDefaultIdleConfig(t *testing.T) {
	c := DefaultConfig()
	if c.IdleTimeoutSecs != 300 {
		t.Errorf("IdleTimeoutSecs = %d, want 300", c.IdleTimeoutSecs)
	}
	if c.MaxIdleWarnings != 3 {
		t.Errorf("MaxIdleWarnings = %d, want 3", c.MaxIdleWarnings)
	}
}

// A baron who founded a realm before the scheduled start could not play it, and
// the three-day grace on a young game keeps it through the opening days. The
// game days then count from the launch, not from the signup.
func TestRealmFoundedBeforeStartSurvivesTheOpeningDays(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameStartDate = "2026-07-10"
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	w.DailyMaintenance("2026-07-08") // the board is up, the game is not
	w.AddHuman("early", "Earlyland")
	w.DailyMaintenance("2026-07-09") // still before the start
	if w.GameDay > 1 {
		t.Fatalf("the game advanced before its start, GameDay=%d", w.GameDay)
	}

	w.DailyMaintenance("2026-07-10") // the launch day
	e := w.FindByOwner("early")
	if e == nil {
		t.Fatal("a realm founded before the start was erased on the day the game launched")
	}
	if e.TurnsLeft != cfg.TurnsPerDay {
		t.Errorf("TurnsLeft = %d on launch day, want %d", e.TurnsLeft, cfg.TurnsPerDay)
	}

	for d := 11; w.GameDay <= UnplayedRealmGraceDays; d++ {
		w.DailyMaintenance(fmt.Sprintf("2026-07-%02d", d))
		if w.GameDay <= UnplayedRealmGraceDays && w.FindByOwner("early") == nil {
			t.Fatalf("realm erased on game day %d, inside the grace", w.GameDay)
		}
	}
	w.DailyMaintenance("2026-07-20")
	if w.FindByOwner("early") != nil {
		t.Error("an unplayed realm should be collected once the game is past the grace")
	}
}
