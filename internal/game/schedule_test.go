package game

import "testing"

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
