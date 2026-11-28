package game

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// The base figures are the original's, written out as literals so that a
// retune of a balance constant fails here and asks for evidence.
func TestBasePirateRulesAreTheOriginals(t *testing.T) {
	want := PirateRules{
		RaidChanceOutOf: 20, RaidChanceBase: 2, RaidChanceCap: 6, RaidRegionsPerStep: 1200,
		RaidTakeDivisor: 33, RaidCapBase: 24_000, RaidCapJitter: 1_000, RaidKeepMult: 3, RaidLandMax: 25,
		CapGold: 600_000_000, CapRegions: 300, CapAgents: 200_000, CapTroopers: 300_000,
		CapJets: 400_000, CapTurrets: 400_000, CapTanks: 200_000,
		SeedTroopers: 7_000, SeedTanks: 4_000, SeedJets: 7_000, SeedTurrets: 9_000, SeedRegions: 75,
		SeedGoldBase: 500_000, SeedGoldRoll: 300,
		AttackerLossMin: 2, AttackerLossJitter: 5, DefenderLossMin: 4, DefenderLossJitter: 10,
		ReclaimDivMain: 3, ReclaimDivArmour: 4,
		Names: []string{"Humans", "Barbarians", "Nightjackals", "Sharks", "Dunkleoids",
			"Trilobarians", "Raptorians", "Gorgonoids", "Ammonians"},
	}
	if got := BaseRules().PirateFigures; !reflect.DeepEqual(got, want) {
		t.Errorf("base pirate rules\n got %+v\nwant %+v", got, want)
	}
	if err := BaseRules().Validate(); err != nil {
		t.Errorf("the base rules fail their own limits: %v", err)
	}
}

// Cloning must not let one mod's names reach the base rules.
func TestCloneDoesNotShareNames(t *testing.T) {
	r := BaseRules()
	r.PirateFigures.Names[0] = "Changed"
	if BaseRules().PirateFigures.Names[0] != "Humans" {
		t.Error("a clone's names share the base rules' array")
	}
}

// Every key refuses a value one past each end of its limits, and the error
// names the key.
func TestPirateFieldsRefuseOutOfRangeValues(t *testing.T) {
	for _, f := range PirateFields {
		for _, v := range []int64{f.Min - 1, f.Max + 1} {
			r := BaseRules()
			f.Set(&r.PirateFigures, v)
			err := r.Validate()
			if err == nil || !strings.Contains(err.Error(), "pirates: "+f.Key+" "+fmt.Sprint(v)+" is outside") {
				t.Errorf("%s = %d: err %v, want it refused by key", f.Key, v, err)
			}
		}
		for _, v := range []int64{f.Min, f.Max} {
			r := BaseRules()
			f.Set(&r.PirateFigures, v)
			if err := r.Validate(); err != nil && strings.Contains(err.Error(), "pirates: "+f.Key+" "+fmt.Sprint(v)+" is outside") {
				t.Errorf("%s = %d is inside its limits but was refused: %v", f.Key, v, err)
			}
		}
	}
}

func TestPirateLossPairsAndNamesAreChecked(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(p *PirateRules)
		want   string
	}{
		"attacker pair": {func(p *PirateRules) { p.AttackerLossMin, p.AttackerLossJitter = 60, 50 }, "attacker_loss_min + attacker_loss_jitter"},
		"defender pair": {func(p *PirateRules) { p.DefenderLossMin, p.DefenderLossJitter = 100, 2 }, "defender_loss_min + defender_loss_jitter"},
		"eight names":   {func(p *PirateRules) { p.Names = p.Names[:8] }, "8 given, there are 9"},
		"empty name":    {func(p *PirateRules) { p.Names[2] = "" }, "names[2]: must not be empty"},
		"padded name":   {func(p *PirateRules) { p.Names[2] = " Pad" }, "names[2]: must not be empty"},
		"long name":     {func(p *PirateRules) { p.Names[3] = strings.Repeat("x", 21) }, "names[3]: longer than 20"},
		"control":       {func(p *PirateRules) { p.Names[4] = "a\x1b[2Jb" }, "names[4]: holds a control or format"},
		"bidi":          {func(p *PirateRules) { p.Names[4] = "a‮b" }, "names[4]: holds a control or format"},
		"duplicate":     {func(p *PirateRules) { p.Names[5] = p.Names[1] }, "names[5]"},
	} {
		r := BaseRules()
		tc.change(&r.PirateFigures)
		if err := r.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, want it to say %q", name, err, tc.want)
		}
	}
	// 100 percent in all is the edge that is allowed.
	r := BaseRules()
	r.PirateFigures.AttackerLossMin, r.PirateFigures.AttackerLossJitter = 100, 1
	if err := r.Validate(); err != nil {
		t.Errorf("a loss of exactly 100 percent was refused: %v", err)
	}
}

