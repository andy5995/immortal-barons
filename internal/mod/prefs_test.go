package mod

import (
	"encoding/json"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/andy5995/immortal-barons/internal/game"
)

const prefsManifest = `
[prefs.fierce]
label = "Fiercer raids"
description = "Pirates raid you every turn."
default = false
`

// prefsMod checks a mod named "pirates" holding the prefs manifest and script.
func prefsMod(t *testing.T, manifestExtra, script string) (*Mod, *game.Rules) {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "pirates", map[string]string{
		ManifestFile: manifest("pirates") + manifestExtra,
		PiratesFile:  script,
	})
	m, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m, r
}

const fierceScript = `
def raid(realm, factions):
    if realm.pref("fierce"):
        return [{"faction": 1, "take": "jets"}]
    return []
`

func TestPrefDefaultAppliesUntilThePlayerChooses(t *testing.T) {
	_, r := prefsMod(t, prefsManifest, fierceScript)
	w, e := scriptWorld(r, 1)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) != 0 {
		t.Fatalf("default false still raided: %+v", e.PirateHits)
	}
	e.SetModPref("pirates", "fierce", true)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) == 0 {
		t.Fatal("a player who chose fierce raids was not raided")
	}
	// The default is a mod's to change: true, and an unset realm raids.
	_, r = prefsMod(t, strings.Replace(prefsManifest, "default = false", "default = true", 1), fierceScript)
	w, e = scriptWorld(r, 1)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) == 0 {
		t.Error("default true did not raid an unset realm")
	}
	e.PirateHits = nil
	e.SetModPref("pirates", "fierce", false)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) != 0 {
		t.Error("an explicit false did not override a true default")
	}
}

func TestAnotherModsPrefIsNotSeen(t *testing.T) {
	_, r := prefsMod(t, prefsManifest, fierceScript)
	w, e := scriptWorld(r, 1)
	e.SetModPref("other", "fierce", true)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) != 0 {
		t.Error("a pref of another mod reached this mod's script")
	}
}

func TestUnknownPrefFailsTheScriptAndFallsBack(t *testing.T) {
	out := captureWarnings(t)
	_, r := prefsMod(t, prefsManifest, "def raid(realm, factions):\n    return [{\"faction\": 1, \"take\": \"jets\"}] if realm.pref(\"nope\") else []\n")
	w, e := scriptWorld(r, 1)
	w.PlayTurn(e, "2026-11-23")
	if got := out.String(); !strings.Contains(got, `pref "nope" does not exist in this mod`) {
		t.Errorf("warning %q does not name the unknown pref", got)
	}
}

func TestRegionScriptAnswerFollowsAPrefToggle(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "lands", map[string]string{
		ManifestFile: manifest("lands") + `
[prefs.farm]
label = "Farm the desert"
default = false
`,
		desertScript: "def region(realm, row):\n    return {\"food_base\": 100 if realm.pref(\"farm\") else 0}\n",
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, e := regionWorld(r, game.RegionMix{Desert: 10})
	for range 2 { // the second call is answered from the script's memory
		if got := w.FoodGrown(e); got != 0 {
			t.Fatalf("food grown %d before the choice, want 0", got)
		}
	}
	e.SetModPref("lands", "farm", true)
	if got := w.FoodGrown(e); got != 1000 {
		t.Errorf("food grown %d after the toggle, want 1000: a stale answer", got)
	}
	e.SetModPref("lands", "farm", false)
	if got := w.FoodGrown(e); got != 0 {
		t.Errorf("food grown %d after toggling back, want 0", got)
	}
}

func TestPrefManifestIsChecked(t *testing.T) {
	many := ""
	for i := range MaxPrefs + 1 {
		many += "[prefs.p" + string(rune('a'+i)) + "]\nlabel = \"L\"\ndefault = true\n"
	}
	for name, c := range map[string]struct{ toml, want string }{
		"no label":      {"[prefs.a]\ndefault = true\n", "label is required"},
		"no default":    {"[prefs.a]\nlabel = \"A\"\n", "default is required"},
		"default text":  {"[prefs.a]\nlabel = \"A\"\ndefault = \"yes\"\n", "prefs"},
		"bad id":        {"[prefs.\"Bad Id\"]\nlabel = \"A\"\ndefault = true\n", "lowercase letters"},
		"too many":      {many, "at most 8"},
		"escape label":  {"[prefs.a]\nlabel = \"A\\u001b[2J\"\ndefault = true\n", "control characters"},
		"format char":   {"[prefs.a]\nlabel = \"A\\u202eB\"\ndefault = true\n", "control characters"},
		"escape desc":   {"[prefs.a]\nlabel = \"A\"\ndescription = \"x\\u0007\"\ndefault = true\n", "control characters"},
		"long label":    {"[prefs.a]\nlabel = \"" + strings.Repeat("x", MaxPrefLabel+1) + "\"\ndefault = true\n", "at most 40"},
		"unknown field": {"[prefs.a]\nlabel = \"A\"\ndefault = true\ntype = \"bool\"\n", "type"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "pirates", map[string]string{ManifestFile: manifest("pirates") + c.toml})
			_, _, err := Check(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error holding %q", err, c.want)
			}
			// A copy that arrives in a world or a league order is held to the same.
			if _, err := ManifestFromRef(game.ModRef{Name: "pirates", Files: map[string]string{ManifestFile: manifest("pirates") + c.toml}}); err == nil {
				t.Error("ManifestFromRef accepted it")
			}
		})
	}
}

