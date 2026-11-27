// Package modsim plays a mod against the base game in throwaway worlds and
// reports the difference (#374). Every world lives in memory: nothing here
// reads or writes a data directory, so it is safe to run on a board's machine.
//
// The computer barons play by their own code and do not adapt to a mod, so the
// figures show what a change does mechanically, not how a person would use it.
package modsim

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

// Defaults for a run. The day count is a balance question the author answers;
// these are what keeps a first run to a few seconds.
const (
	DefaultDays   = 60
	DefaultSeeds  = 8
	DefaultRealms = 8
	// samplePoints is how many times over the run the figures are read.
	samplePoints = 5
	// The most a run may ask for. MaxRealms is the planet's slot count: more
	// would be silently cut short by AddAIEmpires.
	MaxDays   = 3650
	MaxSeeds  = 1000
	MaxRealms = game.PlanetSlots
	// startDate is the synthetic first day; no system clock is involved.
	startDate = "2026-01-01"
)

// Options is one comparison.
type Options struct {
	ModPath string
	// Choices are the mod's option values by id, as -mod-option writes them:
	// true, false, on, off, yes or no for a bool and digits for a number.
	Choices map[string]string
	Days    int
	Seeds   int
	Realms  int
}

// Stat is one figure across the seeds.
type Stat struct{ Mean, Min, Max float64 }

// Row is one metric at one sampled day.
type Row struct {
	Metric string
	Day    int
	Count  bool // a whole number, not a ratio
	Base   Stat
	Mod    Stat
}

// Delta is the mod's mean less the base game's.
func (r Row) Delta() float64 { return r.Mod.Mean - r.Base.Mean }

// Percent is Delta as a share of the base mean; false when the base is zero.
func (r Row) Percent() (float64, bool) {
	if r.Base.Mean == 0 {
		return 0, false
	}
	return 100 * r.Delta() / r.Base.Mean, true
}

// Report is the result of a comparison.
type Report struct {
	ModName string
	// ModVersion and ModHash say which copy of the mod was played.
	ModVersion, ModHash string
	// Options is every option's chosen value, defaults included, one line each.
	Options   []string
	Seeds     int
	Days      int
	Realms    int
	SampleDay []int
	Rows      []Row
	// Changes is what the mod changes from the base game, as -check-mod lists it.
	Changes []string
	// OutOfScope names changed inter-BBS kinds, which one board cannot play.
	OutOfScope []string
	// ScriptFailures are the distinct reasons a mod script failed and the game
	// fell back to the built-in rules.
	ScriptFailures []string
	Elapsed        time.Duration
}

// metricNames is the ordered metric set; snapshot returns values in this order.
func metricNames() []string {
	names := []string{"realms alive", "eliminated", "net worth top", "net worth median", "net worth top/median",
		"regions top", "regions median", "gold (hand+bank)"}
	for _, g := range game.AllGoods {
		names = append(names, strings.ToLower(g.Plural))
	}
	return names
}

// parseChoices turns -mod-option text into the mod's stored values, checking
// each against the type the mod declares for it.
func parseChoices(m *mod.Mod, raw map[string]string) (map[string]int64, error) {
	out := map[string]int64{}
	for id, text := range raw {
		o := m.Manifest.Options[id]
		if o == nil {
			return nil, fmt.Errorf("the mod has no option %q (it has: %s)", id, strings.Join(m.SortedOptionIDs(), ", "))
		}
		switch o.Type {
		case mod.OptionBool:
			switch strings.ToLower(text) {
			case "true", "on", "yes":
				out[id] = 1
			case "false", "off", "no":
				out[id] = 0
			default:
				return nil, fmt.Errorf("option %s is a bool: use true, false, on, off, yes or no, not %q", id, text)
			}
		default:
			n, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("option %s is a number: use digits, not %q", id, text)
			}
			out[id] = n
		}
	}
	return out, nil
}

func stderr() io.Writer { return os.Stderr }

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// snapshot reads the living realms in metricNames order.
func snapshot(w *game.World, start int) []float64 {
	var nw, land []float64
	var gold float64
	units := make([]float64, len(game.AllGoods))
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		nw = append(nw, float64(w.NetWorth(e)))
		land = append(land, float64(e.Land))
		gold += float64(e.Gold + e.Bank)
		for i, g := range game.AllGoods {
			units[i] += float64(*g.Count(e))
		}
	}
	top := func(v []float64) float64 {
		m := 0.0
		for _, x := range v {
			m = max(m, x)
		}
		return m
	}
	ratio := 0.0
	if m := median(nw); m > 0 {
		ratio = top(nw) / m
	}
	out := []float64{float64(len(nw)), float64(start - len(nw)), top(nw), median(nw), ratio,
		top(land), median(land), gold}
	return append(out, units...)
}

