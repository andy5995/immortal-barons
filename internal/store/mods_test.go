package store

import (
	"bytes"
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
		mod.ManifestFile: "name = \"" + name + "\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n",
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
	writeTestMod(t, cfg.DataDir, "cheap-tanks", "[units.tank]\nbuild_cost = 250\n")

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
	writeTestMod(t, cfg.DataDir, "cheap-tanks", "[units.tank]\nbuild_cost = 300\n")
	loaded, err = Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Rules.Units[game.Tank].Cost; got != 250 {
		t.Errorf("tank cost %d after the mod was edited mid-game, want the reset's 250", got)
	}
}

// A league board does not choose mods for itself: the Coordinator's
// -league-reset carries them. A world that holds the league's mods loads and
// plays them with no mod folder on the board, and a copy edited in world.json
// changes the fingerprint its packets carry.
func TestLeagueBoardPlaysTheLeaguesMods(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.IBBS = true
	cfg.Mods = []string{"m"}
	writeTestMod(t, cfg.DataDir, "m", "[units.tank]\nbuild_cost = 250\n")
	if err := ApplyResetMods(game.NewWorldSeed(cfg, 1), cfg); !errors.Is(err, ErrModsInLeague) {
		t.Errorf("err = %v, want ErrModsInLeague", err)
	}

	// The world as a league reset leaves it: the mods recorded, no folder.
	r, refs, err := mod.Resolve(cfg.DataDir, cfg.Mods)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(cfg.DataDir, mod.Dir)); err != nil {
		t.Fatal(err)
	}
	cfg.Mods = nil
	w := game.NewWorldSeed(cfg, 1)
	w.Rules, w.Mods = r, refs
	fp := w.RulesetFingerprint()
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	out := captureModWarnings(t)
	got, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Rules == nil || got.Rules.Units[game.Tank].Cost != 250 || got.RulesetFingerprint() != fp {
		t.Errorf("loaded rules %v, fingerprint %q (want tank cost 250 and %q)", got.Rules, got.RulesetFingerprint(), fp)
	}
	if out.Len() != 0 {
		t.Errorf("a league board warned about its mod folder: %q", out.String())
	}

	got.Mods[0].Files[mod.ValuesFile] = "[units.tank]\nbuild_cost = 100\n"
	if err := Save(got, cfg); err != nil {
		t.Fatal(err)
	}
	edited, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if edited.RulesetFingerprint() == fp {
		t.Error("a copy edited in world.json kept the league's fingerprint")
	}
}

// captureModWarnings sends the mod package's warnings to a buffer for the
// length of the test.
func captureModWarnings(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	mod.SetWarnOutput(&buf)
	t.Cleanup(func() { mod.SetWarnOutput(os.Stderr) })
	return &buf
}
