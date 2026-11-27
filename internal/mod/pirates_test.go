package mod

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/andy5995/immortal-barons/internal/game"
)

const nineNames = `names = ["A", "B", "C", "D", "E", "F", "G", "H", "I"]`

func TestPiratesSectionChangesTheRulesAndDiff(t *testing.T) {
	r, err := hqMod(t, "[pirates]\nraid_take_divisor = 10\ncap_gold = 1000\nseed_gold_base = 7\nattacker_loss_min = 9\n"+nineNames+"\n")
	if err != nil {
		t.Fatal(err)
	}
	p := r.PirateFigures
	if p.RaidTakeDivisor != 10 || p.CapGold != 1000 || p.SeedGoldBase != 7 || p.AttackerLossMin != 9 || p.Names[8] != "I" {
		t.Errorf("rules pirates = %+v", p)
	}
	got := Diff(game.BaseRules(), r)
	for _, want := range []string{
		"pirates.raid_take_divisor: 33 -> 10",
		"pirates.cap_gold: 600000000 -> 1000",
		"pirates.seed_gold_base: 500000 -> 7",
		"pirates.attacker_loss_min: 2 -> 9",
		`pirates.names: ["Humans", "Barbarians", "Nightjackals", "Sharks", "Dunkleoids", "Trilobarians", "Raptorians", "Gorgonoids", "Ammonians"] -> ["A", "B", "C", "D", "E", "F", "G", "H", "I"]`,
	} {
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("diff lacks %q:\n%s", want, strings.Join(got, "\n"))
		}
	}
	if len(got) != 5 {
		t.Errorf("diff has %d lines, want 5:\n%s", len(got), strings.Join(got, "\n"))
	}
}

func TestPiratesSectionRefusesWhatItShould(t *testing.T) {
	for body, want := range map[string]string{
		"[pirates]\nraid_take_divisor = 0\n":     "raid_take_divisor 0 is outside 1 to 1000000",
		"[pirates]\ncap_gold = 1000000000001\n":  "cap_gold 1000000000001 is outside 0 to 1000000000000",
		"[pirates]\nraid_chance_out_of = 1001\n": "raid_chance_out_of 1001 is outside 1 to 1000",
		"[pirates]\nbogus = 1\n":                 "pirates.bogus: unknown key",
		"[pirates]\ncap_jets = 1.5\n":            "pirates.cap_jets: want a whole number",
		"[pirates]\nattacker_loss_min = 99\n":    "attacker_loss_min + attacker_loss_jitter may not exceed 101",
		"[pirates]\nnames = [\"A\"]\n":           "names: 1 given, there are 9",
		"[pirates]\nnames = \"A\"\n":             "pirates.names: want a list of names",
		"[pirates]\nnames = [\"Рейдеры\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n": "CP437",
		"[pirates]\nnames = [1, 2]\n": "pirates.names: name 0 is not text",
		"[pirates]\nnames = [\"a\\u001b[2J\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n":                     "holds a control or format character",
		"[pirates]\nnames = [\"a\\u202eb\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n":                       "CP437",
		"[pirates]\nnames = [\"" + strings.Repeat("x", 21) + "\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n": "longer than 20 characters",
	} {
		if _, err := hqMod(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want it to say %q", body, err, want)
		}
	}
}

// The refusal for an unsafe name never prints the raw character.
func TestPirateNameErrorIsCleaned(t *testing.T) {
	_, err := hqMod(t, "[pirates]\nnames = [\"a\\u001b[2J\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n")
	if err == nil || strings.ContainsRune(err.Error(), 0x1b) {
		t.Errorf("err = %q holds the escape character", err)
	}
	if _, err := hqMod(t, "[pirates]\nnames = [\"é\", \"B\", \"C\", \"D\", \"E\", \"F\", \"G\", \"H\", \"I\"]\n"); err != nil {
		t.Errorf("a plain accented name was refused: %v", err)
	}
}

