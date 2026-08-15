package game

import (
	"fmt"
	"strings"
	"testing"
)

// A band starts with nothing. Its army is stolen goods, so a fresh game's
// factions cannot defend themselves until they have robbed somebody.
func TestSeedPiratesStartEmpty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if len(w.Pirates) != len(PirateFactions) {
		t.Fatalf("want %d factions, got %d", len(PirateFactions), len(w.Pirates))
	}
	for _, p := range w.Pirates {
		if p.Defense() != 0 || p.Land != 0 || p.Gold != 0 || p.LootAgents != 0 {
			t.Errorf("%s seeded with holdings: %+v", p.Name, p)
		}
	}
}

// Defense is the loot, weighted: tanks + turrets/2 + troopers/3.
func TestPirateDefenseIsItsLoot(t *testing.T) {
	p := PirateFaction{LootTanks: 100, LootTurrets: 50, LootTroopers: 90}
	if got, want := p.Defense(), 100+25+30; got != want {
		t.Errorf("Defense() = %d, want %d", got, want)
	}
}

// stockAll gives v the same amount in inventory AND on the market for every
// good a raid can reach, so each of the sixteen faces has somewhere to land.
func stockAll(w *World, v *Empire, n int) {
	v.Troopers, v.Jets, v.Turrets, v.Tanks, v.Agents = n, n, n, n, n
	v.Gold = int64(n)
	w.Market = nil
	for _, g := range []string{"Trooper", "Jet", "Turret", "Tank", "Agent"} {
		w.Market = append(w.Market, MarketListing{Realm: v.Name, Good: g, Qty: n, Price: 1})
	}
}

// heldTotal sums everything a raid could have taken, across both stores.
func heldTotal(w *World, v *Empire) int64 {
	t := int64(v.Troopers+v.Jets+v.Turrets+v.Tanks+v.Agents) + v.Gold
	for _, l := range w.Market {
		t += int64(l.Qty)
	}
	return t
}

// Escrowing military on the Trading Market does NOT hide it from pirates: five
// of the sixteen faces drain the listing instead of the inventory. Verified by
// driving BRE — 73 troopers listed showed up at record +0x211, the field the
// raid's bucket 11 reads.
func TestPirateRaidTakesFromTheMarketListing(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")

	hitListing := false
	for i := 0; i < 300; i++ {
		v.Troopers = 0 // everything is escrowed, so only a market face can take any
		v.Jets, v.Turrets, v.Tanks, v.Agents, v.Gold = 0, 0, 0, 0, 0
		w.Market = []MarketListing{{Realm: v.Name, Good: "Trooper", Qty: 33_000, Price: 1}}
		v.PirateHits = nil

		w.pirateRaidVictim(0, v)

		if len(v.PirateHits) == 0 {
			continue
		}
		hit := v.PirateHits[0]
		if hit.Spoil != SpoilTroopers {
			t.Fatalf("only the escrowed troopers could be taken, got %v", hit)
		}
		want := int64(33_000 / PirateRaidTakeDivisor)
		if hit.Amount != want {
			t.Fatalf("took %d from the listing, want %d", hit.Amount, want)
		}
		if w.Market[0].Qty != 33_000-int(want) {
			t.Fatalf("listing not drained: %d", w.Market[0].Qty)
		}
		hitListing = true
		break
	}
	if !hitListing {
		t.Error("no raid ever reached the market listing")
	}
}

// A raid takes ONE kind of thing, never a slice of everything: BRE draws a
// single category per raid (BRE.OVR 0x35e30), which is why its notices only
// ever name one. Run many raids so every category comes up, and check each
// leaves the other five untouched.
func TestPirateRaidTakesOneCategoryAndGrantsLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	p := &w.Pirates[0]

	seen := map[PirateSpoil]bool{}
	grantedLand := false
	const held = 1000
	for i := 0; i < 200; i++ {
		stockAll(w, v, held)
		v.PirateHits = nil
		p.Land = 0
		landBefore := p.Land

		w.pirateRaidVictim(0, v)

		if len(v.PirateHits) != 1 {
			t.Fatalf("human victim should get exactly one raid notice, got %d", len(v.PirateHits))
		}
		hit := v.PirateHits[0]
		seen[hit.Spoil] = true
		want := int64(held / PirateRaidTakeDivisor)
		if hit.Amount != want {
			t.Fatalf("%v: took %d, want %d", hit.Spoil, hit.Amount, want)
		}
		// Exactly one of the eleven sources moved, by exactly that much.
		if total, wantTotal := heldTotal(w, v), int64(11*held)-want; total != wantTotal {
			t.Fatalf("raid took %v; holdings total %d, want %d (only one source may fall)", hit.Spoil, total, wantTotal)
		}
		// Random(25), so a single raid may grant none.
		if p.Land < landBefore {
			t.Fatalf("faction land fell, %d -> %d", landBefore, p.Land)
		}
		if p.Land > landBefore {
			grantedLand = true
		}
	}
	if !grantedLand {
		t.Error("no raid ever granted the faction regions")
	}
	if len(seen) != 6 {
		t.Errorf("only %d of the 6 categories were ever raided: %v", len(seen), seen)
	}
}

