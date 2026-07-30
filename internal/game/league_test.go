package game

import (
	"fmt"
	"testing"
)

func TestDailyMaintenanceEndsGameAtGameLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameLength = 3
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-06-27" // 3 days before target
	// Capture the ended game's empire names BEFORE maintenance resets the world:
	// LastMaster records the just-ended game's master, whose (now varied) name need
	// not match a freshly-seeded post-reset empire.
	endedNames := map[string]bool{}
	for _, e := range w.Empires {
		endedNames[e.Name] = true
	}
	// Maintenance advances one day per real day, so drive each day in turn.
	for _, d := range []string{"2026-06-28", "2026-06-29", "2026-06-30"} {
		w.DailyMaintenance(d)
	}

	if w.LastMaster == "" {
		t.Fatal("expected LastMaster to be set after game ended")
	}
	if !endedNames[w.LastMaster] {
		t.Errorf("LastMaster %q was not one of the ended game's empires", w.LastMaster)
	}
	if w.GameDay >= cfg.GameLength {
		t.Errorf("expected GameDay to have reset below GameLength, got %d", w.GameDay)
	}
	if len(w.Empires) != cfg.AICount {
		t.Errorf("expected %d AI empires after reset, got %d", cfg.AICount, len(w.Empires))
	}
}

func TestDailyMaintenanceEndlessByDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-06-20"
	// One game day per real day: ten days of play takes ten daily runs.
	for d := 21; d <= 30; d++ {
		w.DailyMaintenance(fmt.Sprintf("2026-06-%02d", d))
	}

	// GameLength==0 means endless play: no league end, so endGame never
	// runs and LastMaster stays unset. The daily standing goes to
	// CurrentMaster instead (see postMasterNews).
	if w.LastMaster != "" {
		t.Errorf("expected LastMaster to stay empty in an endless game, got %q", w.LastMaster)
	}
	found := false
	for _, e := range w.Empires {
		if e.Name == w.CurrentMaster {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CurrentMaster to name a living empire, got %q", w.CurrentMaster)
	}
	if w.GameDay != 10 {
		t.Errorf("expected GameDay to climb to 10 without resetting, got %d", w.GameDay)
	}
}

func TestEndGame(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	winner := w.Empires[0]
	winner.Land = 1_000_000

	w.endGame()

	if w.LastMaster != winner.Name {
		t.Errorf("expected LastMaster %q, got %q", winner.Name, w.LastMaster)
	}
	if w.GameDay != 0 {
		t.Errorf("expected GameDay reset to 0, got %d", w.GameDay)
	}
	if len(w.Empires) != cfg.AICount {
		t.Errorf("expected %d AI empires after reset, got %d", cfg.AICount, len(w.Empires))
	}
	if w.Alliances != nil {
		t.Errorf("expected Alliances reset to nil, got %v", w.Alliances)
	}
}

func TestEndGameCrownsSoleSurvivorWithNegativeNetWorth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	survivor := w.AddHuman("s", "Survivor")
	survivor.Land = 1
	survivor.Debt = 1_000_000_000

	w.endGame()

	if w.LastMaster != survivor.Name {
		t.Errorf("expected LastMaster %q, got %q", survivor.Name, w.LastMaster)
	}
}
