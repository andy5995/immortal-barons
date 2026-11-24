package mod

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// scriptRules checks a mod carrying script as its pirates.star.
func scriptRules(t *testing.T, script string) *game.Rules {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "pirates", map[string]string{
		ManifestFile: manifest("pirates"),
		PiratesFile:  script,
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// scriptWorld is a fixed-seed world playing r, with one exposed realm.
func scriptWorld(r *game.Rules, seed int64) (*game.World, *game.Empire) {
	w := game.NewWorldSeed(game.DefaultConfig(), seed)
	w.Rules = r
	e := w.AddHuman("e", "Empire")
	e.Protection = 0
	e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Agents, e.Gold = 90_000, 1_000_000, 90_000, 90_000, 90_000, 9_000_000
	return w, e
}

// captureWarnings sends script warnings to a buffer and forgets which mods
// have warned, for the length of the test.
func captureWarnings(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	warnMu.Lock()
	old := warnOut
	warnOut, warned = &buf, map[string]map[string]bool{}
	warnMu.Unlock()
	t.Cleanup(func() {
		warnMu.Lock()
		warnOut = old
		warnMu.Unlock()
	})
	return &buf
}

const raidJets = `
def raid(realm, factions):
    return [{"faction": 1, "take": "jets", "market": False}]
`

func TestScriptRaidTakesWhatItOrders(t *testing.T) {
	w, e := scriptWorld(scriptRules(t, raidJets), 1)
	w.PlayTurn(e, "2026-11-23")
	// 1,000,000/33 is past the cap, so the take is 24,000 plus the seed's
	// jitter roll.
	want := []game.PirateHit{{Faction: w.Pirates[1].Name, Slot: 1, Spoil: game.SpoilJets, Amount: 24_081}}
	if !reflect.DeepEqual(e.PirateHits, want) {
		t.Errorf("hits %+v, want %+v", e.PirateHits, want)
	}
}

func TestScriptEmptyListMeansNoRaid(t *testing.T) {
	w, e := scriptWorld(scriptRules(t, "def raid(realm, factions):\n    return []\n"), 1)
	for range 100 {
		w.PlayTurn(e, "2026-11-23")
	}
	if len(e.PirateHits) != 0 {
		t.Errorf("%d raids, want none", len(e.PirateHits))
	}
}

// A script that fails raids the built-in way, and says why once.
func TestFailingScriptFallsBack(t *testing.T) {
	for _, c := range []struct{ name, script, reason string }{
		{"step cap", "def raid(realm, factions):\n    for i in range(100000000):\n        pass\n    return []\n", "too many steps"},
		{"slot", "def raid(realm, factions):\n    return [{\"faction\": 99, \"take\": \"jets\"}]\n", "faction 99 is outside"},
		{"bombers", "def raid(realm, factions):\n    return [{\"faction\": 0, \"take\": \"bombers\"}]\n", `take "bombers" is not one of`},
		{"gold from market", "def raid(realm, factions):\n    return [{\"faction\": 0, \"take\": \"gold\", \"market\": True}]\n", "gold is not traded"},
		{"runtime error", "def raid(realm, factions):\n    return [factions[99]]\n", "pirates.star: line 2: list index 99"},
		{"not a list", "def raid(realm, factions):\n    return None\n", "must return a list"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := captureWarnings(t)
			w, e := scriptWorld(scriptRules(t, c.script), 1)
			plain, pe := scriptWorld(nil, 1)
			for range 50 {
				w.PlayTurn(e, "2026-11-23")
				plain.PlayTurn(pe, "2026-11-23")
			}
			if len(pe.PirateHits) == 0 || !reflect.DeepEqual(e.PirateHits, pe.PirateHits) {
				t.Errorf("raids differ from the built-in ones:\n%+v\n%+v", e.PirateHits, pe.PirateHits)
			}
			if got := out.String(); strings.Count(got, "\n") != 1 || !strings.Contains(got, c.reason) || !strings.Contains(got, `mod "pirates"`) {
				t.Errorf("warning %q, want one line naming the mod and %q", got, c.reason)
			}
		})
	}
}

// A script that fails a new way later is reported again, so a failure that
// hits only some realms is not hidden behind the first. Writing to a global is
// one such failure: the top level is frozen once it has run.
func TestScriptFailingANewWayIsReported(t *testing.T) {
	out := captureWarnings(t)
	w, e := scriptWorld(scriptRules(t, `
SEEN = {}
def raid(realm, factions):
    if day() > 1:
        SEEN[realm.name] = day()
    return [factions[99]]
`), 1)
	for range 3 {
		w.PlayTurn(e, "2026-11-23")
		w.GameDay++
	}
	got := out.String()
	if strings.Count(got, "\n") != 2 || !strings.Contains(got, "list index 99") || !strings.Contains(got, "frozen") {
		t.Errorf("want one line for each of the two failures, got:\n%s", got)
	}
}

// The warnings stop after ScriptWarnings different reasons, with a line
// saying so.
func TestScriptWarningsAreCapped(t *testing.T) {
	out := captureWarnings(t)
	w, e := scriptWorld(scriptRules(t, "def raid(realm, factions):\n    return [factions[90 + day()]]\n"), 1)
	for range ScriptWarnings + 5 {
		w.PlayTurn(e, "2026-11-23")
		w.GameDay++
	}
	got := out.String()
	if strings.Count(got, "\n") != ScriptWarnings+1 || !strings.Contains(got, "no more are reported") {
		t.Errorf("want %d reasons and a closing line, got:\n%s", ScriptWarnings, got)
	}
}

func TestCheckRefusesAScriptWithoutRaid(t *testing.T) {
	for _, c := range []struct{ name, script, want string }{
		{"no raid", "def attack(realm, factions):\n    return []\n", "must define raid(realm, factions)"},
		{"one parameter", "def raid(realm):\n    return []\n", "exactly two parameters"},
		{"syntax", "def raid(realm, factions)\n", "pirates.star:2:1: got newline"},
		{"while", "def raid(realm, factions):\n    while True:\n        pass\n", "while"},
		{"load", "load(\"x.star\", \"y\")\ndef raid(realm, factions):\n    return []\n", "load"},
		{"rand at top level", "x = rand(3)\ndef raid(realm, factions):\n    return []\n", "only inside raid()"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := writeMod(t, t.TempDir(), "m", map[string]string{ManifestFile: manifest("m"), PiratesFile: c.script})
			if _, _, err := Check(dir); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err %v, want %q", err, c.want)
			}
		})
	}
}

