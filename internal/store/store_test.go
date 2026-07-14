package store

import (
	"errors"
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

func TestLoadMissingReturnsErrNoWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	if _, err := Load(cfg); !errors.Is(err, ErrNoWorld) {
		t.Errorf("Load of a missing world: want ErrNoWorld, got %v", err)
	}
}

func TestNewGameSeedsAI(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	cfg.AICount = 3
	w := NewGame(cfg)
	if len(w.Empires) != 3 {
		t.Errorf("NewGame should seed %d AI, got %d", 3, len(w.Empires))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Gold = 4242
	e.Events = []string{"hello"}
	e.Investments = []game.Investment{{Amount: 1000, Return: 1150, MaturesDay: 5}}
	w.GameDay = 7
	w.LastMaintDate = "2026-07-03"
	w.InvestRate = 12

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
	if got.InvestRate != 12 {
		t.Errorf("InvestRate=%d, want 12", got.InvestRate)
	}
	ge := got.FindByOwner("khan")
	if ge == nil || ge.Gold != 4242 || len(ge.Events) != 1 {
		t.Errorf("empire not preserved: %+v", ge)
	}
	if ge != nil {
		if len(ge.Investments) != 1 {
			t.Fatalf("Investments not preserved: %+v", ge.Investments)
		}
		inv := ge.Investments[0]
		if inv.Amount != 1000 || inv.Return != 1150 || inv.MaturesDay != 5 {
			t.Errorf("Investment fields not preserved: %+v", inv)
		}
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
	if ge.Land != 15 {
		t.Errorf("Land=%d, want 15", ge.Land)
	}
}

func TestSupportMigration(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Support = 0 // simulate a save written before Support existed

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
	if ge.Support != 100 {
		t.Errorf("Support=%d, want migrated default 100", ge.Support)
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
