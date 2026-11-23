package mod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// writeMod creates <dataDir>/mods/<name> holding the given files.
func writeMod(t *testing.T, dataDir, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(dataDir, Dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func manifest(name string) string {
	return "name = \"" + name + "\"\nversion = \"1.0\"\n"
}

func TestBaseRulesAreWithinTheModLimits(t *testing.T) {
	// A base value outside the limits would make every mod fail to load.
	if err := game.BaseRules().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyOverridesOnlyWhatItNames(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "cheap-tanks", map[string]string{
		ManifestFile: manifest("cheap-tanks"),
		ValuesFile: `
# comments are allowed, which is why the format is TOML
[units.tank]
build_cost = 400
networth = 1.5
upkeep = 0.3
`,
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(Diff(game.BaseRules(), r), "\n")
	want := "tank.build_cost: 500 -> 400\ntank.networth: 1.25 -> 1.5\ntank.upkeep: 0.6 -> 0.3"
	if got != want {
		t.Errorf("diff:\n%s\nwant:\n%s", got, want)
	}
}

func TestReadRefusesWhatItDoesNotKnow(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"misspelled key", map[string]string{ValuesFile: "[units.tank]\ncots = 1\n"}, "units.tank.cots: unknown key"},
		{"unknown unit", map[string]string{ValuesFile: "[units.dragon]\nbuild_cost = 1\n"}, "units.dragon: not a unit"},
		{"unused file", map[string]string{"brain.star": "x"}, "brain.star is not a file"},
		{"too many places", map[string]string{ValuesFile: "[units.tank]\nupkeep = 0.25\n"}, "more decimal places than 0.1 allows"},
		{"out of range", map[string]string{ValuesFile: "[units.tank]\nbuild_cost = 0\n"}, "build_cost 0 is outside"},
		{"offense out of range", map[string]string{ValuesFile: "[units.jet]\noffense = 100.01\n"}, "offense 100.01 is outside 0 to 100"},
		{"turret offense", map[string]string{ValuesFile: "[units.turret]\noffense = 1\n"}, "turret: offense must be 0"},
		{"defense below zero", map[string]string{ValuesFile: "[units.trooper]\ndefense = -1\n"}, "defense -1 is outside 0 to 100"},
		{"wraps an int32", map[string]string{ValuesFile: "[units.tank]\nbuild_cost = 5000000000\n"}, "out of range"},
		{"control character", map[string]string{ManifestFile: manifest("m") + "label = \"a\\u001b[2J\"\n"}, "may not hold control characters"},
		{"narrow band", map[string]string{ValuesFile: "[units.tank]\nprice_low = 2200\n"}, "price_high must be at least"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{ManifestFile: manifest("m")}
			for k, v := range c.files {
				files[k] = v
			}
			_, _, err := Check(writeMod(t, t.TempDir(), "m", files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestManifestMustNameItsFolder(t *testing.T) {
	_, _, err := Check(writeMod(t, t.TempDir(), "a", map[string]string{ManifestFile: manifest("b")}))
	if err == nil || !strings.Contains(err.Error(), "must match the folder") {
		t.Errorf("err = %v", err)
	}
}

func TestResolveAppliesInOrderLaterWins(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "one", map[string]string{ManifestFile: manifest("one"), ValuesFile: "[units.tank]\nbuild_cost = 400\n[units.jet]\nbuild_cost = 100\n"})
	writeMod(t, data, "two", map[string]string{ManifestFile: manifest("two"), ValuesFile: "[units.tank]\nbuild_cost = 600\n"})
	r, refs, err := Resolve(data, []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Units[game.Tank].Cost; got != 600 {
		t.Errorf("tank cost %d, want the later mod's 600", got)
	}
	if got := r.Units[game.Jet].Cost; got != 100 {
		t.Errorf("jet cost %d, want the earlier mod's 100 kept", got)
	}
	if len(refs) != 2 || refs[0].Name != "one" || refs[1].Name != "two" || refs[0].Hash == "" {
		t.Errorf("refs = %+v", refs)
	}
	if _, _, err := Resolve(data, []string{"../one"}); err == nil {
		t.Error("a name with a path in it must be refused")
	}
}

// A mod edited or removed mid-game is reported, and the game keeps playing the
// values it was reset with.
func TestRestoreKeepsTheOldValuesOfAnEditedMod(t *testing.T) {
	data := t.TempDir()
	dir := writeMod(t, data, "m", map[string]string{ManifestFile: manifest("m"), ValuesFile: "[units.tank]\nbuild_cost = 400\n"})
	_, refs, err := Resolve(data, []string{"m"})
	if err != nil {
		t.Fatal(err)
	}
	r, warnings, err := Restore(data, refs)
	if err != nil || len(warnings) != 0 || r.Units[game.Tank].Cost != 400 {
		t.Fatalf("an unchanged mod restores silently: warnings %v, err %v", warnings, err)
	}

	if err := os.WriteFile(filepath.Join(dir, ValuesFile), []byte("[units.tank]\nbuild_cost = 450\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, warnings, err = Restore(data, refs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Units[game.Tank].Cost != 400 {
		t.Errorf("tank cost %d, want the value the game was reset with, 400", r.Units[game.Tank].Cost)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "keeps the mod's old values") {
		t.Errorf("warnings = %v", warnings)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	r, warnings, err = Restore(data, refs)
	if err != nil {
		t.Fatalf("a removed mod must still restore: %v", err)
	}
	if got := r.Units[game.Tank].Cost; got != 400 {
		t.Errorf("a removed mod: tank cost %d, want the saved copy's 400", got)
	}
	if len(warnings) != 1 {
		t.Errorf("a removed mod: warnings %v, want one", warnings)
	}
}

// What -print-base-mod writes reads back as a mod that changes nothing, so
// every key it prints is one a values file accepts, in a form it accepts.
func TestBaseValuesReadBackUnchanged(t *testing.T) {
	text := Values(game.BaseRules(), "header\nsecond line")
	dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: manifest("m"), ValuesFile: text})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	if diff := Diff(game.BaseRules(), r); len(diff) != 0 {
		t.Errorf("the base values read back changed: %v", diff)
	}
	for _, want := range []string{"[units.tank]", "upkeep = 0.6", "offense = 3.5", "defense = 3.5"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

// A saved mod's name is checked before it becomes a path: world.json can be
// edited by hand, and "../x" would read a folder outside the mods folder.
func TestRestoreRefusesANameThatLeavesTheModsFolder(t *testing.T) {
	ref := game.ModRef{Name: "../x", Version: "1", Hash: "h", Files: map[string]string{
		ManifestFile: manifest("../x"),
	}}
	if _, _, err := Restore(t.TempDir(), []game.ModRef{ref}); err == nil || !strings.Contains(err.Error(), "may hold only") {
		t.Errorf("err = %v, want the name refused", err)
	}
}

// A saved list naming one mod twice is refused, as a reset's list is.
func TestRestoreRefusesAModSavedTwice(t *testing.T) {
	ref := game.ModRef{Name: "a", Version: "1", Hash: "h", Files: map[string]string{ManifestFile: manifest("a")}}
	if _, _, err := Restore(t.TempDir(), []game.ModRef{ref, ref}); err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("err = %v, want the duplicate refused", err)
	}
}

func hqMod(t *testing.T, body string) (*game.Rules, error) {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "hq", map[string]string{ManifestFile: manifest("hq"), ValuesFile: body})
	_, r, err := Check(dir)
	return r, err
}

func TestHQSectionChangesTheRulesAndDiff(t *testing.T) {
	r, err := hqMod(t, "[hq]\nprice_base = 6000\nbuild_per_turn = 10\ntank_bonus = 0.02\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.HQ.PriceBase != 6000 || r.HQ.BuildPerTurn != 10 || r.HQ.TankPerHQ != 2 {
		t.Errorf("rules HQ = %+v", r.HQ)
	}
	got := strings.Join(Diff(game.BaseRules(), r), "\n")
	want := "hq.tank_bonus: 0.01 -> 0.02\nhq.build_per_turn: 5 -> 10\nhq.price_base: 5000 -> 6000"
	if got != want {
		t.Errorf("diff:\n%s\nwant:\n%s", got, want)
	}
}

func TestHQSectionRefusesWhatItShould(t *testing.T) {
	for body, want := range map[string]string{
		"[hq]\nprice_cap = 100\n":        "price_cap 100 must be at least price_base (5000)",
		"[hq]\nbattle_loss_jitter = 0\n": "battle_loss_jitter 0 is outside 1 to 100",
		"[hq]\nbogus = 1\n":              "hq.bogus: unknown key",
		"[hq]\nprice_base = 1.5\n":       "hq.price_base: want a whole number",
	} {
		if _, err := hqMod(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want it to say %q", body, err, want)
		}
	}
}

func TestBaseValuesPrintTheHQSection(t *testing.T) {
	text := Values(game.BaseRules(), "h")
	for _, want := range []string{"[hq]\n", "tank_bonus = 0.01", "build_start = 5", "build_per_turn = 5",
		"battle_loss_min = 5", "battle_loss_jitter = 3", "invasion_loss_divisor = 5",
		"price_base = 5000", "price_per_turn = 75", "price_jitter = 300",
		"price_cap = 100000", "price_cap_jitter = 1000", "sabotage_points = 15"} {
		if !strings.Contains(text, want+"\n") && !strings.HasSuffix(want, "\n") {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}
