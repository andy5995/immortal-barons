package game

import (
	"fmt"
	"strings"
	"testing"
)

func TestSeedPiratesRandomStrengthNoLadder(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if len(w.Pirates) != len(PirateFactions) {
		t.Fatalf("want %d factions, got %d", len(PirateFactions), len(w.Pirates))
	}
	ascending := true
	for i, p := range w.Pirates {
		if p.Forces < pirateForcesMin || p.Forces >= pirateForcesMax {
			t.Errorf("%s forces %d out of seed range", p.Name, p.Forces)
		}
		if i > 0 && p.Forces < w.Pirates[i-1].Forces {
			ascending = false
		}
	}
	if ascending {
		t.Error("seeded forces are strictly ascending — that's the old index ladder, not random")
	}
}

func TestPirateRaidTakesUnitsAndGrantsLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers, v.Jets, v.Tanks = 1000, 200, 100
	p := &w.Pirates[0]
	landBefore := p.Land

	w.pirateRaidVictim(p, v)

	if v.Troopers != 1000-1000*PirateRaidUnitPct/100 {
		t.Errorf("victim troopers not reduced: %d", v.Troopers)
	}
	if p.LootTanks != 100*PirateRaidUnitPct/100 {
		t.Errorf("faction should hold looted tanks, got %d", p.LootTanks)
	}
	if p.Land <= landBefore {
		t.Errorf("faction should be granted regions by the game, land %d -> %d", landBefore, p.Land)
	}
	if len(v.PirateRaids) == 0 {
		t.Error("human victim should get a raid notice")
	}
}

func TestPirateRaidOnAIRecordsNoNotice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	ai := w.Empires[0] // seeded AI empire (Owner == "")
	ai.Tanks = 100
	p := &w.Pirates[0]

	w.pirateRaidVictim(p, ai)

	if ai.Tanks != 100-100*PirateRaidUnitPct/100 {
		t.Errorf("AI victim should still lose units, got %d tanks", ai.Tanks)
	}
	if len(ai.PirateRaids) != 0 {
		t.Errorf("AI victim should not accumulate raid notices, got %d", len(ai.PirateRaids))
	}
}

func TestRaidFactionWinReclaimsPortion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	p := &w.Pirates[0]
	p.Forces = 100
	p.LootTanks = 400
	p.Land = 400

	before := a.Tanks
	w.RaidFaction(a, 0, 1_000_000, 0, 0)

	if p.LootTanks != 400-400*PirateReclaimPct/100 {
		t.Errorf("faction loot should drop by reclaim pct, got %d", p.LootTanks)
	}
	if a.Tanks != before+400*PirateReclaimPct/100 {
		t.Errorf("attacker should recover reclaimed tanks, got %d", a.Tanks)
	}
	if p.Land != 400-400*PirateReclaimPct/100 {
		t.Errorf("faction land should drop by reclaim pct, got %d", p.Land)
	}
}

// A win against a faction that holds no land captures no regions, so the caller
// shows no picker (#21 — BRE: raiding a landless band yields gold/military only).
func TestRaidFactionLandlessCapturesNoRegions(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
	p.Forces = 100
	p.Land = 0 // landless band
	p.Gold = 50_000

	beforeLand := a.Land
	_, captured := w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if captured != 0 {
		t.Errorf("landless faction should capture no regions, got %d", captured)
	}
	if a.Land != beforeLand {
		t.Errorf("attacker land must be unchanged, got %d want %d", a.Land, beforeLand)
	}
}

func TestPiratesSkipProtectedEmpires(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	safe := w.AddHuman("safe", "Safe")
	safe.Protection = 100
	safe.Troopers = 1_000_000
	exposed := w.AddHuman("exp", "Exposed")
	exposed.Protection = 0
	exposed.Troopers = 1_000_000

	for i := 0; i < 200; i++ {
		w.maybePirateRaid(safe)
		w.maybePirateRaid(exposed)
	}
	if len(safe.PirateRaids) != 0 {
		t.Errorf("empire under protection was raided %d times, want 0", len(safe.PirateRaids))
	}
	if len(exposed.PirateRaids) == 0 {
		t.Error("unprotected empire should have been raided at least once")
	}
}

