package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	return c
}

func TestLoadMissingReturnsFreshWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	cfg.AICount = 3
	w, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Empires) != 3 {
		t.Errorf("fresh world should have 3 AI, got %d", len(w.Empires))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Gold = 4242
	e.Events = []string{"hello"}
	w.GameDay = 7
	w.LastMaintDate = "2026-07-03"

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.GameDay != 7 || got.LastMaintDate != "2026-07-03" {
		t.Errorf("world scalars not preserved: day=%d date=%q", got.GameDay, got.LastMaintDate)
	}
	ge := got.FindByOwner("khan")
	if ge == nil || ge.Gold != 4242 || len(ge.Events) != 1 {
		t.Errorf("empire not preserved: %+v", ge)
	}
}

func TestLoadMigratesPreRegionTypesSave(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Regions = game.RegionMix{} // simulate a save written before region types

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ge := got.FindByOwner("khan")
	if ge == nil {
		t.Fatal("empire not found after load")
	}
	if ge.Regions.Total() != ge.Land {
		t.Errorf("Regions.Total()=%d, Land=%d: migration did not run", ge.Regions.Total(), ge.Land)
	}
	if ge.Land != 100 {
		t.Errorf("Land=%d, want 100", ge.Land)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	// no leftover temp file
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "world.json.tmp")); !os.IsNotExist(err) {
		t.Error("temp file should not remain after save")
	}
}