// The draw is weighted as BRE's ladder is: 3/16 for each of the four unit
// types, 2/16 for gold and for agents.
func TestPirateSpoilWeights(t *testing.T) {
	want := map[PirateSpoil]int{
		SpoilTroopers: 3, SpoilJets: 3, SpoilTurrets: 3,
		SpoilTanks: 3, SpoilGold: 2, SpoilAgents: 2,
	}
	got := map[PirateSpoil]int{}
	market := 0
	for _, d := range pirateSpoilWeights {
		got[d.Spoil]++
		if d.FromMarket {
			market++
		}
	}
	// Faces 11-15 target the Trading Market listing (proven by driving BRE).
	if market != 5 {
		t.Errorf("%d of 16 faces take from the market listing, want 5", market)
	}
	for spoil, n := range want {
		if got[spoil] != n {
			t.Errorf("%v has %d/16 of the draw, want %d/16", spoil, got[spoil], n)
		}
	}
}

func TestPirateRaidOnAIRecordsNoNotice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	ai := w.Empires[0] // seeded AI empire (Owner == "")
	stockAll(w, ai, 100)

	w.pirateRaidVictim(0, ai)

	// One source is drained, whichever the draw picked.
	held := heldTotal(w, ai)
	if want := int64(11*100 - 100/PirateRaidTakeDivisor); held != want {
		t.Errorf("AI victim should still lose units, holdings total %d want %d", held, want)
	}
	if len(ai.PirateHits) != 0 {
		t.Errorf("AI victim should not accumulate raid notices, got %d", len(ai.PirateHits))
	}
}

func TestRaidFactionWinReclaimsPortion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	p := &w.Pirates[0]
	p.LootAgents = 400 // agents take no battle losses, so the maths is exact
	p.Land = 400

	before := a.Agents
	w.RaidFaction(a, 0, 1_000_000, 0, 0)

	// A third of agents and of land, per BRE.
	if want := 400 - 400/PirateReclaimDivMain; p.LootAgents != want {
		t.Errorf("faction agents = %d, want %d", p.LootAgents, want)
	}
	if want := before + 400/PirateReclaimDivMain; a.Agents != want {
		t.Errorf("attacker agents = %d, want %d", a.Agents, want)
	}
	if want := 400 - 400/PirateReclaimDivMain; p.Land != want {
		t.Errorf("faction land = %d, want %d", p.Land, want)
	}
}

// A winning raid still costs the attacker: 2-6% of what it committed (#119).
func TestRaidFactionWinStillCostsTheAttacker(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	sent := 1_000_000

	report, _ := w.RaidFaction(a, 0, sent, 0, 0)

	lost := 1_000_000 - a.Troopers
	lo := sent * PirateAttackerLossMin / 100
	hi := sent * (PirateAttackerLossMin + PirateAttackerLossJitter - 1) / 100
	if lost < lo || lost > hi {
		t.Errorf("winner lost %d troopers, want %d..%d", lost, lo, hi)
	}
	if !strings.Contains(report, "You lost") {
		t.Errorf("a winning raid should report its casualties, got:\n%s", report)
	}
}

// A win against a faction that holds no land captures no regions, so the caller
// shows no picker (#21 — BRE: raiding a landless band yields gold/military only).
func TestRaidFactionLandlessCapturesNoRegions(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
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
	if len(safe.PirateHits) != 0 {
		t.Errorf("empire under protection was raided %d times, want 0", len(safe.PirateHits))
	}
	if len(exposed.PirateHits) == 0 {
		t.Error("unprotected empire should have been raided at least once")
	}
}

