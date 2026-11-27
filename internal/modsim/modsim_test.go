package modsim

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeMod(t *testing.T, values string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "m")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	man := "name = \"m\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n"
	for f, body := range map[string]string{"mod.toml": man, "values.toml": values} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func opts(dir string) Options {
	return Options{ModPath: dir, Days: 10, Seeds: 4, Realms: 6}
}

func row(t *testing.T, r *Report, metric string, day int) Row {
	t.Helper()
	for _, x := range r.Rows {
		if x.Metric == metric && x.Day == day {
			return x
		}
	}
	t.Fatalf("no row %q on day %d", metric, day)
	return Row{}
}

// A mod that changes nothing has a delta of exactly zero in every row.
func TestEmptyModDeltaIsZero(t *testing.T) {
	r, err := Run(opts(writeMod(t, "")))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) == 0 {
		t.Fatal("no rows")
	}
	for _, x := range r.Rows {
		if x.Delta() != 0 {
			t.Errorf("%s day %d: delta %v, want 0", x.Metric, x.Day, x.Delta())
		}
	}
}

func TestRunIsDeterministic(t *testing.T) {
	dir := writeMod(t, "[units.tank]\nbuild_cost = 5000\n")
	a, err := Run(opts(dir))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(opts(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Rows, b.Rows) {
		t.Error("two runs of the same mod and seeds differ")
	}
}

// Every realm starts with troopers, so a heavier net-worth weight on them
// raises the median realm's net worth whatever the seed.
func TestUnitNetWorthMovesNetWorthUp(t *testing.T) {
	dir := writeMod(t, "[units.trooper]\nnetworth = 5\n")
	r, err := Run(opts(dir))
	if err != nil {
		t.Fatal(err)
	}
	x := row(t, r, "net worth median", 10)
	if x.Delta() <= 0 {
		t.Errorf("raising a unit's net worth moved median net worth by %v; want a rise", x.Delta())
	}
}

func TestNoFilesWritten(t *testing.T) {
	dir := writeMod(t, "")
	work := t.TempDir()
	t.Chdir(work)
	if _, err := Run(opts(dir)); err != nil {
		t.Fatal(err)
	}
	if ents, _ := os.ReadDir(work); len(ents) != 0 {
		t.Errorf("run wrote %d entries into the working directory", len(ents))
	}
}

func TestOutputsRender(t *testing.T) {
	r, err := Run(opts(writeMod(t, "")))
	if err != nil {
		t.Fatal(err)
	}
	if r.Text() == "" || r.CSV() == "" {
		t.Error("empty output")
	}
}

func writeOptionMod(t *testing.T) string {
	t.Helper()
	dir := writeMod(t, "")
	man := "name = \"m\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n" +
		"[options.cost]\ntype = \"number\"\nlabel = \"Cost\"\ndescription = \"d\"\ndefault = 5\nmin = 1\nmax = 10\n" +
		"[options.fast]\ntype = \"bool\"\nlabel = \"Fast\"\ndescription = \"d\"\ndefault = false\n"
	if err := os.WriteFile(filepath.Join(dir, "mod.toml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOptionChoicesAreTypeChecked(t *testing.T) {
	dir := writeOptionMod(t)
	for _, c := range []map[string]string{
		{"cost": "true"}, {"fast": "1"}, {"fast": "maybe"}, {"nope": "1"}, {"cost": "x"},
	} {
		o := opts(dir)
		o.Choices = c
		if _, err := Run(o); err == nil {
			t.Errorf("choices %v were accepted", c)
		}
	}
	o := opts(dir)
	o.Choices = map[string]string{"cost": "7", "fast": "yes"}
	r, err := Run(o)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Text(); !strings.Contains(got, "Cost (cost): 7") || !strings.Contains(got, "Fast (fast): ") {
		t.Errorf("header lacks the options:\n%s", got)
	}
	if !strings.Contains(r.CSV(), "# option,Cost (cost): 7") {
		t.Error("CSV lacks the options")
	}
}

func TestLimitsAreRefused(t *testing.T) {
	dir := writeMod(t, "")
	for _, o := range []Options{
		{ModPath: dir, Days: 5, Seeds: 1, Realms: MaxRealms + 1},
		{ModPath: dir, Days: MaxDays + 1, Seeds: 1, Realms: 2},
		{ModPath: dir, Days: 5, Seeds: MaxSeeds + 1, Realms: 2},
	} {
		if _, err := Run(o); err == nil {
			t.Errorf("%+v was accepted", o)
		}
	}
}

// A label is the world's own day: the last sample is the day asked for.
func TestSamplesAreLabeledByGameDay(t *testing.T) {
	for _, days := range []int{3, 10} {
		o := opts(writeMod(t, ""))
		o.Days = days
		r, err := Run(o)
		if err != nil {
			t.Fatal(err)
		}
		if last := r.SampleDay[len(r.SampleDay)-1]; last != days {
			t.Errorf("days %d: last sample %d", days, last)
		}
		w := game.NewWorldSeed(game.DefaultConfig(), 1)
		w.AddAIEmpires(2)
		for i := range days + 1 {
			w.DailyMaintenance(fmt.Sprintf("2026-02-%02d", i+1))
		}
		if w.GameDay != days {
			t.Errorf("days+1 maintenances give GameDay %d, want %d", w.GameDay, days)
		}
	}
}

func TestInterBBSKinds(t *testing.T) {
	for k, want := range map[game.EffectKind]bool{
		game.EffectConventionalWon: false, game.EffectNuclear: false,
		game.EffectIPAttackWon: true, game.EffectIPNuclear: true, game.EffectSabreFood: true,
	} {
		if game.InterBBSKind(k) != want {
			t.Errorf("%s: got %v", k, !want)
		}
	}
}
// Script failures are reported once per run, so a second Run in the same
// process must report a failing script again rather than find it already told.
func TestEveryRunReportsItsScriptFailures(t *testing.T) {
	dir := writeMod(t, "")
	src := "def raid(realm, factions):\n    fail(\"no raid today\")\n"
	if err := os.WriteFile(filepath.Join(dir, "pirates.star"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		r, err := Run(opts(dir))
		if err != nil {
			t.Fatal(err)
		}
		if len(r.ScriptFailures) == 0 {
			t.Errorf("run %d reported no script failures", run)
		}
	}
}
