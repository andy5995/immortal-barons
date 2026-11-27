package store

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ReloadMods puts a mod's current values into a game in progress and reports
// what changed; a league board refuses it.
func TestReloadModsTakesTheFolderAsItStands(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "mods", "cheap")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(modDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mod.toml", "name = \"cheap\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n")
	write("values.toml", "[units.tank]\nbuild_cost = 250\n")

	cfg := game.DefaultConfig()
	cfg.DataDir, cfg.Mods = dir, []string{"cheap"}
	w := game.NewWorldSeed(cfg, 1)
	changes, err := ReloadMods(w, cfg)
	if err != nil || !slices.Contains(changes, "tank.build_cost: 500 -> 250") || len(w.Mods) != 1 {
		t.Fatalf("first reload: changes %q, err %v, mods %d", changes, err, len(w.Mods))
	}
	write("values.toml", "[units.tank]\nbuild_cost = 300\n")
	if changes, err = ReloadMods(w, cfg); err != nil || !slices.Equal(changes, []string{"tank.build_cost: 250 -> 300"}) {
		t.Fatalf("second reload: changes %q, err %v", changes, err)
	}

	cfg.IBBS = true
	if _, err := ReloadMods(w, cfg); err != ErrModsInLeague {
		t.Errorf("league board: err %v, want ErrModsInLeague", err)
	}
}

// A reload changes the names the factions already carry, not only the rules.
func TestReloadModsRenamesTheFactions(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "mods", "names")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"mod.toml":    "name = \"names\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n",
		"values.toml": "[pirates]\nnames = [\"A\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(modDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := game.DefaultConfig()
	cfg.DataDir, cfg.Mods, cfg.Pirates = dir, []string{"names"}, true
	w := game.NewWorldSeed(cfg, 1)
	if w.Pirates[0].Name != "Humans" {
		t.Fatalf("a world with no mods names slot 0 %q", w.Pirates[0].Name)
	}
	if _, err := ReloadMods(w, cfg); err != nil {
		t.Fatal(err)
	}
	if w.Pirates[0].Name != "A" || w.Pirates[8].Name != "I" {
		t.Errorf("after the reload the factions are %q ... %q", w.Pirates[0].Name, w.Pirates[8].Name)
	}
}
