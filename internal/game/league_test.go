package game

import "testing"

func TestDailyMaintenanceEndsGameAtGameLength(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameLength = 3
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-06-27" // 3 days before target
	w.DailyMaintenance("2026-06-30")

	if w.LastMaster == "" {
		t.Fatal("expected LastMaster to be set after game ended")
	}
	found := false
	for _, e := range w.Empires {
		if e.Name == w.LastMaster {
			found = true
		}
	}
	if !found {
		t.Errorf("LastMaster %q does not match any empire", w.LastMaster)
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
	w.DailyMaintenance("2026-06-30")

	if w.LastMaster != "" {
		t.Errorf("expected LastMaster to remain unset with GameLength=0, got %q", w.LastMaster)
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
	if w.Active != nil {
		t.Errorf("expected Active reset to nil, got %v", w.Active)
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
