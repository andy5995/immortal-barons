package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

func writeTestMod(t *testing.T, dataDir, name, values string) {
	t.Helper()
	dir := filepath.Join(dataDir, mod.Dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		mod.ManifestFile: "name = \"" + name + "\"\nversion = \"1.0\"\n",
		mod.ValuesFile:   values,
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A game reset with a mod plays it after a save and a load, in another
// process's world and in the reloading store a door session uses.
func TestModsSurviveSaveAndLoad(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Mods = []string{"cheap-tanks"}
	writeTestMod(t, cfg.DataDir, "cheap-tanks", "[units.tank]\ncost = 250\n")

	w := game.NewWorldSeed(cfg, 1)
	if err := ApplyResetMods(w, cfg); err != nil {
		t.Fatal(err)
	}
	w.Reset()
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Rules == nil || loaded.Rules.Units[game.Tank].Cost != 250 {
		t.Fatalf("a loaded world must play the mod it was reset with, got %+v", loaded.Rules)
	}
	loaded.SetStore(NewFileStore(loaded, cfg))
	loaded.With(func() {})
	if loaded.Rules == nil || loaded.Rules.Units[game.Tank].Cost != 250 {
		t.Fatal("a transaction's reload must keep the mod's rules")
	}

	// config.json names the NEXT reset's mods; changing it does not reach the
	// game in progress.
	cfg.Mods = nil
	loaded, err = Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Rules == nil || loaded.Rules.Units[game.Tank].Cost != 250 {
		t.Fatal("removing a mod from config.json must not change the game in progress")
	}

	// An edit mid-game is warned about and does not reach the game in progress.
	writeTestMod(t, cfg.DataDir, "cheap-tanks", "[units.tank]\ncost = 300\n")
	loaded, err = Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Rules.Units[game.Tank].Cost; got != 250 {
		t.Errorf("tank cost %d after the mod was edited mid-game, want the reset's 250", got)
	}
}

func TestLeagueBoardRefusesMods(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.IBBS = true
	cfg.Mods = []string{"m"}
	writeTestMod(t, cfg.DataDir, "m", "")
	if err := ApplyResetMods(game.NewWorldSeed(cfg, 1), cfg); !errors.Is(err, ErrModsInLeague) {
		t.Errorf("err = %v, want ErrModsInLeague", err)
	}

	// A world already holding mods on a board now set to IBBS does not load,
	// and says so in the form a reset knows to get past.
	cfg.IBBS = false
	w := game.NewWorldSeed(cfg, 1)
	if err := ApplyResetMods(w, cfg); err != nil {
		t.Fatal(err)
	}
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.IBBS = true
	if _, err := Load(cfg); !errors.Is(err, ErrWorldMods) || !errors.Is(err, ErrModsInLeague) {
		t.Errorf("err = %v, want ErrWorldMods wrapping ErrModsInLeague", err)
	}
}
