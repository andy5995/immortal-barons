package mod

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func checkValues(t *testing.T, manifestExtra, values string) (*game.Rules, error) {
	t.Helper()
	dir := writeMod(t, t.TempDir(), "atk", map[string]string{
		ManifestFile: manifest("atk") + manifestExtra,
		ValuesFile:   values,
	})
	_, r, err := Check(dir)
	return r, err
}

func TestAttackSectionsChangeTheRules(t *testing.T) {
	r, err := checkValues(t, "", `
[attacks.nuclear.target]
waste = [10, 2, 2]
kill_turret = 30
capture = 5

[attacks.nuclear.attacker]
people = 2
morale = [3, 4]
`)
	if err != nil {
		t.Fatal(err)
	}
	fx := r.Attacks[game.EffectNuclear]
	if got := fx.Target.Waste; got != (game.Roll{Pct: 10, Up: 2, Down: 2}) {
		t.Errorf("waste %+v, want 10 up 2 down 2", got)
	}
	if got := fx.Target.Kill[game.KillIndex(game.Turret)]; got != (game.Roll{Pct: 30}) {
		t.Errorf("turret kill %+v, want a flat 30", got)
	}
	if fx.Target.Capture.Pct != 5 || fx.Attacker.People.Pct != 2 || fx.Attacker.Morale != (game.Fraction{Num: 3, Den: 4}) {
		t.Errorf("effects not read: %+v", fx)
	}
	want := strings.Join([]string{
		"attacks.nuclear.target.waste: [7, 3, 3] -> [10, 2, 2]",
		"attacks.nuclear.target.capture: 0 -> 5",
		"attacks.nuclear.target.kill_turret: 0 -> 30",
		"attacks.nuclear.attacker.people: 0 -> 2",
		"attacks.nuclear.attacker.morale: [1, 1] -> [3, 4]",
	}, "\n")
	if got := strings.Join(Diff(game.BaseRules(), r), "\n"); got != want {
		t.Errorf("diff:\n%s\nwant:\n%s", got, want)
	}
}

func TestAttackValuesAreRefusedWithAClearMessage(t *testing.T) {
	cases := []struct{ name, values, want string }{
		{"percent over 100", "[attacks.nuclear.target]\nwaste = 101\n", "attacks.nuclear.target.waste: percent 101 is outside 0 to 100"},
		{"jitter over 100", "[attacks.chemical.target]\npeople = [5, 101, 0]\n", "attacks.chemical.target.people: up 101 and down 0 must each be 0 to 100"},
		{"hq over 100", "[attacks.nuclear.attacker]\nhq = 150\n", "attacks.nuclear.attacker.hq: percent 150 is outside 0 to 100"},
		{"numerator over denominator", "[attacks.biological.target]\nmorale = [5, 4]\n", "numerator 5 is outside 0 to the denominator (4)"},
		{"zero denominator", "[attacks.biological.target]\nsupport = [0, 0]\n", "denominator 0 is outside 1 to 1000"},
		{"fraction as number", "[attacks.nuclear.target]\nmorale = 2\n", "want [num, den]"},
		{"roll of wrong length", "[attacks.nuclear.target]\nwaste = [1, 2]\n", "want a whole number or [pct, up, down]"},
		{"unknown key", "[attacks.nuclear.target]\nburn = 1\n", "attacks.nuclear.target.burn: unknown key"},
		{"unknown kind", "[attacks.laser.target]\nwaste = 1\n", "attacks.laser: not an attack a mod can change (nuclear, chemical, biological, conventional.won, conventional.lost, ip.attack.won, ip.attack.lost, ip.nuclear, ip.chemical, sabre.intelligence, sabre.people, sabre.military_bases, sabre.airbases, sabre.regions, sabre.food)"},
		{"attacker on a remote missile", "[attacks.ip.chemical.attacker]\npeople = 1\n", "attacks.ip.chemical.attacker: this missile comes from another board"},
		{"capture on a remote missile", "[attacks.sabre.food.target]\ncapture = 1\n", "attacks.sabre.food.target.capture: the attacker is on another board"},
		{"capture in an ip attack's attacker set", "[attacks.ip.attack.won.attacker]\ncapture = 1\n", "attacks.ip.attack.won.attacker.capture: the target is on another board"},
		{"keys outside a side", "[attacks.nuclear]\nwaste = 1\n", "effects go in [attacks.<kind>.target]"},
	}
	for _, c := range cases {
		if _, err := checkValues(t, "", c.values); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want it to contain %q", c.name, err, c.want)
		}
	}
	// Every mistake is reported at once.
	_, err := checkValues(t, "", "[attacks.nuclear.target]\nwaste = 101\npeople = 101\n")
	if err == nil || !strings.Contains(err.Error(), "waste") || !strings.Contains(err.Error(), "people") {
		t.Errorf("want both errors at once, got %v", err)
	}
}

