package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestLoadConfig_MissingFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := game.DefaultConfig()
	want.DataDir = dir
	if cfg != want {
		t.Errorf("LoadConfig(%q) = %+v, want %+v", dir, cfg, want)
	}
}

func TestSaveConfig_LoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	cfg := game.DefaultConfig()
	cfg.DataDir = dir
	cfg.TurnsPerDay = 15
	cfg.AICount = 3
	cfg.GameLength = 30

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got != cfg {
		t.Errorf("round trip = %+v, want %+v", got, cfg)
	}
	if got.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", got.DataDir, dir)
	}
}

func TestLoadConfig_PartialFile_KeepsDefaultsForMissingFields(t *testing.T) {
	dir := t.TempDir()

	partial, err := json.Marshal(map[string]int{"TurnsPerDay": 20})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), partial, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := game.DefaultConfig()
	want.DataDir = dir
	want.TurnsPerDay = 20

	if got != want {
		t.Errorf("LoadConfig(partial) = %+v, want %+v", got, want)
	}
}
