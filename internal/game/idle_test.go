package game

import (
	"fmt"
	"testing"
)

// A caller who creates a realm and never takes a turn is erased at the next
// maintenance, and may build a fresh realm normally afterwards.
func TestUnplayedRealmErasedAtMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-01"
	e := w.AddHuman("caller", "Neverplayed")
	if e.CreatedDay != w.GameDay {
		t.Fatalf("setup: CreatedDay %d should be the current game day %d", e.CreatedDay, w.GameDay)
	}

	w.DailyMaintenance("2026-07-02")

	if w.FindByOwner("caller") != nil {
		t.Error("a realm that never played a turn should be erased at maintenance")
	}
	// The caller may start over with no penalty.
	again := w.AddHuman("caller", "Secondtry")
	if again == nil || w.FindByOwner("caller") == nil {
		t.Error("the caller should be able to create a new realm after being erased")
	}
}

// The realm is safe on the day it was created: its owner is entitled to the day
// they joined, and on a multi-node board another caller's login can roll the day
// over while they are still at the menu.
func TestUnplayedRealmSurvivesTheDayItWasCreated(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-01"
	e := w.AddHuman("caller", "Fresh")
	e.CreatedDay = w.GameDay + 1 // as if created after this maintenance advanced

	w.DailyMaintenance("2026-07-02")

	if w.FindByOwner("caller") == nil {
		t.Error("a realm created today must survive to its first turn")
	}
}

// A realm that HAS played is left alone by the never-played sweep.
func TestPlayedRealmIsNotErased(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-01"
	e := w.AddHuman("caller", "Active")
	e.LastPlayed = "2026-07-01"

	w.DailyMaintenance("2026-07-02")

	if w.FindByOwner("caller") == nil {
		t.Error("a realm that has been played should not be erased")
	}
}

// An abandoned realm is removed once it has gone unplayed for the configured
// number of days (#83).
func TestIdleRealmRemovedAfterConfiguredDays(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IdleDaysRemove != 10 {
		t.Fatalf("default idle removal should be 10 days, got %d", cfg.IdleDaysRemove)
	}
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-20"
	stale := w.AddHuman("gone", "Abandoned")
	stale.LastPlayed = "2026-07-01" // 20 days ago
	recent := w.AddHuman("here", "Active")
	recent.LastPlayed = "2026-07-19"

	w.DailyMaintenance("2026-07-21")

	if w.FindByOwner("gone") != nil {
		t.Error("a realm unplayed past the limit should be removed")
	}
	if w.FindByOwner("here") == nil {
		t.Error("a recently played realm must be kept")
	}
}

// A realm exactly at the boundary is kept; one day past it goes.
func TestIdleRemovalBoundary(t *testing.T) {
	run := func(lastPlayed string) bool {
		cfg := DefaultConfig()
		cfg.IdleDaysRemove = 10
		w := NewWorldSeed(cfg, 1)
		w.LastMaintDate = "2026-07-20"
		e := w.AddHuman("h", "Realm")
		e.LastPlayed = lastPlayed
		w.DailyMaintenance("2026-07-21")
		return w.FindByOwner("h") != nil
	}
	// Maintenance advances the clock to 2026-07-21, so the cutoff is 2026-07-11.
	if !run("2026-07-11") {
		t.Error("a realm played exactly at the limit should be kept")
	}
	if run("2026-07-10") {
		t.Error("a realm played one day past the limit should be removed")
	}
}

// 0 means never remove, matching how GameLength treats 0 as endless — and it
// governs the never-played sweep too, so one switch turns reaping off.
func TestIdleRemovalDisabledByZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IdleDaysRemove = 0
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-20"
	stale := w.AddHuman("gone", "Abandoned")
	stale.LastPlayed = "2020-01-01"
	w.AddHuman("never", "Neverplayed")

	w.DailyMaintenance("2026-07-21")

	if w.FindByOwner("gone") == nil {
		t.Error("0 should mean never remove an idle realm")
	}
	if w.FindByOwner("never") == nil {
		t.Error("0 should also keep a never-played realm")
	}
}

// AI barons are never reaped: they are played every day by definition.
func TestAIBaronsAreNeverReaped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 3
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-20"
	for _, e := range w.AIEmpires() {
		e.LastPlayed = "" // as if never played
	}
	before := len(w.AIEmpires())

	w.DailyMaintenance(fmt.Sprintf("2026-07-%02d", 21))

	if got := len(w.AIEmpires()); got != before {
		t.Errorf("AI barons should never be reaped: %d -> %d", before, got)
	}
}

// The world records the day the game actually began, which is what the opening
// menu reports. Config.GameStartDate cannot answer that on its own: it is the
// date a sysop SCHEDULED the game for, and is empty on the common
// start-immediately setup.
func TestStartedDateRecordedAtFirstMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if w.StartedDate != "" {
		t.Fatalf("a fresh world has not started yet, got %q", w.StartedDate)
	}

	w.DailyMaintenance("2026-07-30")
	if w.StartedDate != "2026-07-30" {
		t.Errorf("StartedDate: want 2026-07-30, got %q", w.StartedDate)
	}

	// It is the day the game BEGAN, so later days must not move it.
	w.DailyMaintenance("2026-07-31")
	if w.StartedDate != "2026-07-30" {
		t.Errorf("StartedDate should not move with the clock, got %q", w.StartedDate)
	}
}

// A game scheduled for a future date has not begun, so nothing is recorded
// until the start date arrives.
func TestStartedDateWaitsForScheduledStart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameStartDate = "2026-08-10"
	w := NewWorldSeed(cfg, 1)

	w.DailyMaintenance("2026-08-01")
	if w.StartedDate != "" {
		t.Errorf("the game has not started yet, got %q", w.StartedDate)
	}

	w.DailyMaintenance("2026-08-10")
	if w.StartedDate != "2026-08-10" {
		t.Errorf("StartedDate should be the scheduled start, got %q", w.StartedDate)
	}
}
