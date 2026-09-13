package store

import (
	"os"
	"path/filepath"
	"testing"
)

// A save that predates the pirate factions must load the same way every time.
// json.Unmarshal leaves a field alone when the document has no key for it, so
// the fresh NewWorld that Load unmarshals into was handing such a save its own
// random starting hoard — a different one on every load, and never saved by a
// read-only transaction, so the Attack Pirates screen showed a new strength
// each time it was drawn.
func TestAPreFactionsSaveGetsTheSamePiratesEveryLoad(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	if !cfg.Pirates {
		t.Skip("pirates are off in the default config")
	}
	fixture, err := os.ReadFile("testdata/world-v0.0.3.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "world.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Pirates) == 0 {
		t.Fatal("the factions were not backfilled at all")
	}
	for i, p := range first.Pirates {
		if p.Gold != 0 || p.Land != 0 || p.LootTroopers != 0 {
			t.Fatalf("faction %d came back with a hoard (%+v); a game already under way gets names only", i, p)
		}
	}
	second, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := range first.Pirates {
		if first.Pirates[i] != second.Pirates[i] {
			t.Errorf("faction %d differs between two loads of the same save:\n first: %+v\nsecond: %+v",
				i, first.Pirates[i], second.Pirates[i])
		}
	}
}
