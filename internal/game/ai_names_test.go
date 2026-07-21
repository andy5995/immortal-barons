package game

import (
	"strings"
	"testing"
)

// AI baron names come from a modifier x noun matrix, so a game can field many
// more than the old fixed list of 15 and each name is distinct. Seeding 30 AIs
// (twice the old cap) must yield 30 unique two-word names.
func TestAIBaronNamesFromMatrix(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 30
	w := NewWorldSeed(cfg, 1)

	ais := w.AIEmpires()
	if len(ais) != 30 {
		t.Fatalf("want 30 AI barons from the name matrix, got %d (old fixed list capped at 15)", len(ais))
	}
	seen := map[string]bool{}
	for _, e := range ais {
		key := strings.ToLower(e.Name)
		if seen[key] {
			t.Errorf("duplicate AI name %q", e.Name)
		}
		seen[key] = true
		if len(strings.Fields(e.Name)) != 2 {
			t.Errorf("AI name %q should be two words (modifier + noun)", e.Name)
		}
	}
}

// Two worlds seeded with different RNG seeds should not produce the same AI
// name lineup — the matrix + RNG gives game-to-game variety.
func TestAIBaronNamesVaryBySeed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 10
	names := func(seed int64) []string {
		w := NewWorldSeed(cfg, seed)
		var out []string
		for _, e := range w.AIEmpires() {
			out = append(out, e.Name)
		}
		return out
	}
	a, b := names(1), names(2)
	if strings.Join(a, ",") == strings.Join(b, ",") {
		t.Errorf("two seeds produced identical AI name lineups:\n%v", a)
	}
}
