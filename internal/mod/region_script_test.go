package mod

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// regionRules checks a mod carrying script as its regions.star.
func regionRules(t *testing.T, script string) *game.Rules {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{
		ManifestFile: manifest("lands"),
		RegionsFile:  script,
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// regionWorld is a fixed-seed world playing r, with one realm holding mix.
func regionWorld(r *game.Rules, mix game.RegionMix) (*game.World, *game.Empire) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Rules = r
	e := w.AddHuman("e", "Empire")
	e.Regions = mix
	e.Land = mix.Total()
	return w, e
}

const desertFood = `
def regions(realm, rows):
    return {"desert": {"food_base": 100}}
`

// A script's figures reach the engine: deserts that grow nothing in the base
// game grow 100 food each, and the turn report has a line for it.
func TestRegionScriptChangesWhatRegionsDo(t *testing.T) {
	w, e := regionWorld(regionRules(t, desertFood), game.RegionMix{Desert: 10})
	if got := w.FoodGrown(e); got != 1000 {
		t.Errorf("food grown %d, want 1000 (10 deserts x 100)", got)
	}
	if got := w.IncomeThisTurn(e).FoodFrom(game.Desert); got != 1000 {
		t.Errorf("desert food line %d, want 1000", got)
	}
	plain, pe := regionWorld(nil, game.RegionMix{Desert: 10})
	if got := plain.FoodGrown(pe); got != 0 {
		t.Errorf("base deserts grew %d food, want none", got)
	}
}

const smallRealmBonus = `
def regions(realm, rows):
    if realm.land >= 100:
        return {}
    return {"coastal": {"gold_base": rows["coastal"].gold_base * 2, "gold_rate": 0, "support_share": 0}}
`

// The answer is the realm's own: a small realm's coastal regions pay double,
// a large realm's what the rows say.
func TestRegionScriptAnswersRealmByRealm(t *testing.T) {
	r := regionRules(t, smallRealmBonus)
	w, small := regionWorld(r, game.RegionMix{Coastal: 10})
	small.Support = 100
	big := w.AddHuman("b", "Big")
	big.Regions = game.RegionMix{Coastal: 10, Urban: 200}
	big.Land = big.Regions.Total()
	big.Support = 100
	small.TechSlots, big.TechSlots = [game.TechSlotCount]int{}, [game.TechSlotCount]int{}
	if got := w.IncomeThisTurn(small).GoldFrom(game.Coastal); got != 10*2*game.CoastalBase {
		t.Errorf("small realm's tourism %d, want %d", got, 10*2*game.CoastalBase)
	}
	if got := w.IncomeThisTurn(big).GoldFrom(game.Coastal); got < 10*game.CoastalBase || got >= 10*(game.CoastalBase+game.CoastalRate) {
		t.Errorf("large realm's tourism %d, want the base band", got)
	}
}

// A script that fails, or answers outside the limits, leaves the realm on the
// mod's values, and says why once.
func TestFailingRegionScriptFallsBack(t *testing.T) {
	for _, c := range []struct{ name, script, reason string }{
		{"runtime error", "def regions(realm, rows):\n    return rows[\"atlantis\"]\n", "regions.star: line 2: key \"atlantis\" not in dict"},
		{"not a dict", "def regions(realm, rows):\n    return []\n", "must return a dict"},
		{"unknown type", "def regions(realm, rows):\n    return {\"atlantis\": {}}\n", "\"atlantis\" is not a region type"},
		{"unknown key", "def regions(realm, rows):\n    return {\"desert\": {\"ore\": 1}}\n", "desert: unknown key \"ore\""},
		{"out of range", "def regions(realm, rows):\n    return {\"desert\": {\"gold_share\": 101}}\n", "desert: gold_share 101 is outside 0 to 100"},
		{"waste", "def regions(realm, rows):\n    return {\"waste\": {\"food_base\": 5}}\n", "waste: food_base must be 0"},
		{"step cap", "def regions(realm, rows):\n    for i in range(100000000):\n        pass\n", "too many steps"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := captureWarnings(t)
			mix := game.RegionMix{Desert: 10, River: 5, Agricultural: 5}
			w, e := regionWorld(regionRules(t, c.script), mix)
			plain, pe := regionWorld(nil, mix)
			for range 3 {
				if got, want := w.IncomeThisTurn(e), plain.IncomeThisTurn(pe); got != want {
					t.Errorf("income %+v, want the mod's values: %+v", got, want)
				}
			}
			if got := out.String(); strings.Count(got, "\n") != 1 || !strings.Contains(got, c.reason) || !strings.Contains(got, `mod "lands"`) {
				t.Errorf("warning %q, want one line naming the mod and %q", got, c.reason)
			}
		})
	}
}

func TestCheckRefusesARegionScriptWithoutRegions(t *testing.T) {
	for _, c := range []struct{ name, script, want string }{
		{"no regions", "def raid(realm, factions):\n    return []\n", "must define regions(realm, rows)"},
		{"one parameter", "def regions(realm):\n    return {}\n", "exactly two parameters"},
		// A region script must answer the same every time it is asked, so it
		// has no random numbers at all.
		{"rand", "def regions(realm, rows):\n    return {\"desert\": {\"food_base\": rand(9)}}\n", "undefined: rand"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: manifest("m"), RegionsFile: c.script})
			if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err %v, want %q", err, c.want)
			}
		})
	}
}

// -check-mod names the script among the mod's changes.
func TestDiffNamesTheRegionScript(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{ManifestFile: manifest("lands"), RegionsFile: desertFood})
	m, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "regions: the values every realm plays by -> lands/" + RegionsFile + " (" + m.Hash + ")"
	if got := Diff(game.BaseRules(), r); len(got) != 1 || got[0] != want {
		t.Errorf("Diff %q, want [%q]", got, want)
	}
}
