package mod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// exampleDir is the example mod that ships in docs/examples/mods.
const exampleDir = "../../docs/examples/mods/example"

// exampleRules loads the shipped example through Resolve, as a reset does,
// with the given option choices.
func exampleRules(t *testing.T, options map[string]int64) *game.Rules {
	t.Helper()
	data := t.TempDir()
	if err := os.CopyFS(filepath.Join(data, Dir, "example"), os.DirFS(exampleDir)); err != nil {
		t.Fatal(err)
	}
	r, refs, err := Resolve(data, []string{"example"}, map[string]map[string]int64{"example": options})
	if err != nil {
		t.Fatalf("the example mod no longer loads: %v", err)
	}
	if len(refs) != 1 || refs[0].Name != "example" {
		t.Fatalf("refs %+v, want the example mod", refs)
	}
	if r.Pirates == nil || r.RegionBrains[game.Desert] == nil {
		t.Fatal("the example's scripts were not loaded")
	}
	return r
}

func TestExampleModFigures(t *testing.T) {
	if _, err := os.Stat(filepath.Join(exampleDir, "LICENSE")); err != nil {
		t.Error(err)
	}
	base := game.BaseRules()
	r := exampleRules(t, nil)
	if got := r.Units[game.Jet].Cost; got != 120 {
		t.Errorf("jet build cost %d, want 120", got)
	}
	if got := r.Units[game.Tank].Cost; got != 400 {
		t.Errorf("tank build cost %d (option default), want 400", got)
	}
	if r.Units[game.Turret].HQBonus != 2 || r.Units[game.Tank].HQBonus != 2 {
		t.Errorf("hq_bonus turret %d, tank %d, want 2 and 2",
			r.Units[game.Turret].HQBonus, r.Units[game.Tank].HQBonus)
	}
	if got := r.HQ.PriceBase; got != 4000 {
		t.Errorf("HQ base price %d, want 4000", got)
	}
	if got := r.Regions[game.Agricultural].Upkeep; got != 800 {
		t.Errorf("agricultural upkeep %d, want 800", got)
	}
	nuke := r.Attacks[game.EffectNuclear]
	if got := nuke.Target.Kill[game.KillIndex(game.Turret)]; got != (game.Roll{Pct: 20}) {
		t.Errorf("nuclear turret kill %+v, want a flat 20", got)
	}
	if got := nuke.Attacker.People; got != (game.Roll{Pct: 1, Up: 2}) {
		t.Errorf("nuclear blowback on people %+v, want 1 up 2", got)
	}
	if nuke.Target.Waste != base.Attacks[game.EffectNuclear].Target.Waste {
		t.Error("the example changed the nuclear waste band")
	}
	if r.Units[game.Bomber] != base.Units[game.Bomber] {
		t.Error("the example changed bombers")
	}

	r = exampleRules(t, map[string]int64{"tank_cost": 250, "cheap_hq": 1})
	if got := r.Units[game.Tank].Cost; got != 250 {
		t.Errorf("tank build cost %d with tank_cost 250, want 250", got)
	}
	if r.HQ.PriceBase != 1500 || r.HQ.BuildPerTurn != 15 {
		t.Errorf("HQ price %d, build %d with cheap_hq on, want 1500 and 15", r.HQ.PriceBase, r.HQ.BuildPerTurn)
	}
}

func TestExampleModScriptsRun(t *testing.T) {
	out := captureWarnings(t)

	w, e := scriptWorld(exampleRules(t, nil), 1)
	raids := 0
	for range 100 {
		w.PlayTurn(e, "2026-11-23")
		for _, h := range e.PirateHits {
			raids++
			if h.Spoil != game.SpoilJets {
				t.Errorf("raid took %v, want jets", h.Spoil)
			}
		}
	}
	if raids == 0 {
		t.Fatal("the example's pirates.star never raided a realm with a jet stockpile")
	}
	poor, pe := scriptWorld(exampleRules(t, nil), 1)
	pe.Jets = 1000
	for range 100 {
		poor.PlayTurn(pe, "2026-11-23")
		if len(pe.PirateHits) != 0 {
			t.Fatalf("a realm with few jets was raided: %+v", pe.PirateHits)
		}
	}

	// Deserts grow food only once the realm has played farm_turns turns.
	rules := exampleRules(t, map[string]int64{"farm_turns": 30})
	mix := game.RegionMix{Desert: 10}
	rw, re := regionWorld(rules, mix)
	re.TurnsPlayed = 29
	if got := rw.FoodGrown(re); got != 0 {
		t.Errorf("food grown %d at 29 turns, want 0", got)
	}
	re.TurnsPlayed = 30
	if got := rw.FoodGrown(re); got != 1500 {
		t.Errorf("food grown %d at 30 turns, want 1500 (10 deserts x 150)", got)
	}

	if got := out.String(); got != "" {
		t.Errorf("the example's scripts warned:\n%s", got)
	}
}