// pirateWorld is a world playing figures, with one exposed realm holding n of
// everything, and every faction empty so a raid's gain reads straight off.
func pirateWorld(figures func(p *PirateRules), n int) (*World, *Empire) {
	cfg := DefaultConfig()
	cfg.Pirates = true
	w := NewWorldSeed(cfg, 1)
	w.Rules = BaseRules()
	figures(&w.Rules.PirateFigures)
	e := w.AddHuman("e", "Empire")
	e.Protection = 0
	stockAll(w, e, n)
	for i := range w.Pirates {
		w.Pirates[i] = PirateFaction{Name: w.Pirates[i].Name}
	}
	return w, e
}

func TestPirateTakeAndKeepFollowTheRules(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {
		p.RaidTakeDivisor, p.RaidCapBase, p.RaidCapJitter, p.RaidKeepMult = 10, 1_000_000, 1, 5
	}, 100_000)
	w.pirateRaid(0, e, pirateDraw{Spoil: SpoilJets})
	if e.Jets != 90_000 || w.Pirates[0].LootJets != 50_000 {
		t.Errorf("victim jets %d, faction jets %d; want 90000 and 50000", e.Jets, w.Pirates[0].LootJets)
	}
	w.pirateRaid(0, e, pirateDraw{Spoil: SpoilGold})
	if e.Gold != 90_000 || w.Pirates[0].Gold != 50_000 {
		t.Errorf("victim gold %d, faction gold %d; want 90000 and 50000", e.Gold, w.Pirates[0].Gold)
	}
}

func TestPirateTakeIsCappedByTheRulesCap(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) { p.RaidCapBase, p.RaidCapJitter = 700, 1 }, 1<<20)
	w.pirateRaid(2, e, pirateDraw{Spoil: SpoilTanks})
	if got := 1<<20 - e.Tanks; got != 700 {
		t.Errorf("took %d tanks, want the cap of 700", got)
	}
}

func TestPirateCapsHoldWhatTheFactionBanks(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {
		p.RaidTakeDivisor, p.RaidKeepMult, p.CapJets, p.CapGold, p.CapRegions, p.RaidLandMax = 1, 3, 1_000, 2_500, 4, 1000
	}, 100_000)
	w.pirateRaid(0, e, pirateDraw{Spoil: SpoilJets})
	w.pirateRaid(0, e, pirateDraw{Spoil: SpoilGold})
	p := w.Pirates[0]
	if p.LootJets != 1_000 || p.Gold != 2_500 || p.Land != 4 {
		t.Errorf("jets %d, gold %d, land %d; want the caps 1000, 2500 and 4", p.LootJets, p.Gold, p.Land)
	}
}

// A take times a large multiple is worked in int64 and held to the cap, on a
// 32-bit build too (GOARCH=386 go test runs this).
func TestPirateKeepSaturatesInsteadOfWrapping(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {
		p.RaidTakeDivisor, p.RaidCapBase, p.RaidCapJitter, p.RaidKeepMult = 1, ModPirateTakeCapMax, 1, ModPirateKeepMax
		p.CapTroopers = ModPirateUnitCapMax
	}, 100)
	e.Troopers = 2_000_000_000
	w.pirateRaid(0, e, pirateDraw{Spoil: SpoilTroopers})
	if e.Troopers != 1_000_000_000 || w.Pirates[0].LootTroopers != ModPirateUnitCapMax {
		t.Errorf("victim troopers %d, faction %d; want 1e9 left and the cap %d", e.Troopers, w.Pirates[0].LootTroopers, ModPirateUnitCapMax)
	}
	if got := mulSat(math.MaxInt64/2, 3); got != math.MaxInt64 {
		t.Errorf("mulSat overflowed to %d", got)
	}
	if got := bank(5, math.MaxInt64, 100); got != 100 {
		t.Errorf("bank(5, MaxInt64, 100) = %d", got)
	}
}