// The per-turn roll scales with the realm: Random(20) <= min(6, regions/1200+2),
// then a 1-in-10 retry. A small realm is raided on about 1 turn in 6, a huge one
// on about 1 in 2.6 — so the test pins both ends of the band, not one figure.
func TestPirateRaidFrequencyScalesWithRealmSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	for _, c := range []struct{ land, wantK int }{
		{100, 2},    // floor
		{2_400, 4},  // two steps up
		{50_000, 6}, // capped
	} {
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("e", "Empire")
		e.Protection = 0
		e.Land = c.land
		stockAll(w, e, 1<<20)

		if got := pirateRaidChance(e); got != c.wantK {
			t.Errorf("land %d: chance %d/20, want %d/20", c.land, got, c.wantK)
		}
		const turns = 4000
		raids := 0
		for i := 0; i < turns; i++ {
			before := len(e.PirateHits)
			w.maybePirateRaid(e)
			raids += len(e.PirateHits) - before
		}
		// Raids per turn = P(pass) x the retry chain's 1/(1-1/10).
		want := float64(c.wantK+1) / PirateRaidChanceOutOf * PirateRaidRetryOutOf / (PirateRaidRetryOutOf - 1)
		got := float64(raids) / turns
		if got < want*0.85 || got > want*1.15 {
			t.Errorf("land %d: %.3f raids/turn, want ~%.3f", c.land, got, want)
		}
	}
}

func TestRaidFactionScoreIsSmall(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	w.Pirates[0].LootTanks = 100

	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if want := 100 / PirateScoreDivisor; a.Score != want {
		t.Errorf("pirate-win Score = %d, want %d (scaled by faction strength, not army size)", a.Score, want)
	}

	// A loss shaves the same small amount, and never below zero.
	b := w.AddHuman("you", "Yours")
	b.Troopers = 10
	b.Score = 1000
	w.Pirates[1].LootTanks = 1000
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

// Bombers and carriers are absent from BRE's name table, so no draw can reach
// them however many raids land.
func TestPirateRaidNeverTakesBombersOrCarriers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	for i := 0; i < 200; i++ {
		stockAll(w, v, 1000)
		v.Bombers, v.Carriers = 500, 500
		w.pirateRaidVictim(0, v)
		if v.Bombers != 500 || v.Carriers != 500 {
			t.Fatalf("pirates must not take bombers/carriers: bombers=%d carriers=%d", v.Bombers, v.Carriers)
		}
	}
}

func TestPirateTakeCappedAtMax(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	// Every category far over the take cap, so whichever the draw picks, 5% of
	// it would be 5,000,000.
	for i := 0; i < 50; i++ {
		stockAll(w, v, 100_000_000)
		v.PirateHits = nil

		w.pirateRaidVictim(0, v)

		// The cap is jittered: 24000 + Random(1000), so the take lands in the band.
		if len(v.PirateHits) != 1 {
			t.Fatalf("want one notice, got %v", v.PirateHits)
		}
		got := v.PirateHits[0].Amount
		if got < PirateRaidCapBase || got >= PirateRaidCapBase+PirateRaidCapJitter {
			t.Fatalf("take %d outside the cap band [%d,%d)", got, PirateRaidCapBase, PirateRaidCapBase+PirateRaidCapJitter)
		}
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
	w.Pirates[0].LootTanks = 1 << 20 // unbeatable, so the loss branch is certain

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

// With pirates off there are no bands at all: none are seeded on a fresh game,
// none are seeded on load, and nobody is raided.
func TestPiratesDisabledMeansNoBandsAndNoRaids(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Pirates = false
	w := NewWorldSeed(cfg, 1)
	if len(w.Pirates) != 0 {
		t.Fatalf("a fresh game seeded %d bands with pirates off", len(w.Pirates))
	}
	w.EnsurePirates()
	if len(w.Pirates) != 0 {
		t.Errorf("loading re-seeded %d bands with pirates off", len(w.Pirates))
	}

	// A realm the bands would certainly rob if they existed: no protection, and
	// enough troopers that every raid roll finds something to take.
	e := w.AddHuman("h", "Realm")
	e.Protection, e.Troopers, e.Land = 0, 1_000_000, 500
	e.syncLand()
	for i := 0; i < 500; i++ {
		w.maybePirateRaid(e)
	}
	if len(e.PirateHits) != 0 {
		t.Errorf("pirates raided %d times with the setting off", len(e.PirateHits))
	}

	// Turning them back on restores the bands rather than leaving the game
	// permanently pirate-free.
	w.Config.Pirates = true
	w.EnsurePirates()
	if len(w.Pirates) != len(PirateFactions) {
		t.Errorf("turning pirates back on seeded %d bands, want %d", len(w.Pirates), len(PirateFactions))
	}
}