// An [attacks] table belongs in values.toml; in mod.toml it is an unknown key,
// not something read nowhere.
func TestAttackTableInTheManifestIsRefused(t *testing.T) {
	_, err := checkValues(t, "\n[attacks.nuclear.target]\nwaste = 50\n", "")
	if err == nil || !strings.Contains(err.Error(), "attacks.nuclear.target.waste") {
		t.Fatalf("err %v, want the manifest's attacks key refused", err)
	}
}

func TestAttackKeysTakeOptionsAndWhenSections(t *testing.T) {
	opts := `
[options.fallout]
type = "number"
label = "Fallout"
default = 4
min = 0
max = 10

[options.harsh]
type = "bool"
label = "Harsh"
default = false
`
	values := `
[attacks.nuclear.target]
kill_turret = { option = "fallout" }

[when.harsh.attacks.nuclear.target]
destroy = 3
`
	r, err := checkValues(t, opts, values)
	if err != nil {
		t.Fatal(err)
	}
	fx := r.Attacks[game.EffectNuclear].Target
	if fx.Kill[game.KillIndex(game.Turret)].Pct != 4 || fx.Destroy.Pct != 0 {
		t.Errorf("turret %d destroy %d, want 4 and 0 with harsh off", fx.Kill[game.KillIndex(game.Turret)].Pct, fx.Destroy.Pct)
	}
	dir := writeMod(t, t.TempDir(), "atk", map[string]string{ManifestFile: manifest("atk") + opts, ValuesFile: values})
	_, r, err = CheckWithOptions(dir, map[string]int64{"harsh": 1, "fallout": 9})
	if err != nil {
		t.Fatal(err)
	}
	fx = r.Attacks[game.EffectNuclear].Target
	if fx.Kill[game.KillIndex(game.Turret)].Pct != 9 || fx.Destroy.Pct != 3 {
		t.Errorf("turret %d destroy %d, want 9 and 3", fx.Kill[game.KillIndex(game.Turret)].Pct, fx.Destroy.Pct)
	}
}

func TestPrintedBaseModReadsBackAsTheBase(t *testing.T) {
	text := Values(game.BaseRules(), "base")
	for _, want := range []string{"[attacks.nuclear.target]\nwaste = [7, 3, 3]", "[attacks.biological.target]", "kill_trooper = [15, 6, 4]", "morale = [3, 4]", "[attacks.chemical.attacker]"} {
		if !strings.Contains(text, want) {
			t.Errorf("printed base mod lacks %q", want)
		}
	}
	r, err := checkValues(t, "", text)
	if err != nil {
		t.Fatalf("the printed base mod does not load: %v", err)
	}
	if d := Diff(game.BaseRules(), r); len(d) != 0 {
		t.Errorf("the printed base mod differs from the base: %v", d)
	}
}

func TestConventionalKindsTakeADottedSection(t *testing.T) {
	r, err := checkValues(t, "", `
[attacks.conventional.won.target]
waste = 5
kill_tank = 10

[attacks.conventional.lost.attacker]
people = 2
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Attacks[game.EffectConventionalWon].Target; got.Waste.Pct != 5 || got.Kill[game.KillIndex(game.Tank)].Pct != 10 {
		t.Errorf("won target %+v", got)
	}
	if got := r.Attacks[game.EffectConventionalLost].Attacker.People.Pct; got != 2 {
		t.Errorf("lost attacker people %d, want 2", got)
	}
	text := Values(game.BaseRules(), "base")
	for _, want := range []string{"[attacks.conventional.won.target]", "[attacks.conventional.lost.attacker]"} {
		if !strings.Contains(text, want) {
			t.Errorf("printed base mod lacks %q", want)
		}
	}
}

func TestInterplanetaryAttackKindsLoad(t *testing.T) {
	r, err := checkValues(t, "", `
[attacks.ip.attack.won.target]
capture = 5
kill_tank = 10

[attacks.ip.attack.lost.attacker]
people = 2
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Attacks[game.EffectIPAttackWon].Target; got.Capture.Pct != 5 || got.Kill[game.KillIndex(game.Tank)].Pct != 10 {
		t.Errorf("won target %+v", got)
	}
	if got := r.Attacks[game.EffectIPAttackLost].Attacker.People.Pct; got != 2 {
		t.Errorf("lost attacker people %d, want 2", got)
	}
}