func TestPirateLandGrantFollowsTheRules(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) { p.RaidLandMax = 1 }, 1000)
	for range 20 {
		w.pirateRaid(3, e, pirateDraw{Spoil: SpoilAgents})
	}
	if w.Pirates[3].Land != 0 {
		t.Errorf("land %d, want none from raid_land_max 1", w.Pirates[3].Land)
	}
	w.Rules.PirateFigures.RaidLandMax = 0
	w.pirateRaid(3, e, pirateDraw{Spoil: SpoilAgents})
	if w.Pirates[3].Land != 0 {
		t.Errorf("land %d with raid_land_max 0", w.Pirates[3].Land)
	}
}

func TestPirateRaidChanceFollowsTheRules(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {
		p.RaidChanceBase, p.RaidChanceCap, p.RaidRegionsPerStep = 1, 9, 100
	}, 100_000)
	for land, want := range map[int]int{0: 1, 99: 1, 100: 2, 450: 5, 5_000: 9} {
		e.Land = land
		if got := w.pirateRaidChance(e); got != want {
			t.Errorf("land %d: chance %d, want %d", land, got, want)
		}
	}
	// A die of one face always lands.
	w.Rules.PirateFigures.RaidChanceOutOf = 1
	e.Land = 0
	for range 5 {
		w.maybePirateRaid(e)
	}
	if len(e.PirateHits) == 0 {
		t.Error("no raid landed on a die of one face")
	}
}

func TestPirateAttackLossesAndReclaimFollowTheRules(t *testing.T) {
	w, a := pirateWorld(func(p *PirateRules) {
		p.AttackerLossMin, p.AttackerLossJitter = 10, 1
		p.DefenderLossMin, p.DefenderLossJitter = 20, 1
		p.ReclaimDivMain, p.ReclaimDivArmour = 1, 2
	}, 10_000)
	f := &w.Pirates[0]
	f.LootTroopers, f.LootJets, f.LootTanks, f.LootTurrets, f.LootAgents, f.Gold, f.Land = 1000, 1000, 1000, 1000, 1000, 1000, 80
	// The faction's defense is 800 tanks + 500 turrets + 266 troopers after its
	// 20 percent loss, far below the attacker's offense, so the attack wins.
	// The attacker loses 10 percent of each force it committed.
	_, land := w.RaidFaction(a, 0, 10_000, 10_000, 10_000)
	if a.Jets != 9_000+400 { // 10% lost, then half of the faction's 800
		t.Errorf("jets %d, want 9400", a.Jets)
	}
	if a.Tanks != 9_000+400 {
		t.Errorf("tanks %d, want 9400", a.Tanks)
	}
	if a.Troopers != 9_000+800 { // all of the faction's remaining 800
		t.Errorf("troopers %d, want 9800", a.Troopers)
	}
	if a.Turrets != 10_000+500 || a.Agents != 10_000+1000 {
		t.Errorf("turrets %d agents %d, want 10500 and 11000", a.Turrets, a.Agents)
	}
	if land != 80 || f.Land != 0 || f.Gold != 0 {
		t.Errorf("won %d land; faction land %d gold %d; want 80, 0, 0", land, f.Land, f.Gold)
	}
}

