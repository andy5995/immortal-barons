package game

import (
	"testing"
	"time"
)

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

func TestDailyMaintenanceHandlesMalformedDate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-0" // malformed, lexicographically < today
	done := make(chan struct{})
	go func() { w.DailyMaintenance("2026-07-03"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DailyMaintenance hung on a malformed LastMaintDate")
	}
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("LastMaintDate = %q, want snapped to today", w.LastMaintDate)
	}
}

func TestHQAdvancesEachTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = HQCost
	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	if e.HQ != 5 {
		t.Fatalf("HQ after start: want 5, got %d", e.HQ)
	}

	want := []int{10, 15, 20}
	for _, w2 := range want {
		w.PlayTurn(e, "2026-07-03")
		if e.HQ != w2 {
			t.Errorf("HQ after turn: want %d, got %d", w2, e.HQ)
		}
	}

	e.HQ = 100
	w.PlayTurn(e, "2026-07-03")
	if e.HQ != 100 {
		t.Errorf("HQ should cap at 100, got %d", e.HQ)
	}
}

func TestFoodSpoilageAboveBuffer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.People = 0
	e.Troopers = 0
	e.Jets = 0
	e.Tanks = 0
	e.Regions = RegionMix{}
	e.Land = 0
	e.Food = 100000

	w.PlayTurn(e, "2026-07-03")

	if e.LastSpoiled <= 0 {
		t.Errorf("expect food spoilage above buffer, got LastSpoiled=%d", e.LastSpoiled)
	}
}

func TestFoodNoSpoilageBelowBuffer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.People = 100
	e.Troopers = 10
	e.Jets = 0
	e.Tanks = 0
	e.Regions = RegionMix{}
	e.Land = 0
	e.Food = 150 // consumption (110) leaves 40, well below the buffer (220)

	w.PlayTurn(e, "2026-07-03")

	if e.LastSpoiled != 0 {
		t.Errorf("expect no spoilage below buffer, got LastSpoiled=%d", e.LastSpoiled)
	}
}