const randomRaids = `
SPOILS = ["troopers", "jets", "turrets", "tanks", "gold", "agents"]

def raid(realm, factions):
    if rand(4) != 0 or day() < 0:
        return []
    f = factions[rand(len(factions))]
    return [{"faction": f.slot, "take": SPOILS[rand(len(SPOILS))], "market": False}]
`

func TestScriptGameReplaysForASeed(t *testing.T) {
	r := scriptRules(t, randomRaids)
	play := func() []byte {
		w, e := scriptWorld(r, 7)
		for range 200 {
			w.PlayTurn(e, "2026-11-23")
		}
		if len(e.PirateHits) == 0 {
			t.Fatal("the script never raided")
		}
		// Founded is the wall-clock moment the realm was made, to the second,
		// which no seed decides: two plays a second boundary apart differ there
		// and nowhere else.
		e.Founded = ""
		b, err := json.Marshal([]any{e, w.Pirates})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if a, b := play(), play(); !bytes.Equal(a, b) {
		t.Error("the same seed played two different games")
	}
}

func TestRestoreRebuildsTheScript(t *testing.T) {
	dataDir := t.TempDir()
	dir := writeMod(t, dataDir, "pirates", map[string]string{ManifestFile: manifest("pirates"), PiratesFile: raidJets})
	_, refs, err := Resolve(dataDir, []string{"pirates"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	r, warnings, err := Restore(dataDir, refs)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("err %v, warnings %q", err, warnings)
	}
	w, e := scriptWorld(r, 1)
	w.PlayTurn(e, "2026-11-23")
	if len(e.PirateHits) != 1 || e.PirateHits[0].Slot != 1 || e.PirateHits[0].Spoil != game.SpoilJets {
		t.Errorf("hits %+v, want one jet raid by faction 1", e.PirateHits)
	}
	if got := Diff(game.BaseRules(), r); !reflect.DeepEqual(got, []string{"pirate raids: built in -> pirates/" + PiratesFile + " (" + r.Pirates.(*pirateScript).hash + ")"}) {
		t.Errorf("diff %q", got)
	}
}

// Two mods carrying the same script are allowed, the later one's running; the
// editors are told which one does not run.
func TestReplacedNamesTheScriptThatDoesNotRun(t *testing.T) {
	data := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		files := map[string]string{ManifestFile: manifest(name)}
		if name != "c" {
			files[PiratesFile] = "def raid(realm, factions):\n    return []\n"
			files["regions/desert.star"] = "def region(realm, row):\n    return {}\n"
		}
		writeMod(t, data, name, files)
	}
	got := Replaced(data, []string{"a", "c", "b"})
	want := []Replacement{
		{File: PiratesFile, By: "b", Over: "a"},
		{File: "regions/desert.star", By: "b", Over: "a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Replaced = %q, want %q", got, want)
	}
	if got := Replaced(data, []string{"a", "c"}); len(got) != 0 {
		t.Errorf("one script each: Replaced = %q, want none", got)
	}
}