// The per-turn raid roll fires ~PirateRaidChancePerTurn% of turns (BRE ~1-in-5),
// far from the old near-certain daily sweep. Over many turns an unprotected
// empire is raided sometimes but not almost every turn.
func TestPirateRaidPerTurnFrequency(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("e", "Empire")
	e.Protection = 0
	e.Troopers = 1_000_000

	const turns = 1000
	raidTurns := 0
	for i := 0; i < turns; i++ {
		before := len(e.PirateRaids)
		w.maybePirateRaid(e)
		if len(e.PirateRaids) > before {
			raidTurns++
		}
	}
	// Expect ~20% of turns to raid; allow a wide band for RNG (well under the
	// old daily model's near-100%, and clearly non-zero).
	if raidTurns < turns*10/100 || raidTurns > turns*32/100 {
		t.Errorf("raided on %d/%d turns; want roughly %d%%", raidTurns, turns, PirateRaidChancePerTurn)
	}
}

func TestRaidFactionScoreIsSmall(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	p := &w.Pirates[0]
	p.Forces = 100

	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if want := 100 / PirateScoreDivisor; a.Score != want {
		t.Errorf("pirate-win Score = %d, want %d (scaled by faction strength, not army size)", a.Score, want)
	}

	// A loss shaves the same small amount, and never below zero.
	b := w.AddHuman("you", "Yours")
	b.Troopers = 10
	b.Score = 1000
	q := &w.Pirates[1]
	q.Forces = 1000
	w.RaidFaction(b, 1, 10, 0, 0)
	if want := 1000 - 1000/PirateScoreDivisor; b.Score != want {
		t.Errorf("pirate-loss Score = %d, want %d", b.Score, want)
	}
}

func TestRaidFactionDrainsOverSeveralHits(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
	p.Forces = 100
	p.LootTroopers = 1000

	// One hit must not clear it; many hits should drain most of it.
	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if p.LootTroopers == 0 {
		t.Error("a single hit should not fully drain a faction")
	}
	for i := 0; i < 12; i++ {
		p.Forces = 100 // keep it beatable each hit
		w.RaidFaction(a, 0, 1_000_000, 0, 0)
	}
	if p.LootTroopers > 200 {
		t.Errorf("after many hits most loot should be reclaimed, %d left", p.LootTroopers)
	}
}

func TestEnsurePiratesSeedsOldSave(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Pirates = nil // simulate a save predating the pirate model
	w.EnsurePirates()
	if len(w.Pirates) != len(PirateFactions) {
		t.Errorf("EnsurePirates should seed %d factions, got %d", len(PirateFactions), len(w.Pirates))
	}
}

func TestPirateRaidStealsAllButBombersAndCarriers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers, v.Jets, v.Turrets, v.Tanks, v.Agents, v.Gold = 1000, 1000, 1000, 1000, 1000, 1000
	v.Bombers, v.Carriers = 500, 500
	p := &w.Pirates[0]

	w.pirateRaidVictim(p, v)

	if v.Bombers != 500 || v.Carriers != 500 {
		t.Errorf("pirates must not take bombers/carriers: bombers=%d carriers=%d", v.Bombers, v.Carriers)
	}
	stolen := map[string]int64{
		"troopers": int64(v.Troopers), "jets": int64(v.Jets), "turrets": int64(v.Turrets),
		"tanks": int64(v.Tanks), "agents": int64(v.Agents), "gold": v.Gold,
	}
	for name, got := range stolen {
		if got != 950 { // 1000 - 5%
			t.Errorf("%s: expected 950 after a 5%% raid, got %d", name, got)
		}
	}
	if p.LootAgents != 50 || p.Gold != 50 || p.LootTurrets != 50 {
		t.Errorf("faction should hold looted agents/gold/turrets, got A=%d G=%d U=%d",
			p.LootAgents, p.Gold, p.LootTurrets)
	}
}

