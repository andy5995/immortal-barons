package mod

import (
	"os"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func optionManifest(name, options string) string { return manifest(name) + options }

func TestOptionDeclarationsAreRefused(t *testing.T) {
	const good = `[options.tank_cost]
label = "Tank cost"
description = "What one tank costs"
`
	cases := []struct{ name, options, want string }{
		{"unknown type", good + `type = "text"
default = 100
`, `type "text" is unknown`},
		{"missing label", "[options.tank_cost]\ntype = \"number\"\ndefault = 100\nmin = 1\nmax = 200\n", "label is required"},
		{"control character", "[options.tank_cost]\nlabel = \"a\\u001b\"\ntype = \"number\"\ndefault = 100\nmin = 1\nmax = 200\n", "label may not hold control characters"},
		{"default outside range", good + `type = "number"
default = 300
min = 1
max = 200
`, "default 300 is outside 1 to 200"},
		{"reversed range", good + `type = "number"
default = 100
min = 200
max = 1
`, "min 200 is greater than max 1"},
		{"missing bounds", good + `type = "number"
default = 100
`, "min and max are required"},
		{"bool bound", good + `type = "bool"
default = false
min = 0
max = 1
`, "min and max are for number options only"},
		{"bool default number", good + `type = "bool"
default = 1
`, "default of a bool must be true or false, not 1"},
		{"number default bool", good + `type = "number"
default = true
min = 1
max = 200
`, "default of a number must be a whole number, not true"},
		{"number default float", good + `type = "number"
default = 1.5
min = 1
max = 200
`, "default of a number must be a whole number, not 1.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: optionManifest("m", c.options)})
			_, _, err := Check(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestNumberOptionFeedsValuesAndIsRecorded(t *testing.T) {
	data := t.TempDir()
	const options = `[options.tank_cost]
label = "Tank cost"
description = "What one tank costs"
type = "number"
default = 500
min = 1
max = 2000000
`
	writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", options),
		ValuesFile:   "[units.tank]\nbuild_cost = { option = \"tank_cost\" }\n",
	})
	chosen := map[string]map[string]int64{"m": {"tank_cost": 400}}
	r, refs, err := Resolve(data, []string{"m"}, chosen)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 400 {
		t.Errorf("tank cost %d, want the chosen 400", got)
	}
	if got := refs[0].Options["tank_cost"]; got != 400 {
		t.Errorf("recorded option %d, want 400", got)
	}

	chosen["m"]["tank_cost"] = 0
	_, _, err = Resolve(data, []string{"m"}, chosen)
	if err == nil || !strings.Contains(err.Error(), `option "tank_cost" must be from 1 to 2000000`) {
		t.Errorf("err = %v, want the option's range", err)
	}

	// The option's own range is not the game's limit: a value inside it still
	// goes through Rules.Validate like a figure written directly.
	chosen["m"]["tank_cost"] = 2000000
	_, _, err = Resolve(data, []string{"m"}, chosen)
	if err == nil || !strings.Contains(err.Error(), "build_cost 2000000 is outside") {
		t.Errorf("err = %v, want Rules.Validate to refuse the value", err)
	}
}

func TestValuesRefuseAnOptionItCannotUse(t *testing.T) {
	cases := []struct{ name, options, values, want string }{
		{"unknown option", "", "[units.tank]\nbuild_cost = { option = \"nope\" }\n", `option "nope" does not exist`},
		{"bool in a figure", `[options.fast_start]
label = "Fast"
type = "bool"
default = false
`, "[units.tank]\nbuild_cost = { option = \"fast_start\" }\n", "is a bool, not a number"},
		{"extra key", `[options.cost]
label = "Cost"
type = "number"
default = 100
min = 1
max = 200
`, "[units.tank]\nbuild_cost = { option = \"cost\", other = 1 }\n", "unknown key units.tank.build_cost.other"},
		{"top-level option", "", "option = \"x\"\n", "unknown key option"},
		{"option beside a unit", "", "[units.tank]\noption = { x = 1 }\n", "unknown key units.tank.option.x"},
		{"option in a section", "", "[hq]\n[hq.option]\nx = 1\n", "unknown key hq.option.x"},
		{"option in a when section", "", "[when.fast]\noption = \"x\"\n", "unknown key when.fast.option"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{
				ManifestFile: optionManifest("m", c.options),
				ValuesFile:   c.values,
			})
			_, _, err := Check(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestBoolOptionSelectsAWhenSection(t *testing.T) {
	data := t.TempDir()
	const options = `[options.fast_start]
label = "Fast start"
type = "bool"
default = false
`
	writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", options),
		ValuesFile: `
[units.tank]
build_cost = 500
[when.fast_start.units.tank]
build_cost = 400
[hq]
build_per_turn = 20
[when.fast_start.hq]
build_per_turn = 30
`,
	})
	r, refs, err := Resolve(data, []string{"m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 500 {
		t.Errorf("off: tank cost %d, want 500", got)
	}
	if got := r.HQ.BuildPerTurn; got != 20 {
		t.Errorf("off: HQ build per turn %d, want 20", got)
	}
	if got := refs[0].Options["fast_start"]; got != 0 {
		t.Errorf("off: recorded bool %d, want 0", got)
	}

	r, refs, err = Resolve(data, []string{"m"}, map[string]map[string]int64{"m": {"fast_start": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 400 {
		t.Errorf("on: tank cost %d, want 400", got)
	}
	if got := r.HQ.BuildPerTurn; got != 30 {
		t.Errorf("on: HQ build per turn %d, want 30", got)
	}
	if got := refs[0].Options["fast_start"]; got != 1 {
		t.Errorf("on: recorded bool %d, want 1", got)
	}
}

func TestWhenSectionRefusesABadSelector(t *testing.T) {
	for _, c := range []struct{ name, options, values, want string }{
		{"unknown option", "", "[when.nope.units.tank]\nbuild_cost = 400\n", `when.nope: option "nope" does not exist`},
		{"number option", `[options.cost]
label = "Cost"
type = "number"
default = 100
min = 1
max = 200
`, "[when.cost.units.tank]\nbuild_cost = 400\n", "only a bool option selects a section"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{
				ManifestFile: optionManifest("m", c.options),
				ValuesFile:   c.values,
			})
			_, _, err := Check(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestScriptsSeeChosenOptions(t *testing.T) {
	data := t.TempDir()
	const options = `[options.slot]
type = "number"
label = "Slot"
default = 1
min = 0
max = 2
[options.bonus]
type = "number"
label = "Bonus"
default = 0
min = 0
max = 100
[options.fast_start]
type = "bool"
label = "Fast start"
default = false
`
	files := map[string]string{
		ManifestFile: optionManifest("m", options),
		PiratesFile: `SLOT = option("slot")
FAST = option("fast_start")
def raid(realm, factions):
    if FAST:
        return []
    return [{"faction": SLOT, "take": "jets", "market": False}]
`,
		desertScript: `BONUS = option("bonus")
def region(realm, row):
    return {"food_base": 50 + BONUS}
`,
	}
	writeMod(t, data, "m", files)
	play := func(given map[string]int64, wantSlot int, wantFood int) {
		t.Helper()
		r, _, err := Resolve(data, []string{"m"}, map[string]map[string]int64{"m": given})
		if err != nil {
			t.Fatal(err)
		}
		bw, be := regionWorld(r, game.RegionMix{Desert: 10})
		if got := bw.FoodGrown(be); got != wantFood {
			t.Errorf("food %d, want %d", got, wantFood)
		}
		w, e := scriptWorld(r, 1)
		w.PlayTurn(e, "2026-11-23")
		if wantSlot < 0 {
			if len(e.PirateHits) != 0 {
				t.Errorf("raids %+v, want none", e.PirateHits)
			}
			return
		}
		if len(e.PirateHits) != 1 || e.PirateHits[0].Slot != wantSlot {
			t.Errorf("raids %+v, want one by faction %d", e.PirateHits, wantSlot)
		}
	}
	play(map[string]int64{"slot": 1, "bonus": 10}, 1, 600)
	// Same folder and source, different choices: the second script must not
	// reuse the first one's compiled top level.
	play(map[string]int64{"slot": 2, "bonus": 20, "fast_start": 1}, -1, 700)
}

func TestScriptRefusesAnUnknownOption(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "m", map[string]string{
		ManifestFile: manifest("m"),
		PiratesFile:  "X = option(\"nope\")\ndef raid(realm, factions):\n    return []\n",
	})
	_, _, err := Check(dir)
	if err == nil || !strings.Contains(err.Error(), `option "nope" does not exist`) {
		t.Errorf("err = %v, want an unknown option", err)
	}
}

func TestLoadForLeagueCarriesTheChosenValues(t *testing.T) {
	data := t.TempDir()
	const options = `[options.tank_cost]
label = "Tank cost"
type = "number"
default = 500
min = 1
max = 1000
`
	dir := writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", options),
		ValuesFile:   "[units.tank]\nbuild_cost = { option = \"tank_cost\" }\n",
	})
	_, refs, err := Resolve(data, []string{"m"}, map[string]map[string]int64{"m": {"tank_cost": 400}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	r, err := LoadForLeague(refs, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 400 {
		t.Errorf("league tank cost %d, want 400", got)
	}
	refs[0].Options["tank_cost"] = 450
	r, err = LoadForLeague(refs, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 450 {
		t.Errorf("league tank cost %d, want 450", got)
	}
}

// Game Setup reads a saved game's manifest without checking it again, so an
// option's default must come out right from the decoded manifest alone.
func TestManifestFromRefGivesOptionDefaults(t *testing.T) {
	ref := game.ModRef{Name: "m", Files: map[string]string{ManifestFile: `name = "m"
version = "1"
mod_api = 1
license = "MIT"
[options.fast]
type = "bool"
label = "Fast"
default = true
[options.cost]
type = "number"
label = "Cost"
default = 500
min = 1
max = 900
`}}
	man, err := ManifestFromRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	if got := man.Options["fast"].Value(); got != 1 {
		t.Errorf("bool default true read back as %d, want 1", got)
	}
	if got := man.Options["cost"].Value(); got != 500 {
		t.Errorf("number default read back as %d, want 500", got)
	}
}

// An option reference is accepted wherever a figure is, in hq, regions and
// under a when section, while a stray key named option is not.
func TestOptionReferencesInEverySection(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", `[options.n]
label = "N"
type = "number"
default = 25
min = 1
max = 100
[options.on]
label = "On"
type = "bool"
default = true
`),
		ValuesFile: `
[hq]
build_per_turn = { option = "n" }
[regions.coastal]
gold_base = { option = "n" }
[when.on.hq]
build_per_turn = { option = "n" }
[when.on.regions.coastal]
gold_base = { option = "n" }
`,
	})
	r, _, err := Resolve(data, []string{"m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.HQ.BuildPerTurn; got != 25 {
		t.Errorf("HQ build per turn %d, want 25", got)
	}
}
