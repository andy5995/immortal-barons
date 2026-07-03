package game

import "testing"

func TestPlayTurnAffectsOnlyActingEmpire(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	other := w.Empires[1]
	otherGold := other.Gold
	me := w.AddHuman("me", "Mine")
	turns := me.TurnsLeft
	prot := me.Protection

	w.PlayTurn(me, "2026-07-03")

	if me.TurnsLeft != turns-1 {
		t.Errorf("TurnsLeft: want %d, got %d", turns-1, me.TurnsLeft)
	}
	if me.Protection != prot-1 {
		t.Errorf("Protection: want %d, got %d", prot-1, me.Protection)
	}
	if me.LastPlayed != "2026-07-03" {
		t.Errorf("LastPlayed: got %q", me.LastPlayed)
	}
	if other.Gold != otherGold {
		t.Error("PlayTurn must not touch other empires")
	}
	if me.Gold <= 10000 {
		t.Errorf("acting empire should collect income, got %d", me.Gold)
	}
}

func TestDailyMaintenanceInitialisesDate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.DailyMaintenance("2026-07-03")
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("first maintenance should just set the date, got %q", w.LastMaintDate)
	}
	if w.GameDay != 0 {
		t.Errorf("first maintenance should not advance the day, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceCatchesUpAndRefills(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-01"
	me := w.AddHuman("me", "Mine")
	me.TurnsLeft = 0
	w.DailyMaintenance("2026-07-03") // two days missed
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("should catch up to today, got %q", w.LastMaintDate)
	}
	if w.GameDay != 2 {
		t.Errorf("two days should advance GameDay to 2, got %d", w.GameDay)
	}
	if me.TurnsLeft != cfg.TurnsPerDay {
		t.Errorf("turns should be refilled to %d, got %d", cfg.TurnsPerDay, me.TurnsLeft)
	}
}

func TestDailyMaintenanceIdempotent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.GameDay = 5
	w.DailyMaintenance("2026-07-03")
	if w.GameDay != 5 {
		t.Errorf("same-day maintenance should be a no-op, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceCullsDead(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-02"
	dead := w.AddHuman("gone", "Gone")
	dead.Land = 0
	w.DailyMaintenance("2026-07-03")
	if w.FindByOwner("gone").Alive {
		t.Error("empire with 0 land should be marked dead")
	}
}