func TestPirateSeedFollowsTheRules(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 3)
	w.Rules = BaseRules()
	p := &w.Rules.PirateFigures
	p.SeedTroopers, p.SeedTanks, p.SeedJets, p.SeedTurrets, p.SeedRegions = 1, 0, 1, 1, 1
	p.SeedGoldBase, p.SeedGoldRoll = 123_456, 1
	p.Names = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	w.seedPirates()
	for i, f := range w.Pirates {
		if f.Name != p.Names[i] || f.Gold != 123_456 || f.LootTroopers != 0 || f.LootTanks != 0 || f.Land != 0 {
			t.Errorf("faction %d seeded as %+v", i, f)
		}
	}
	p.SeedTroopers, p.CapTroopers = 1_000_000, 10
	p.SeedGoldBase, p.CapGold = 1_000_000, 777
	w.seedPirates()
	for _, f := range w.Pirates {
		if f.LootTroopers > 10 || f.Gold != 777 {
			t.Errorf("a seed went past its cap: %+v", f)
		}
	}
	w.Pirates = nil
	w.Config.Pirates = true
	w.EnsurePirates()
	if got := w.Pirates[4].Name; got != "E" {
		t.Errorf("EnsurePirates named slot 4 %q, want E", got)
	}
}

// A scripted raid's share and land are honoured, and refused out of range.
func TestScriptedRaidShareAndLand(t *testing.T) {
	ptr := func(n int) *int { return &n }
	b := &fakeBrain{orders: []PirateOrder{{Slot: 1, Spoil: SpoilJets, Share: ptr(50), Land: ptr(7)}}}
	w, e := brainWorld(b)
	e.Jets = 33_000
	land := w.Pirates[1].Land
	w.maybePirateRaid(e)
	if e.Jets != 33_000-500 { // 1000 is a normal take, half of it
		t.Errorf("jets %d, want 32500", e.Jets)
	}
	if got := w.Pirates[1].Land - land; got != 7 {
		t.Errorf("land granted %d, want 7", got)
	}

	// Share 0 takes nothing and still grants the land ordered.
	b.orders = []PirateOrder{{Slot: 1, Spoil: SpoilJets, Share: ptr(0), Land: ptr(3)}}
	jets, land := e.Jets, w.Pirates[1].Land
	w.maybePirateRaid(e)
	if e.Jets != jets || w.Pirates[1].Land-land != 3 {
		t.Errorf("share 0: jets %d -> %d, land +%d", jets, e.Jets, w.Pirates[1].Land-land)
	}

	// Left out: today's roll, up to raid_land_max, with the full take.
	b.orders = []PirateOrder{{Slot: 1, Spoil: SpoilJets}}
	jets = e.Jets
	w.maybePirateRaid(e)
	if jets-e.Jets != 984 { // 32,500 / 33
		t.Errorf("absent share took %d, want 984", jets-e.Jets)
	}

	for _, o := range []PirateOrder{
		{Slot: 1, Spoil: SpoilJets, Share: ptr(101)},
		{Slot: 1, Spoil: SpoilJets, Share: ptr(-1)},
		{Slot: 1, Spoil: SpoilJets, Land: ptr(ModPirateRaidLandMax + 1)},
		{Slot: 1, Spoil: SpoilJets, Land: ptr(-1)},
	} {
		b.orders = []PirateOrder{o}
		failed := len(b.failed)
		jets = e.Jets
		w.maybePirateRaid(e)
		if len(b.failed) != failed+1 {
			t.Errorf("order %+v was not refused", o)
		}
		_ = jets
	}
}

// scriptedSource is a rand.Source that hands out the Int31 values it was given
// and fails the test on any draw past them: Int31 is Int63()>>32.
type scriptedSource struct {
	t    *testing.T
	vals []int64
}

func (s *scriptedSource) Int63() int64 {
	if len(s.vals) == 0 {
		s.t.Errorf("a draw was made past the ones the routine may make")
		return 0
	}
	v := s.vals[0]
	s.vals = s.vals[1:]
	return v << 32
}
func (s *scriptedSource) Seed(int64) {}

// A raid that takes nothing ends the routine: no land, no notice and no retry
// (BRE.OVR 0x35f9b). Only the faction, chance, spoil and cap-jitter draws are
// allowed; the scripted source fails on any other.
func TestAZeroTakeRaidGrantsNoLandAndRollsNoRetry(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {}, 10)
	e.Jets = 10 // under 33: a take of nothing
	src := &scriptedSource{t: t, vals: []int64{1, 0, 2, 0}}
	w.rng = rand.New(src)
	w.maybePirateRaid(e)
	if len(src.vals) != 0 {
		t.Errorf("%d scripted draws unused", len(src.vals))
	}
	if w.Pirates[1].Land != 0 || len(e.PirateHits) != 0 || e.Jets != 10 {
		t.Errorf("land %d, hits %v, jets %d", w.Pirates[1].Land, e.PirateHits, e.Jets)
	}
}