func sampleDays(days int) []int {
	var out []int
	for i := 1; i <= samplePoints; i++ {
		d := days * i / samplePoints
		if d >= 1 && (len(out) == 0 || out[len(out)-1] != d) {
			out = append(out, d)
		}
	}
	return out
}

// playOne plays one world forward and reads it at each sample day. rules nil is
// the base game. The same seed gives the same start under either rule set.
func playOne(rules *game.Rules, seed int64, days, realms int, sample []int) [][]float64 {
	w := game.NewWorldSeed(game.DefaultConfig(), seed)
	w.Rules = rules
	w.Reset()
	w.AddAIEmpires(realms)
	start := 0
	for _, e := range w.Empires {
		if e.Alive {
			start++
		}
	}
	date, _ := time.Parse("2006-01-02", startDate)
	var out [][]float64
	next := 0
	// The first maintenance of a fresh game plays the opening turns without
	// counting a day, so the label is the world's own GameDay, not a loop count.
	for n := 0; n <= days+1 && next < len(sample); n++ {
		date = date.AddDate(0, 0, 1)
		w.DailyMaintenance(date.Format("2006-01-02"))
		if w.GameDay == sample[next] {
			out = append(out, snapshot(w, start))
			next++
		}
	}
	return out
}

// Run plays the comparison. Seeds run in parallel, bounded by GOMAXPROCS. Script
// failures are collected through the mod package's process-wide warning output,
// so two Runs must not overlap.
func Run(o Options) (*Report, error) {
	if o.Days < 1 || o.Seeds < 1 || o.Realms < 1 {
		return nil, errors.New("days, seeds and realms must each be at least 1")
	}
	began := time.Now()
	if o.Days > MaxDays || o.Seeds > MaxSeeds || o.Realms > MaxRealms {
		return nil, fmt.Errorf("at most %d days, %d seeds and %d realms", MaxDays, MaxSeeds, MaxRealms)
	}
	declared, err := mod.Read(o.ModPath)
	if err != nil {
		return nil, err
	}
	choices, err := parseChoices(declared, o.Choices)
	if err != nil {
		return nil, err
	}
	m, rules, err := mod.CheckWithOptions(o.ModPath, choices)
	if err != nil {
		return nil, err
	}
	var warned bytes.Buffer
	mod.ResetWarnings()
	mod.SetWarnOutput(&warned)
	defer mod.SetWarnOutput(stderr())

	rep := &Report{ModName: m.Ref().DisplayName(), ModVersion: m.Version, ModHash: m.Hash, Seeds: o.Seeds, Days: o.Days, Realms: o.Realms,
		SampleDay: sampleDays(o.Days), Changes: mod.Diff(game.BaseRules(), rules)}
	for _, id := range m.SortedOptionIDs() {
		o := m.Manifest.Options[id]
		rep.Options = append(rep.Options, fmt.Sprintf("%s (%s): %s", o.Label, id, mod.OptionValueWord(o, m.Chosen(id))))
	}
	for _, c := range rep.Changes {
		for _, k := range game.EffectKinds() {
			if game.InterBBSKind(k) && strings.HasPrefix(c, "attacks."+string(k)+".") {
				rep.OutOfScope = append(rep.OutOfScope, c)
			}
		}
	}

	// Each job gets its own rules: a script's state is then never shared
	// between worlds running at once.
	type result struct{ base, mod [][]float64 }
	results := make([]result, o.Seeds)
	errs := make([]error, o.Seeds)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := range o.Seeds {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			_, r, err := mod.CheckWithOptions(o.ModPath, choices)
			if err != nil {
				errs[i] = err
				return
			}
			seed := int64(i + 1)
			results[i] = result{
				base: playOne(nil, seed, o.Days, o.Realms, rep.SampleDay),
				mod:  playOne(r, seed, o.Days, o.Realms, rep.SampleDay),
			}
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	names := metricNames()
	for ni, name := range names {
		for si, day := range rep.SampleDay {
			var b, mo []float64
			for _, r := range results {
				b = append(b, r.base[si][ni])
				mo = append(mo, r.mod[si][ni])
			}
			rep.Rows = append(rep.Rows, Row{Metric: name, Day: day, Count: name != "net worth top/median",
				Base: stat(b), Mod: stat(mo)})
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(warned.String()), "\n") {
		if line != "" {
			rep.ScriptFailures = append(rep.ScriptFailures, strings.TrimPrefix(line, "immortal-barons: "))
		}
	}
	rep.Elapsed = time.Since(began)
	return rep, nil
}

func stat(v []float64) Stat {
	s := Stat{Min: v[0], Max: v[0]}
	sum := 0.0
	for _, x := range v {
		sum += x
		s.Min, s.Max = min(s.Min, x), max(s.Max, x)
	}
	s.Mean = sum / float64(len(v))
	return s
}

// Format is a figure at the width a table column holds: whole numbers in full
// up to 99,999 and with a k, M or G suffix above, ratios to two places.
func Format(v float64, count bool) string {
	if !count {
		return fmt.Sprintf("%.2f", v)
	}
	a := v
	if a < 0 {
		a = -a
	}
	switch {
	case a >= 1e9:
		return fmt.Sprintf("%.2fG", v/1e9)
	case a >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	case a >= 1e5:
		return fmt.Sprintf("%.1fk", v/1e3)
	case v == float64(int64(v)):
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.1f", v)
}

// Text writes the report as a table that fits 80 columns.
func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Mod %s %s (%s) against the base game: %d seeds, %d AI realms, %d days.\n",
		r.ModName, r.ModVersion, r.ModHash, r.Seeds, r.Realms, r.Days)
	if len(r.Options) > 0 {
		b.WriteString("Options as played:\n")
		for _, o := range r.Options {
			b.WriteString("  " + o + "\n")
		}
	}
	if len(r.Changes) == 0 {
		b.WriteString("The mod changes nothing from the base game, so every difference is zero.\n")
	}
	b.WriteString("\nMeans across seeds, base game against mod:\n")
	fmt.Fprintf(&b, "%-22s %4s %9s %9s %9s %8s\n", "metric", "day", "base", "mod", "delta", "delta %")
	last := ""
	for _, row := range r.Rows {
		name := row.Metric
		if name == last {
			name = ""
		}
		last = row.Metric
		pct := "-"
		if p, ok := row.Percent(); ok {
			pct = fmt.Sprintf("%+.1f%%", p)
		}
		fmt.Fprintf(&b, "%-22s %4d %9s %9s %9s %8s\n", name, row.Day,
			Format(row.Base.Mean, row.Count), Format(row.Mod.Mean, row.Count),
			signed(row.Delta(), row.Count), pct)
	}
	fmt.Fprintf(&b, "\nSpread across seeds on day %d (min..max):\n", r.SampleDay[len(r.SampleDay)-1])
	fmt.Fprintf(&b, "%-22s %24s %24s\n", "metric", "base", "mod")
	for _, row := range r.Rows {
		if row.Day != r.SampleDay[len(r.SampleDay)-1] {
			continue
		}
		fmt.Fprintf(&b, "%-22s %24s %24s\n", row.Metric, span(row.Base, row.Count), span(row.Mod, row.Count))
	}
	if len(r.ScriptFailures) > 0 {
		b.WriteString("\nMod script failures (the game fell back where one failed):\n")
		for _, f := range r.ScriptFailures {
			b.WriteString("  " + f + "\n")
		}
	}
	if len(r.OutOfScope) > 0 {
		b.WriteString("\nNot simulated: this mod sets inter-BBS kinds (ip.*, sabre.*), which need more than\none board. The figures above leave them out:\n")
		for _, c := range r.OutOfScope {
			b.WriteString("  " + c + "\n")
		}
	}
	b.WriteString("\nThe AI barons play by their own code and do not adapt to a mod. This is a first\ncheck, not a replacement for play-testing.\n")
	fmt.Fprintf(&b, "(%.1f s)\n", r.Elapsed.Seconds())
	return b.String()
}

func signed(v float64, count bool) string {
	s := Format(v, count)
	if v > 0 {
		return "+" + s
	}
	return s
}

func span(s Stat, count bool) string {
	return Format(s.Min, count) + " .. " + Format(s.Max, count)
}

// CSV writes the report one row per metric and day, with the full figures.
func (r *Report) CSV() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# mod,%s,%s,%s\n# seeds,%d\n# realms,%d\n# days,%d\n", csvField(r.ModName), csvField(r.ModVersion), r.ModHash, r.Seeds, r.Realms, r.Days)
	for _, o := range r.Options {
		b.WriteString("# option," + csvField(o) + "\n")
	}
	b.WriteString("metric,day,base_mean,base_min,base_max,mod_mean,mod_min,mod_max,delta,delta_pct\n")
	for _, row := range r.Rows {
		pct := ""
		if p, ok := row.Percent(); ok {
			pct = fmt.Sprintf("%.4f", p)
		}
		fmt.Fprintf(&b, "%s,%d,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%s\n", row.Metric, row.Day,
			row.Base.Mean, row.Base.Min, row.Base.Max, row.Mod.Mean, row.Mod.Min, row.Mod.Max, row.Delta(), pct)
	}
	return b.String()
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
