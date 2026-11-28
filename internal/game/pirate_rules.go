package game

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PirateRules are the pirate figures a mod can change (#351): how often and how
// hard raids come, what a faction keeps and holds, what it starts a game with,
// and what a player's attack on it costs and wins. The base values are the
// constants of balance_pirates.go, binary-verified where that file says so, and
// those constants stay the fidelity contract: -print-base-mod shows them.
//
// Every figure is read where the raid or the attack is resolved, on the board
// that resolves it. None travels in a packet: a raid hits a realm on its own
// board under that board's rules, and a league plays one mod set.
type PirateRules struct {
	// A realm is raided when Random(RaidChanceOutOf) <= min(Land /
	// RaidRegionsPerStep + RaidChanceBase, RaidChanceCap).
	RaidChanceOutOf, RaidChanceBase, RaidChanceCap, RaidRegionsPerStep int64
	// A raid takes min(held / RaidTakeDivisor, RaidCapBase + Random(
	// RaidCapJitter)) of one holding, and the faction banks that times
	// RaidKeepMult. It is granted Random(RaidLandMax) regions.
	RaidTakeDivisor, RaidCapBase, RaidCapJitter, RaidKeepMult, RaidLandMax int64
	// The most a faction holds of each thing, clamped after every raid and
	// after a player beats it.
	CapGold, CapRegions, CapAgents, CapTroopers, CapJets, CapTurrets, CapTanks int64
	// A new game seeds each faction with Random(SeedX) of each thing; gold is
	// SeedGoldBase plus the product of two Random(SeedGoldRoll) draws.
	SeedTroopers, SeedTanks, SeedJets, SeedTurrets, SeedRegions int64
	SeedGoldBase, SeedGoldRoll                                  int64
	// When a player attacks a faction, each side loses LossMin + Random(
	// LossJitter) percent: the attacker of what it committed, the faction of
	// what it holds.
	AttackerLossMin, AttackerLossJitter, DefenderLossMin, DefenderLossJitter int64
	// A win hands back 1/ReclaimDivMain of the faction's gold, regions, agents
	// and troopers and 1/ReclaimDivArmour of its jets, turrets and tanks.
	ReclaimDivMain, ReclaimDivArmour int64
	// Names are the nine factions' names, in slot order. Display only: the
	// colors and every rule go by slot.
	Names []string
}

func basePirateRules() PirateRules {
	return PirateRules{
		RaidChanceOutOf: PirateRaidChanceOutOf, RaidChanceBase: PirateRaidChanceBase,
		RaidChanceCap: PirateRaidChanceCap, RaidRegionsPerStep: PirateRaidRegionsPerStep,
		RaidTakeDivisor: PirateRaidTakeDivisor, RaidCapBase: PirateRaidCapBase,
		RaidCapJitter: PirateRaidCapJitter, RaidKeepMult: PirateRaidKeepMult,
		RaidLandMax: PirateRaidLandMax,
		CapGold:     PirateCapGold, CapRegions: PirateCapRegions, CapAgents: PirateCapAgents,
		CapTroopers: PirateCapTroopers, CapJets: PirateCapJets, CapTurrets: PirateCapTurrets,
		CapTanks:     PirateCapTanks,
		SeedTroopers: PirateSeedTroopersRoll, SeedTanks: PirateSeedTanksRoll,
		SeedJets: PirateSeedJetsRoll, SeedTurrets: PirateSeedTurretsRoll,
		SeedRegions: PirateSeedRegionsRoll, SeedGoldBase: PirateSeedGoldBase,
		SeedGoldRoll:    PirateSeedGoldRoll,
		AttackerLossMin: PirateAttackerLossMin, AttackerLossJitter: PirateAttackerLossJitter,
		DefenderLossMin: PirateDefenderLossMin, DefenderLossJitter: PirateDefenderLossJitter,
		ReclaimDivMain: PirateReclaimDivMain, ReclaimDivArmour: PirateReclaimDivArmour,
		Names: slices.Clone(PirateFactions),
	}
}

// Limits for the pirate figures. Each is tighter than the type: the figures
// multiply or divide a count, and counts are int on the 32-bit door builds.
const (
	// A die of at most a thousand faces; a chance past the die is certain.
	ModPirateChanceMax = 1_000
	// Regions a realm needs for one more chance: a divisor, held at a million
	// regions, past which no realm reaches a second step.
	ModPirateRegionsPerStepMax = 1_000_000
	// Divisors of a holding or a loss (take, reclaim). 1 takes it all.
	ModPirateTakeDivisorMax = 1_000_000
	ModPirateReclaimDivMax  = 1_000
	// The take's cap and its jitter: a billion each, so their sum stays under
	// an int on a 32-bit build.
	ModPirateTakeCapMax = 1_000_000_000
	// What a faction banks is the take times this; the product is worked in
	// int64 and saturates, then is held to the faction's cap.
	ModPirateKeepMax = 1_000
	// Regions one raid grants: a raid order's "land" is held to the same.
	ModPirateRaidLandMax = 1_000
	// Faction caps: a billion units keeps tanks + turrets/2 + troopers/3, the
	// defense of a faction at every cap, inside an int on 32 bits (1.83e9).
	// Regions a million, as the combat floor; gold a trillion, well inside int64.
	ModPirateUnitCapMax   = 1_000_000_000
	ModPirateRegionCapMax = 1_000_000
	ModPirateGoldCapMax   = 1_000_000_000_000
	// Seed rolls are held to the unit cap; the gold roll's product of two
	// draws is at most 10^12, inside int64.
	ModPirateSeedRollMax = ModPirateUnitCapMax
	ModPirateSeedGoldMax = 1_000_000_000
	ModPirateGoldRollMax = 1_000_000
	// A percent of what a side holds. A loss's minimum plus its jitter less
	// one stays at 100: nobody loses more than all it has.
	ModPirateLossMax = 100
	// A faction name: shown beside a list number on an 80-column screen, and
	// long enough for the original's longest twice over.
	ModPirateNameMax = 20
	// A scripted raid's share of a normal raid's take, in percent.
	ModPirateShareMax = 100
)

// PirateField is one figure of PirateRules as a mod names it (see Field).
type PirateField = Field[PirateRules]

func pirateField(key string, min, max int64, wide bool, get func(p *PirateRules) *int64, desc string) PirateField {
	return PirateField{Key: key, Min: min, Max: max, Wide: wide,
		Get:  func(p PirateRules) int64 { return *get(&p) },
		Set:  func(p *PirateRules, v int64) { *get(p) = v },
		Desc: desc}
}

// PirateFields are the figures a mod can set under [pirates], in the order
// -print-base-mod writes them and validation reports them.
var PirateFields = []PirateField{
	pirateField("raid_chance_out_of", 1, ModPirateChanceMax, false, func(p *PirateRules) *int64 { return &p.RaidChanceOutOf },
		"Faces of the die rolled for each raid attempt. A raid lands when a roll from 0 up to one less than this is at most the chance below."),
	pirateField("raid_chance_base", 0, ModPirateChanceMax, false, func(p *PirateRules) *int64 { return &p.RaidChanceBase },
		"The chance, in that many faces, before a realm's size adds to it."),
	pirateField("raid_chance_cap", 0, ModPirateChanceMax, false, func(p *PirateRules) *int64 { return &p.RaidChanceCap },
		"The chance never rises past this, however large the realm."),
	pirateField("raid_regions_per_step", 1, ModPirateRegionsPerStepMax, false, func(p *PirateRules) *int64 { return &p.RaidRegionsPerStep },
		"Regions a realm needs for each extra face of chance."),
	pirateField("raid_take_divisor", 1, ModPirateTakeDivisorMax, false, func(p *PirateRules) *int64 { return &p.RaidTakeDivisor },
		"A raid takes this fraction of the holding it hits: 1 over this. 1 takes all of it."),
	pirateField("raid_take_cap", 0, ModPirateTakeCapMax, false, func(p *PirateRules) *int64 { return &p.RaidCapBase },
		"A raid takes at most this much, plus a random 0 up to one less than `raid_take_cap_jitter`."),
	pirateField("raid_take_cap_jitter", 1, ModPirateTakeCapMax, false, func(p *PirateRules) *int64 { return &p.RaidCapJitter },
		"The random part of that cap. 1 makes the cap exact."),
	pirateField("raid_keep_multiple", 0, ModPirateKeepMax, false, func(p *PirateRules) *int64 { return &p.RaidKeepMult },
		"The faction banks this many times what the realm loses. 1 banks exactly what was taken, 0 destroys it."),
	pirateField("raid_land_max", 0, ModPirateRaidLandMax, false, func(p *PirateRules) *int64 { return &p.RaidLandMax },
		"A raid grants its faction a random 0 up to one less than this many regions. 0 grants none."),
	pirateField("cap_gold", 0, ModPirateGoldCapMax, true, func(p *PirateRules) *int64 { return &p.CapGold },
		"The most gold a faction holds."),
	pirateField("cap_regions", 0, ModPirateRegionCapMax, false, func(p *PirateRules) *int64 { return &p.CapRegions },
		"The most regions a faction holds."),
	pirateField("cap_agents", 0, ModPirateUnitCapMax, false, func(p *PirateRules) *int64 { return &p.CapAgents },
		"The most agents a faction holds."),
	pirateField("cap_troopers", 0, ModPirateUnitCapMax, false, func(p *PirateRules) *int64 { return &p.CapTroopers },
		"The most troopers a faction holds."),
	pirateField("cap_jets", 0, ModPirateUnitCapMax, false, func(p *PirateRules) *int64 { return &p.CapJets },
		"The most jets a faction holds."),
	pirateField("cap_turrets", 0, ModPirateUnitCapMax, false, func(p *PirateRules) *int64 { return &p.CapTurrets },
		"The most turrets a faction holds."),
	pirateField("cap_tanks", 0, ModPirateUnitCapMax, false, func(p *PirateRules) *int64 { return &p.CapTanks },
		"The most tanks a faction holds."),
	pirateField("seed_troopers", 0, ModPirateSeedRollMax, false, func(p *PirateRules) *int64 { return &p.SeedTroopers },
		"At a reset each faction starts with a random 0 up to one less than this many troopers, held to its cap."),
	pirateField("seed_tanks", 0, ModPirateSeedRollMax, false, func(p *PirateRules) *int64 { return &p.SeedTanks },
		"The same for tanks."),
	pirateField("seed_jets", 0, ModPirateSeedRollMax, false, func(p *PirateRules) *int64 { return &p.SeedJets },
		"The same for jets."),
	pirateField("seed_turrets", 0, ModPirateSeedRollMax, false, func(p *PirateRules) *int64 { return &p.SeedTurrets },
		"The same for turrets."),
	pirateField("seed_regions", 0, ModPirateRegionCapMax, false, func(p *PirateRules) *int64 { return &p.SeedRegions },
		"The same for regions."),
	pirateField("seed_gold_base", 0, ModPirateSeedGoldMax, false, func(p *PirateRules) *int64 { return &p.SeedGoldBase },
		"Gold every faction starts with, before the random part."),
	pirateField("seed_gold_roll", 0, ModPirateGoldRollMax, false, func(p *PirateRules) *int64 { return &p.SeedGoldRoll },
		"Two random numbers, each from 0 up to one less than this, are multiplied and added to the starting gold. Most products are small, so a rich faction is rare."),
	pirateField("attacker_loss_min", 0, ModPirateLossMax, false, func(p *PirateRules) *int64 { return &p.AttackerLossMin },
		"When a player attacks a faction, the player loses this percent of what it committed, plus a random 0 up to one less than `attacker_loss_jitter`, win or lose."),
	pirateField("attacker_loss_jitter", 1, ModPirateLossMax, false, func(p *PirateRules) *int64 { return &p.AttackerLossJitter },
		"The random part of that loss. The minimum plus this, less one, stays at 100 or less."),
	pirateField("defender_loss_min", 0, ModPirateLossMax, false, func(p *PirateRules) *int64 { return &p.DefenderLossMin },
		"The faction loses this percent of its troopers, jets and tanks, plus a random 0 up to one less than `defender_loss_jitter`, win or lose."),
	pirateField("defender_loss_jitter", 1, ModPirateLossMax, false, func(p *PirateRules) *int64 { return &p.DefenderLossJitter },
		"The random part of that loss. The minimum plus this, less one, stays at 100 or less."),
	pirateField("reclaim_divisor_main", 1, ModPirateReclaimDivMax, false, func(p *PirateRules) *int64 { return &p.ReclaimDivMain },
		"A winning attack takes 1 over this of the faction's gold, regions, agents and troopers. 1 takes all of them."),
	pirateField("reclaim_divisor_armour", 1, ModPirateReclaimDivMax, false, func(p *PirateRules) *int64 { return &p.ReclaimDivArmour },
		"The same for its jets, turrets and tanks."),
}

// validatePirates reports each pirate figure outside its limits through bad.
func (r *Rules) validatePirates(bad func(format string, a ...any)) {
	p := r.PirateFigures
	validateFields(PirateFields, p, bad)
	for _, side := range []struct {
		name          string
		min, jitter   int64
		minK, jitterK string
	}{
		{"attacker", p.AttackerLossMin, p.AttackerLossJitter, "attacker_loss_min", "attacker_loss_jitter"},
		{"defender", p.DefenderLossMin, p.DefenderLossJitter, "defender_loss_min", "defender_loss_jitter"},
	} {
		if side.min+side.jitter-1 > ModPirateLossMax {
			bad("%s + %s may not exceed %d: a side cannot lose more than all it has", side.minK, side.jitterK, ModPirateLossMax+1)
		}
	}
	if len(p.Names) != len(PirateFactions) {
		bad("names: %d given, there are %d factions", len(p.Names), len(PirateFactions))
		return
	}
	for i, n := range p.Names {
		switch {
		case n == "" || strings.TrimSpace(n) != n:
			bad("names[%d]: must not be empty or start or end with a space", i)
		case utf8.RuneCountInString(n) > ModPirateNameMax:
			bad("names[%d]: longer than %d characters", i, ModPirateNameMax)
		case strings.ContainsFunc(n, UnsafeText):
			bad("names[%d]: holds a control or format character", i)
		case slices.Index(p.Names, n) != i:
			bad("names[%d]: %q is used twice", i, n)
		}
	}
}

// UnsafeText is a character that mod text may not show: a control character, or
// a Unicode format character that reorders or hides text.
func UnsafeText(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }

// pirateFigures is the pirate rules w plays by.
func (w *World) pirateFigures() *PirateRules { return &w.rules().PirateFigures }

// mulSat is a x b for non-negative figures, held at the largest int64.
func mulSat(a, b int64) int64 {
	if a > 0 && b > math.MaxInt64/a {
		return math.MaxInt64
	}
	return a * b
}

// bank adds delta to cur and holds the result to cap, without overflowing for
// any delta: the faction's holding of the thing never passes its cap.
func bank(cur, delta, cap int64) int64 {
	return min(cur+min(delta, cap), cap)
}

// scaleShare is took x pct / 100 without overflowing.
func scaleShare(took int64, pct int) int64 {
	return took/100*int64(pct) + took%100*int64(pct)/100
}

// CheckPirateOrder reports why a scripted raid's size is refused, or nil. The
// game checks every order with it, and the script reader checks its own first.
func CheckPirateOrder(i int, o PirateOrder) error {
	if o.Share != nil && (*o.Share < 0 || *o.Share > ModPirateShareMax) {
		return fmt.Errorf("raid %d: share %d is outside 0 to %d", i, *o.Share, ModPirateShareMax)
	}
	if o.Land != nil && (*o.Land < 0 || *o.Land > ModPirateRaidLandMax) {
		return fmt.Errorf("raid %d: land %d is outside 0 to %d", i, *o.Land, ModPirateRaidLandMax)
	}
	return nil
}