// A missed chance roll still reaches the retry roll.
func TestAMissedChanceRollStillRollsTheRetry(t *testing.T) {
	w, e := pirateWorld(func(p *PirateRules) {}, 10)
	// faction 0; chance roll 19 misses (chance is 2); retry roll 5 is not 0.
	src := &scriptedSource{t: t, vals: []int64{0, 19, 5}}
	w.rng = rand.New(src)
	w.maybePirateRaid(e)
	if len(src.vals) != 0 {
		t.Errorf("%d scripted draws unused", len(src.vals))
	}
}

// A scripted order with no land of its own follows the same rule, and one that
// names land gets it even when it took nothing.
func TestScriptedZeroTakeRaidLand(t *testing.T) {
	ptr := func(n int) *int { return &n }
	b := &fakeBrain{orders: []PirateOrder{{Slot: 1, Spoil: SpoilJets}}}
	w, e := brainWorld(b)
	e.Jets = 10
	src := &scriptedSource{t: t, vals: []int64{0}} // the cap jitter only
	w.rng = rand.New(src)
	land := w.Pirates[1].Land
	w.maybePirateRaid(e)
	if w.Pirates[1].Land != land || len(e.PirateHits) != 0 {
		t.Errorf("no explicit land: land %d -> %d, hits %v", land, w.Pirates[1].Land, e.PirateHits)
	}

	b.orders = []PirateOrder{{Slot: 1, Spoil: SpoilJets, Land: ptr(5)}}
	src = &scriptedSource{t: t, vals: []int64{0}}
	w.rng = rand.New(src)
	w.maybePirateRaid(e)
	if w.Pirates[1].Land != land+5 || len(e.PirateHits) != 0 {
		t.Errorf("explicit land: land %d -> %d, hits %v", land, w.Pirates[1].Land, e.PirateHits)
	}
}

// Winning loot back saturates at MaxUnits instead of wrapping on a 32-bit int.
func TestRaidFactionWinSaturatesTheAttacker(t *testing.T) {
	w, a := pirateWorld(func(p *PirateRules) {
		p.ReclaimDivMain, p.ReclaimDivArmour = 1, 1
		p.AttackerLossMin, p.AttackerLossJitter, p.DefenderLossMin, p.DefenderLossJitter = 0, 1, 0, 1
	}, 10)
	f := &w.Pirates[0]
	// Its defense (tanks + turrets/2 + troopers/3) stays small so the attack wins.
	f.LootTroopers, f.LootJets, f.LootTurrets, f.LootTanks, f.LootAgents, f.Land = 100, 1_000_000_000, 100, 100, 1_000_000_000, 1_000_000
	a.Troopers, a.Jets, a.Turrets, a.Tanks, a.Agents = MaxUnits-5, MaxUnits-5, MaxUnits-5, MaxUnits-5, MaxUnits-5
	a.PendingRegions = MaxUnits - 5
	w.RaidFaction(a, 0, 1000, 1000, 1000)
	for name, got := range map[string]int{"troopers": a.Troopers, "jets": a.Jets, "turrets": a.Turrets,
		"tanks": a.Tanks, "agents": a.Agents, "pending regions": a.PendingRegions} {
		if got != MaxUnits {
			t.Errorf("%s = %d, want %d", name, got, MaxUnits)
		}
	}
}

// The rules are the one source of the displayed name: factions that exist
// before a mod is applied take its names when it is.
func TestSyncPirateNamesFollowsTheRules(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.Pirates = true
	w.Pirates = nil
	w.EnsurePirates() // a repair before any mod: the base names
	if w.Pirates[0].Name != "Humans" {
		t.Fatalf("repaired as %q", w.Pirates[0].Name)
	}
	w.Rules = BaseRules()
	w.Rules.PirateFigures.Names = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	w.SyncPirateNames()
	if w.Pirates[0].Name != "A" || w.Pirates[8].Name != "I" {
		t.Errorf("names after sync: %q ... %q", w.Pirates[0].Name, w.Pirates[8].Name)
	}
}