func TestBaseValuesPrintThePiratesSection(t *testing.T) {
	text := Values(game.BaseRules(), "h")
	for _, want := range []string{"[pirates]\n", "raid_chance_out_of = 20\n", "raid_chance_base = 2\n",
		"raid_chance_cap = 6\n", "raid_regions_per_step = 1200\n", "raid_take_divisor = 33\n",
		"raid_take_cap = 24000\n", "raid_take_cap_jitter = 1000\n", "raid_keep_multiple = 3\n",
		"raid_land_max = 25\n", "cap_gold = 600000000\n", "cap_regions = 300\n", "cap_agents = 200000\n",
		"cap_troopers = 300000\n", "cap_jets = 400000\n", "cap_turrets = 400000\n", "cap_tanks = 200000\n",
		"seed_troopers = 7000\n", "seed_tanks = 4000\n", "seed_jets = 7000\n", "seed_turrets = 9000\n",
		"seed_regions = 75\n", "seed_gold_base = 500000\n", "seed_gold_roll = 300\n",
		"attacker_loss_min = 2\n", "attacker_loss_jitter = 5\n", "defender_loss_min = 4\n",
		"defender_loss_jitter = 10\n", "reclaim_divisor_main = 3\n", "reclaim_divisor_armour = 4\n",
		`names = ["Humans", "Barbarians", "Nightjackals", "Sharks", "Dunkleoids", "Trilobarians", "Raptorians", "Gorgonoids", "Ammonians"]` + "\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

// What -print-base-mod prints reads back to the same rules.
func TestPrintedBaseValuesReadBack(t *testing.T) {
	r, err := hqMod(t, Values(game.BaseRules(), "h"))
	if err != nil {
		t.Fatal(err)
	}
	if d := Diff(game.BaseRules(), r); len(d) != 0 {
		t.Errorf("the printed base differs from the base: %v", d)
	}
}

func TestPiratesTakeOptionsAndWhenSections(t *testing.T) {
	data := t.TempDir()
	writeMod(t, data, "m", map[string]string{
		ManifestFile: optionManifest("m", `[options.n]
label = "N"
type = "number"
default = 12
min = 1
max = 100
[options.on]
label = "On"
type = "bool"
default = false
`),
		ValuesFile: "[pirates]\nraid_take_divisor = { option = \"n\" }\n[when.on.pirates]\ncap_tanks = 5\n",
	})
	r, _, err := Resolve(data, []string{"m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.PirateFigures.RaidTakeDivisor != 12 || r.PirateFigures.CapTanks != 200_000 {
		t.Errorf("off: %+v", r.PirateFigures)
	}
	r, _, err = Resolve(data, []string{"m"}, map[string]map[string]int64{"m": {"on": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if r.PirateFigures.CapTanks != 5 {
		t.Errorf("on: cap_tanks %d, want 5", r.PirateFigures.CapTanks)
	}
}

const raidSized = `
def raid(realm, factions):
    return [{"faction": 1, "take": "jets", "share": 50, "land": 7}]
`

// A script's share and land reach the raid through the worker.
func TestScriptRaidShareAndLandReachTheRaid(t *testing.T) {
	r := scriptRules(t, raidSized)
	w, e := scriptWorld(r, 1)
	land := w.Pirates[1].Land
	w.PlayTurn(e, "2026-11-23")
	// 1,000,000 jets is past the cap: 24,000 plus the seed's jitter, halved.
	want := []game.PirateHit{{Faction: w.Pirates[1].Name, Slot: 1, Spoil: game.SpoilJets, Amount: 12_040}}
	if len(e.PirateHits) != 1 || e.PirateHits[0] != want[0] {
		t.Errorf("hits %+v, want %+v", e.PirateHits, want)
	}
	if got := w.Pirates[1].Land - land; got != 7 {
		t.Errorf("land granted %d, want 7", got)
	}
}

func TestReadOrdersSizeKeys(t *testing.T) {
	order := func(kv ...starlark.Tuple) starlark.Value {
		d := starlark.NewDict(4)
		d.SetKey(starlark.String("faction"), starlark.MakeInt(0))
		d.SetKey(starlark.String("take"), starlark.String("gold"))
		for _, p := range kv {
			d.SetKey(p[0], p[1])
		}
		return starlark.NewList([]starlark.Value{d})
	}
	kv := func(k string, v starlark.Value) starlark.Tuple { return starlark.Tuple{starlark.String(k), v} }

	got, err := readOrders(order(kv("share", starlark.MakeInt(25)), kv("land", starlark.MakeInt(0))))
	if err != nil || *got[0].Share != 25 || *got[0].Land != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
	got, err = readOrders(order())
	if err != nil || got[0].Share != nil || got[0].Land != nil {
		t.Fatalf("absent keys read as %+v, err %v", got, err)
	}
	for key, v := range map[string]starlark.Value{
		"share": starlark.MakeInt(101), "land": starlark.MakeInt(game.ModPirateRaidLandMax + 1),
	} {
		if _, err := readOrders(order(kv(key, v))); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s out of range: err %v", key, err)
		}
	}
	if _, err := readOrders(order(kv("share", starlark.String("half")))); err == nil || !strings.Contains(err.Error(), "share") {
		t.Errorf("a text share: err %v", err)
	}
}
