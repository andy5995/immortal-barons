package mod

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// desertScript and urbanScript are the keys a mod's region scripts for those
// types have in writeMod's files.
const (
	desertScript = RegionsDir + "/desert.star"
	urbanScript  = RegionsDir + "/urban.star"
)

// regionRules checks a mod carrying script as its script for deserts.
func regionRules(t *testing.T, script string) *game.Rules {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{
		ManifestFile: manifest("lands"),
		desertScript: script,
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
def region(realm, row):
    return {"food_base": 100}
`

// A script's figures reach the engine: deserts that grow nothing in the base
// game grow 100 food each, and the turn report has a line for it.
func TestRegionScriptChangesWhatItsRegionsDo(t *testing.T) {
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

// A script changes only its own type: the desert script leaves coasts alone.
func TestRegionScriptTouchesOnlyItsType(t *testing.T) {
	mix := game.RegionMix{Desert: 10, Coastal: 10, Agricultural: 10}
	w, e := regionWorld(regionRules(t, desertFood), mix)
	plain, pe := regionWorld(nil, mix)
	got, want := w.IncomeThisTurn(e), plain.IncomeThisTurn(pe)
	for _, g := range []*game.Region{game.Coastal, game.Agricultural} {
		if got.GoldFrom(g) != want.GoldFrom(g) || got.FoodFrom(g) != want.FoodFrom(g) {
			t.Errorf("%s: gold %d food %d, want the base game's %d and %d",
				g.ID, got.GoldFrom(g), got.FoodFrom(g), want.GoldFrom(g), want.FoodFrom(g))
		}
	}
}

const smallRealmCoast = `
def region(realm, row):
    if realm.land >= 100:
        return {}
    return {"gold_base": row.gold_base * 2, "gold_rate": 0, "support_share": 0}
`

// The answer is the realm's own: a small realm's coastal regions pay double,
// a large realm's what the row says.
func TestRegionScriptAnswersRealmByRealm(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "coasts", map[string]string{
		ManifestFile:                 manifest("coasts"),
		RegionsDir + "/coastal.star": smallRealmCoast,
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, small := regionWorld(r, game.RegionMix{Coastal: 10})
	small.Support = 100
	big := w.AddHuman("b", "Big")
	big.Regions = game.RegionMix{Coastal: 10, Urban: 200}
	big.Land = big.Regions.Total()
	big.Support = 100
	if got := w.IncomeThisTurn(small).GoldFrom(game.Coastal); got != 10*2*game.CoastalBase {
		t.Errorf("small realm's tourism %d, want %d", got, 10*2*game.CoastalBase)
	}
	if got := w.IncomeThisTurn(big).GoldFrom(game.Coastal); got < 10*game.CoastalBase || got >= 10*(game.CoastalBase+game.CoastalRate) {
		t.Errorf("large realm's tourism %d, want the base band", got)
	}
}

// row carries the mod's own values.toml figures, so a script builds on them.
func TestRegionScriptSeesTheModsValues(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{
		ManifestFile: manifest("lands"),
		ValuesFile:   "[regions.desert]\nfood_base = 50\n",
		desertScript: "def region(realm, row):\n    return {\"food_base\": row.food_base * 2}\n",
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, e := regionWorld(r, game.RegionMix{Desert: 10})
	if got := w.FoodGrown(e); got != 1000 {
		t.Errorf("food grown %d, want 1000 (10 deserts x 50 x 2)", got)
	}
}

// A later mod's script for a type replaces an earlier one's, as a later mod's
// values.toml figure does; scripts for different types both run.
func TestALaterModsRegionScriptReplacesAnEarlierOnes(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "farms", map[string]string{ManifestFile: manifest("farms"),
		desertScript: "def region(realm, row):\n    return {\"food_base\": 100}\n",
		urbanScript:  "def region(realm, row):\n    return {\"food_base\": 10}\n"})
	writeMod(t, data, "better-farms", map[string]string{ManifestFile: manifest("better-farms"),
		desertScript: "def region(realm, row):\n    return {\"food_base\": 300}\n"})
	r, _, err := Resolve(data, []string{"farms", "better-farms"})
	if err != nil {
		t.Fatal(err)
	}
	w, e := regionWorld(r, game.RegionMix{Desert: 10, Urban: 10})
	b := w.IncomeThisTurn(e)
	if got := b.FoodFrom(game.Desert); got != 3000 {
		t.Errorf("desert food %d, want better-farms' 3000", got)
	}
	if got := b.FoodFrom(game.Urban); got != 100 {
		t.Errorf("urban food %d, want farms' 100", got)
	}
}

// A script that fails, or answers outside the limits, leaves the realm on the
// mod's values for that type, and says why once.
func TestFailingRegionScriptFallsBack(t *testing.T) {
	for _, c := range []struct{ name, file, script, reason string }{
		{"runtime error", desertScript, "def region(realm, row):\n    return {\"food_base\": row.atlantis}\n", "regions/desert.star: line 2: struct has no .atlantis attribute"},
		{"not a dict", desertScript, "def region(realm, row):\n    return []\n", "must return a dict"},
		{"unknown key", desertScript, "def region(realm, row):\n    return {\"ore\": 1}\n", "unknown key \"ore\""},
		{"out of range", desertScript, "def region(realm, row):\n    return {\"gold_share\": 101}\n", "desert: gold_share 101 is outside 0 to 100"},
		{"waste", RegionsDir + "/waste.star", "def region(realm, row):\n    return {\"food_base\": 5}\n", "waste: food_base must be 0"},
		{"step cap", desertScript, "def region(realm, row):\n    for i in range(100000000):\n        pass\n", "too many steps"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := captureWarnings(t)
			dir := writeMod(t, t.TempDir(), "lands", map[string]string{ManifestFile: manifest("lands"), c.file: c.script})
			_, r, err := Check(dir)
			if err != nil {
				t.Fatal(err)
			}
			mix := game.RegionMix{Desert: 10, River: 5, Agricultural: 5, Waste: 3}
			w, e := regionWorld(r, mix)
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

func TestCheckRefusesABadRegionScript(t *testing.T) {
	for _, c := range []struct{ name, file, script, want string }{
		{"no region", desertScript, "def regions(realm, rows):\n    return {}\n", "must define region(realm, row)"},
		{"one parameter", desertScript, "def region(realm):\n    return {}\n", "exactly two parameters"},
		// A region script must answer the same every time it is asked, so it
		// has no random numbers at all.
		{"rand", desertScript, "def region(realm, row):\n    return {\"food_base\": rand(9)}\n", "undefined: rand"},
		{"not a type", RegionsDir + "/atlantis.star", desertFood, "regions/atlantis.star is not a region script"},
		{"not a script", RegionsDir + "/notes.txt", "hello", "regions/notes.txt is not a region script"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: manifest("m"), c.file: c.script})
			if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err %v, want %q", err, c.want)
			}
		})
	}
}

// Only the regions folder may hold anything below the mod folder.
func TestAModHoldsNoOtherFolder(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: manifest("m"), "extras/x.star": desertFood})
	if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), "a folder in a mod may only be regions") {
		t.Errorf("err %v, want the folder refused", err)
	}
}

// -check-mod names each region script among the mod's changes.
func TestDiffNamesTheRegionScripts(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{ManifestFile: manifest("lands"), desertScript: desertFood})
	m, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "desert script: none -> lands/regions/desert.star (" + m.Hash + ")"
	if got := Diff(game.BaseRules(), r); len(got) != 1 || got[0] != want {
		t.Errorf("Diff %q, want [%q]", got, want)
	}
}

// A world goes on playing the region scripts it was reset with: the saved copy
// carries the regions folder.
func TestRestoreRebuildsTheRegionScripts(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "lands", map[string]string{ManifestFile: manifest("lands"), desertScript: desertFood})
	_, refs, err := Resolve(data, []string{"lands"})
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := Restore(data, refs, true)
	if err != nil {
		t.Fatal(err)
	}
	w, e := regionWorld(r, game.RegionMix{Desert: 10})
	if got := w.FoodGrown(e); got != 1000 {
		t.Errorf("restored script grew %d food, want 1000", got)
	}
}

// Every mod says which mod format it was written for, so a change that breaks
// mods is told apart from a mistake, before any value is applied.
func TestAModNeedsAModAPI(t *testing.T) {
	for _, c := range []struct{ name, manifest, want string }{
		{"missing", "name = \"m\"\nversion = \"1\"\nlicense = \"CC0-1.0\"\n", "mod_api is required; this version of the game reads mod_api = 1"},
		{"newer", "name = \"m\"\nversion = \"1\"\nlicense = \"CC0-1.0\"\nmod_api = 2\n", "written for mod_api 2, but this version of the game reads mod_api 1; it needs a newer version of the game"},
		{"not a version", "name = \"m\"\nversion = \"1\"\nlicense = \"CC0-1.0\"\nmod_api = -1\n", "mod_api -1 is not a version"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// With a script, and without one: the format covers values.toml too.
			for _, files := range []map[string]string{{desertScript: desertFood}, {ValuesFile: "[units.tank]\nbuild_cost = 250\n"}} {
				files[ManifestFile] = c.manifest
				dir := writeMod(t, t.TempDir(), "m", files)
				if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), c.want) {
					t.Errorf("err %v, want %q", err, c.want)
				}
			}
		})
	}
}