func TestPrefRowsCleanTheirText(t *testing.T) {
	ref := game.ModRef{Name: "pirates", Files: map[string]string{ManifestFile: manifest("pirates") + prefsManifest}}
	rows := PrefRows([]game.ModRef{ref, {Name: "broken", Files: map[string]string{ManifestFile: "not toml ["}}})
	if len(rows) != 1 || rows[0].Mod != "pirates" || rows[0].ID != "fierce" || rows[0].Default {
		t.Fatalf("rows %+v", rows)
	}
}

func TestAnOldEmpireWithoutPrefsLoads(t *testing.T) {
	var e game.Empire
	if err := json.Unmarshal([]byte(`{"Name":"Old"}`), &e); err != nil {
		t.Fatal(err)
	}
	if !e.ModPref("m", "x", true) || e.ModPref("m", "x", false) {
		t.Error("an empire with no recorded choice must play the mod's default")
	}
	out, _ := json.Marshal(e)
	if strings.Contains(string(out), "ModPrefs") {
		t.Errorf("an empty ModPrefs was saved: %s", out)
	}
	e.SetModPref("m", "x", false)
	if e.ModPref("m", "x", true) {
		t.Error("an explicit false lost to the default")
	}
}

func TestPrefBuiltinWithoutDeclarationsRefusesEverything(t *testing.T) {
	var p *prefValues
	th := &starlark.Thread{}
	if _, err := starlark.Call(th, p.builtin(""), starlark.Tuple{starlark.String("x")}, nil); err == nil {
		t.Error("pref() answered for a mod that declares none")
	}
}

func TestTooManyPrefsAcrossModsIsRefusedNamingThem(t *testing.T) {
	data := t.TempDir()
	var names []string
	for _, n := range []string{"aa", "bb", "cc", "dd"} {
		toml := manifest(n)
		for i := range MaxPrefs {
			toml += "[prefs.p" + string(rune('a'+i)) + "]\nlabel = \"L\"\ndefault = true\n"
		}
		writeMod(t, data, n, map[string]string{ManifestFile: toml})
		names = append(names, n)
	}
	if _, _, err := Resolve(data, names[:3], nil); err != nil {
		t.Fatalf("24 prefs refused: %v", err)
	}
	_, _, err := Resolve(data, names, nil)
	if err == nil || !strings.Contains(err.Error(), "32") || !strings.Contains(err.Error(), "aa 8") || !strings.Contains(err.Error(), "dd 8") {
		t.Fatalf("got %v, want a refusal naming the mods and the total", err)
	}
}

func TestPrefDescriptionIsCapped(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "pirates", map[string]string{ManifestFile: manifest("pirates") +
		"[prefs.a]\nlabel = \"A\"\ndefault = true\ndescription = \"" + strings.Repeat("x", MaxPrefDescription+1) + "\"\n"})
	if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), "description is") {
		t.Fatalf("got %v", err)
	}
}