func TestPirateTakeCappedAtMax(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers = 100_000_000 // 5% would be 5,000,000 — far over the take cap
	p := &w.Pirates[0]
	before := v.Troopers

	w.pirateRaidVictim(p, v)

	if before-v.Troopers != PirateRaidMaxTake {
		t.Errorf("a single raid should take at most %d, took %d", PirateRaidMaxTake, before-v.Troopers)
	}
}

func TestPirateHoldingClampsToCap(t *testing.T) {
	if got := capAdd(PirateCapTanks-10, 1000, PirateCapTanks); got != PirateCapTanks {
		t.Errorf("holdings should clamp to cap %d, got %d", PirateCapTanks, got)
	}
}

// The raid result follows BRE's captured shape: a headline, a "You took" tally
// with capitalised units and "and" before the last, and short lines that never
// need wrapping. Wording is verbatim from BRE.OVR.
func TestRaidReportMatchesBREsShape(t *testing.T) {
	got := raidWin(fmt.Sprintf(raidWinLines[0], "Dunkleoids"), raidLoot(4999, 6, 104, 62, 116, 19, 4), 0, 0, 0)
	want := "Your efforts against Dunkleoids have brought you success!\n" +
		"You took 4999 Gold, 6 Regions, 104 Troopers, 62 Jets, 116 Turrets, 19 Tanks, and 4 Agents."
	if got != want {
		t.Errorf("raid win report:\n got %q\nwant %q", got, want)
	}
	// Width is not asserted here. BRE's line fits because it abbreviates gold
	// ("78k"); IB prints the figure in full, so a full tally can pass 80 columns
	// and is wrapped where it is displayed — see TestReportsWrapAtWordBoundaries.
}

// Regions drop out when the faction holds no land — recorded from BRE's own
// screen — while the unit fields stay put so the tally reads the same every time.
func TestRaidLootOmitsRegionsWhenTheFactionHoldsNoLand(t *testing.T) {
	with := raidLoot(500, 8, 1, 2, 3, 4, 0)
	without := raidLoot(500, 0, 1, 2, 3, 4, 0)
	if !strings.Contains(with, "8 Regions") {
		t.Errorf("regions missing when the faction holds land: %q", with)
	}
	if strings.Contains(without, "Regions") {
		t.Errorf("regions named when the faction holds none: %q", without)
	}
	if !strings.Contains(without, "and 4 Tanks") {
		t.Errorf("tally should still end with \"and\": %q", without)
	}
}

// Every headline names the faction and leaves room for it inside 80 columns —
// a line that only fits the short faction names would wrap for the long ones.
func TestRaidHeadlinesNameTheFactionAndFit(t *testing.T) {
	longest := ""
	for _, n := range PirateFactions {
		if len(n) > len(longest) {
			longest = n
		}
	}
	for _, pool := range [][]string{raidWinLines, raidFailLines} {
		for _, f := range pool {
			if !strings.Contains(f, "%s") {
				t.Errorf("headline %q never names the faction", f)
				continue
			}
			if n := len(fmt.Sprintf(f, longest)); n >= 80 {
				t.Errorf("headline %q is %d columns with the longest faction name", f, n)
			}
		}
	}
}

// A losing raid uses one of the failure headlines and reports what it cost.
func TestRaidLossReport(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("owner", "Raider")
	a.Troopers, a.Jets, a.Tanks = 10, 10, 10
	w.Pirates[0].Forces = 1 << 30 // unbeatable, so the loss branch is certain

	report, land := w.RaidFaction(a, 0, 10, 10, 10)
	if land != 0 {
		t.Errorf("a lost raid captured %d regions", land)
	}
	headline := strings.SplitN(report, "\n", 2)[0]
	if !strings.Contains(headline, w.Pirates[0].Name) {
		t.Errorf("headline %q should name the faction", headline)
	}
	found := false
	for _, f := range raidFailLines {
		if headline == fmt.Sprintf(f, w.Pirates[0].Name) {
			found = true
		}
	}
	if !found {
		t.Errorf("headline %q is not one of the failure lines", headline)
	}
	if !strings.Contains(report, "\nYou lost ") {
		t.Errorf("a lost raid should report its cost on its own line: %q", report)
	}
}
