package mod

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestCombatSectionChangesTheRulesAndDiff(t *testing.T) {
	r, err := hqMod(t, "[combat]\ncapture_high = 40\nip_capture_high = 20\nretreat_low = 5\ncapture_floor = 20\nscore_win = 90\nscore_crush = 200\n")
	if err != nil {
		t.Fatal(err)
	}
	c := r.Combat
	if c.Capture.High != 40 || c.InterplanetaryCapture.High != 20 || c.Retreat.Low != 5 ||
		c.CaptureFloor != 20 || c.ScoreWin != 90 || c.ScoreCrush != 200 {
		t.Errorf("rules combat = %+v", c)
	}
	got := strings.Join(Diff(game.BaseRules(), r), "\n")
	want := "combat.capture_high: 25 -> 40\ncombat.ip_capture_high: 15 -> 20\ncombat.retreat_low: 10 -> 5\n" +
		"combat.capture_floor: 15 -> 20\ncombat.score_win: 82 -> 90\ncombat.score_crush: 192 -> 200"
	// Diff lists in the table's order, which is the order -print-base-mod writes.
	for _, line := range strings.Split(want, "\n") {
		if !strings.Contains(got, line) {
			t.Errorf("diff lacks %q:\n%s", line, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 6 {
		t.Errorf("diff has %d lines, want 6:\n%s", n, got)
	}
}

func TestCombatSectionRefusesWhatItShould(t *testing.T) {
	for body, want := range map[string]string{
		"[combat]\ncapture_low = 101\n":   "capture_low 101 is outside 0 to 100",
		"[combat]\nretreat_none = 0\n":    "retreat_none 0 is outside 1 to 100",
		"[combat]\nscore_win = 100001\n":  "score_win 100001 is outside 0 to 100000",
		"[combat]\nbogus = 1\n":           "combat.bogus: unknown key",
		"[combat]\ncapture_floor = 1.5\n": "combat.capture_floor: want a whole number",
	} {
		if _, err := hqMod(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want it to say %q", body, err, want)
		}
	}
}

func TestBaseValuesPrintTheCombatSection(t *testing.T) {
	text := Values(game.BaseRules(), "h")
	for _, want := range []string{"[combat]\n", "capture_none = 0\n", "capture_high = 25\n",
		"ip_capture_high = 15\n", "retreat_none = 1\n", "retreat_high = 30\n",
		"capture_floor = 15\n", "score_win = 82\n", "score_crush = 192\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

// An option reference and a when section both reach [combat].
func TestCombatTakesOptionsAndWhenSections(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", `[options.n]
label = "N"
type = "number"
default = 33
min = 1
max = 100
[options.on]
label = "On"
type = "bool"
default = false
`),
		ValuesFile: "[combat]\ncapture_medium = { option = \"n\" }\n[when.on.combat]\nscore_win = 150\n",
	})
	r, _, err := Resolve(data, []string{"m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Combat.Capture.Medium != 33 || r.Combat.ScoreWin != 82 {
		t.Errorf("off: capture_medium %d, score_win %d, want 33 and 82", r.Combat.Capture.Medium, r.Combat.ScoreWin)
	}
	r, _, err = Resolve(data, []string{"m"}, map[string]map[string]int64{"m": {"on": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Combat.ScoreWin != 150 {
		t.Errorf("on: score_win %d, want 150", r.Combat.ScoreWin)
	}
}
