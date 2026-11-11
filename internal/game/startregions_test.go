package game

import (
	"encoding/json"
	"strings"
	"testing"
)

// The default reproduces BRE's start; any other size keeps its proportions and
// totals exactly, with rounding going to the largest fractions.
func TestStartMix(t *testing.T) {
	for _, c := range []struct {
		n    int
		want RegionMix
	}{
		{15, RegionMix{Agricultural: 2, Desert: 5, Mountain: 5, Coastal: 3}},
		{10000, RegionMix{Agricultural: 1334, Desert: 3333, Mountain: 3333, Coastal: 2000}},
		// 2.13 / 5.33 / 5.33 / 3.2: the one spare region goes to Desert, not Agricultural.
		{16, RegionMix{Agricultural: 2, Desert: 6, Mountain: 5, Coastal: 3}},
	} {
		got := startMix(c.n)
		if got != c.want {
			t.Errorf("startMix(%d) = %+v, want %+v", c.n, got, c.want)
		}
		if got.Total() != c.n {
			t.Errorf("startMix(%d) totals %d", c.n, got.Total())
		}
	}
}

func TestStartingRegionsScalesTheWholeStart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartingRegions = 10000
	e := newEmpire("Big", "big", cfg, 1)
	if e.Land != 10000 || e.Troopers != 66_666 || e.Food != 666_666 || e.Gold != 6_666_666 {
		t.Errorf("land %d troopers %d food %d gold %d", e.Land, e.Troopers, e.Food, e.Gold)
	}
	// People are not scaled with the land: they open at the default's share of
	// capacity (2000 of 2436), so the realm grows rather than shedding people.
	if e.People != 787_392 || e.People >= e.popCapacity() {
		t.Errorf("people %d, capacity %d", e.People, e.popCapacity())
	}
	if e := newEmpire("Std", "std", DefaultConfig(), 1); e.People != 2000 {
		t.Errorf("default start: people %d, want 2000", e.People)
	}
}

// A seeded test baron keeps its extra jets and turrets in proportion too.
func TestAIStartScales(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StartingRegions = 10000
	w := NewWorldSeed(cfg, 1)
	e := w.addAIEmpire("Bot")
	if e.Jets != 3333 || e.Turrets != 26_666 {
		t.Errorf("jets %d turrets %d, want 3333 and 26666", e.Jets, e.Turrets)
	}
}

func TestClampStartingRegions(t *testing.T) {
	for in, want := range map[int]int{0: 15, 5: 15, 15: 15, 10000: 10000, 50000: 20000} {
		if got := ClampStartingRegions(in); got != want {
			t.Errorf("ClampStartingRegions(%d) = %d, want %d", in, got, want)
		}
	}
}

// A league at the default sends the ruleset it sent before the setting, so its
// fingerprint is unchanged; a member that applies it gets the default back.
func TestStartingRegionsLeavesTheDefaultRulesetAlone(t *testing.T) {
	c := DefaultConfig()
	b, err := json.Marshal(c.leagueRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "StartingRegions") {
		t.Errorf("the default ruleset carries StartingRegions: %s", b)
	}
	var member Config
	member.applyLeagueRuleset(c.leagueRuleset())
	if member.StartingRegions != 15 {
		t.Errorf("member applied Starting Regions %d, want 15", member.StartingRegions)
	}
	before := c.RulesetFingerprint()
	c.StartingRegions = 10000
	if c.RulesetFingerprint() == before {
		t.Error("setting Starting Regions left the ruleset fingerprint unchanged")
	}
	member.applyLeagueRuleset(c.leagueRuleset())
	if member.StartingRegions != 10000 {
		t.Errorf("member applied Starting Regions %d, want 10000", member.StartingRegions)
	}
}
