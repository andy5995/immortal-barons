package game

import (
	"errors"
	"fmt"
	"maps"
)

// Rules are the gameplay values one world plays by (#202). A world with no mods
// plays the base rules, which are built from the balance*.go constants: those
// files are the base mod, and the binary-verified values in them stay the
// fidelity contract. A mod (internal/mod) changes a copy of the base rules, and
// anything that differs from the base is by definition not the original game.
//
// Only the values listed here can be changed by a mod. The set starts with the
// per-unit figures of the six military units and grows one subject at a time;
// every other number is still a constant.
type Rules struct {
	Units map[*Good]UnitRules
	// Regions is what each region type yields and costs (region_rules.go).
	Regions map[*Region]RegionRules
	// Pirates, when set, decides pirate raids in place of the built-in roll.
	Pirates PirateBrain
	// RegionBrains change a region type's row for each realm, at most one per
	// type (region_brain.go).
	RegionBrains map[*Region]RegionBrain
	// HQ are the HeadQuarters figures.
	HQ HQRules
	// Combat is what the Attack Rewards and Attack Damage levels mean and what a
	// same-planet win pays (combat_rules.go).
	Combat CombatRules
	// Attacks is what each kind of attack does, to its target and back to its
	// attacker (attack_effects.go).
	Attacks map[EffectKind]AttackEffects
}

// HQRules are the HeadQuarters figures. The strength it adds to each unit is a
// unit's figure (UnitRules.HQBonus). What only interplanetary play does to
// a HeadQuarters is not here — the resolver's own tank curve
// (RemoteTankStrength*), the fifth a winning invasion takes
// (InvasionHQLossDivisor) and a landed Sabotage HQ agent's points
// (TerrorHQSabotagePoints) have no key yet. A league can carry mods (every board
// plays the same set, and the ruleset fingerprint holds packets from a board
// whose rules differ), so they could be added. The arriving missiles and the
// S3-Sabre, and the effects an interplanetary attack adds to its battle, are
// already in Rules.Attacks.
type HQRules struct {
	// BuildStart is the completion percent a new HeadQuarters begins at, and
	// BuildPerTurn what it gains each turn until it reaches 100.
	BuildStart, BuildPerTurn int
	// BattleLossMin and BattleLossJitter: a lost defense costs the
	// HeadQuarters BattleLossMin + Random(BattleLossJitter) points.
	BattleLossMin, BattleLossJitter int
	// PriceBase, PricePerTurn and PriceJitter make the price: PriceBase +
	// PricePerTurn x turns played + Random(PriceJitter), held under PriceCap
	// minus Random(PriceCapJitter). Gold.
	PriceBase, PricePerTurn, PriceJitter, PriceCap, PriceCapJitter int64
}

// UnitRules are one unit's figures. Units with no production or price walk
// (agents, food) carry only NetWorth.
type UnitRules struct {
	// Cost is the production cost in points.
	Cost int
	// NetWorth is the unit's weight in net worth, in thousandths of a gold.
	NetWorth int64
	// UpkeepTenths is the gold one unit costs to maintain per turn, in tenths.
	UpkeepTenths int64
	// PriceLo, PriceHi and PriceStep are the band a realm's own price for the
	// unit walks inside, and how far it moves in a turn (stepPrice).
	PriceLo, PriceHi, PriceStep int
	// Offense and Defense are the unit's strength in a same-planet battle, in
	// hundredths of a trooper. A tank's are its strength with no HeadQuarters;
	// unitStrength raises them with HQ. A unit that cannot be sent on an attack
	// (turret, carrier) has no Offense. Pirate raids, an ally's aid and the
	// interplanetary resolvers value units by their own constants.
	Offense, Defense int64
	// HQBonus is the part of the unit's strength a point of HeadQuarters
	// completion adds, over TankStrengthPctBase (the curve's fixed reference):
	// hundredths of a trooper for a unit at the tank's base strength of 3.5.
	// Only tanks have one in the base rules.
	HQBonus int64
}

// baseRules is the original game, built once from the constants.
var baseRules = &Rules{Units: map[*Good]UnitRules{
	Trooper: {Cost: CostTrooper, NetWorth: NetWorthTrooper, UpkeepTenths: MaintTrooperTenths,
		PriceLo: PriceLoTrooper, PriceHi: PriceHiTrooper, PriceStep: PriceStepTrooper,
		Offense: OffenseTrooperPct, Defense: DefenseTrooperPct},
	Jet: {Cost: CostJet, NetWorth: NetWorthJet, UpkeepTenths: MaintJetTenths,
		PriceLo: PriceLoJet, PriceHi: PriceHiJet, PriceStep: PriceStepJet,
		Offense: OffenseJetPct},
	Turret: {Cost: CostTurret, NetWorth: NetWorthTurret, UpkeepTenths: MaintTurretTenths,
		PriceLo: PriceLoTurret, PriceHi: PriceHiTurret, PriceStep: PriceStepTurret,
		Defense: DefenseTurretPct},
	Bomber: {Cost: CostBomber, NetWorth: NetWorthBomber, UpkeepTenths: MaintBomberTenths,
		PriceLo: PriceLoBomber, PriceHi: PriceHiBomber, PriceStep: PriceStepBomber},
	Tank: {Cost: CostTank, NetWorth: NetWorthTank, UpkeepTenths: MaintTankTenths,
		PriceLo: PriceLoTank, PriceHi: PriceHiTank, PriceStep: PriceStepTank,
		Offense: TankStrengthPctBase, Defense: TankStrengthPctBase, HQBonus: TankStrengthPctPerHQ},
	Carrier: {Cost: CostCarrier, NetWorth: NetWorthCarrier, UpkeepTenths: MaintCarrierTenths,
		PriceLo: PriceLoCarrier, PriceHi: PriceHiCarrier, PriceStep: PriceStepCarrier},
	Agent: {NetWorth: NetWorthAgent},
	Food:  {},
},
	Regions: baseRegionRules(),
	Attacks: baseAttackEffects(),
	Combat:  baseCombatRules(),
	HQ: HQRules{
		BuildStart: HQBuildStart, BuildPerTurn: HQBuildPerTurn,
		BattleLossMin: HQBattleLossMin, BattleLossJitter: HQBattleLossJitter,
		PriceBase: HQPriceBase, PricePerTurn: HQPricePerTurn, PriceJitter: HQPriceJitter,
		PriceCap: HQPriceCap, PriceCapJitter: HQPriceCapJitter,
	},
}

// BaseRules returns a copy of the base rules for a mod to change.
func BaseRules() *Rules { return baseRules.Clone() }

// Clone is a copy of r whose tables can be changed without touching r's.
func (r *Rules) Clone() *Rules {
	c := *r
	c.Units, c.Regions = maps.Clone(r.Units), maps.Clone(r.Regions)
	c.RegionBrains, c.Attacks = maps.Clone(r.RegionBrains), maps.Clone(r.Attacks)
	return &c
}

// rules is the set w plays by: the base set unless mods were loaded.
func (w *World) rules() *Rules {
	if w.Rules == nil {
		return baseRules
	}
	return w.Rules
}

// unit is g's figures under w's rules.
func (w *World) unit(g *Good) UnitRules { return w.rules().Units[g] }

// The limits a mod's value must stay inside. They are tighter than the types
// allow on purpose: each is set at what keeps the engine's arithmetic safe on
// the 32-bit door builds, where a unit count times a figure is computed.
const (
	ModUnitCostMax      = 1_000_000
	ModUnitNetWorthMax  = 10_000_000 // thousandths: 10,000 gold of net worth a unit
	ModUnitUpkeepMax    = 10_000     // tenths: 1,000 gold a unit a turn
	ModUnitPriceMax     = 1_000_000
	ModUnitPriceBandMin = max(PriceFloorJitter, PriceCeilJitter) // stepPrice's clamps land inside a band this wide
	// Hundredths: 100 troopers' strength a unit. A unit count is at most 2^31,
	// so a strength sum is under 2^31 x 10^4 (x 4.5/3.5 for a tank at a full
	// HeadQuarters), about 2.8e13. unitStrength multiplies before it divides,
	// so its largest intermediate is 2^31 x 10^4 x 450, about 9.7e15: both are
	// int64, and the sum is held at the 32-bit ceiling before it reaches an int.
	ModUnitStrengthMax = 10_000
)

// Limits for the HeadQuarters figures. The price limits keep price_base +
// price_per_turn x turns played inside int64 for any realm that could exist, and
// the price itself inside an int on a 32-bit build; the percent figures are
// percent of a HeadQuarters that tops out at 100.
const (
	ModUnitHQBonusMax    = 10 // HQBonus: 0.10 a point against the tank's base 0.01
	ModHQPercentMax      = 100
	ModHQPriceMax        = 1_000_000_000
	ModHQPricePerTurnMax = 1_000_000
	ModHQPriceJitterMax  = 1_000_000
)

// HQField is one figure of HQRules as a mod names it: the key in a values.toml
// [hq] section, its limits, and how to read and write it. Validation and the mod
// reader go through HQFields, so a figure is named and bounded in one place.
// Wide marks an int64 figure, which a mod may set past what an int holds on the
// 32-bit door builds; the others are read through an int.
type HQField struct {
	Key      string
	Min, Max int64
	Wide     bool
	// Paired marks a figure checked against another rather than against Min
	// and Max alone (price_cap against price_base, its jitter against it).
	Paired bool
	Get    func(h HQRules) int64
	Set    func(h *HQRules, v int64)
	// Desc is one sentence saying what the figure means; the generated mod
	// reference prints it.
	Desc string
}

// HQFields are the figures a mod can set on the HeadQuarters, in the order
// -print-base-mod writes them and validation reports them.
var HQFields = []HQField{
	{"build_start", 1, ModHQPercentMax, false, false,
		func(h HQRules) int64 { return int64(h.BuildStart) },
		func(h *HQRules, v int64) { h.BuildStart = int(v) }, "Completion percent a newly bought HeadQuarters begins at."},
	{"build_per_turn", 1, ModHQPercentMax, false, false,
		func(h HQRules) int64 { return int64(h.BuildPerTurn) },
		func(h *HQRules, v int64) { h.BuildPerTurn = int(v) }, "Percent a HeadQuarters gains each turn until it reaches 100."},
	{"battle_loss_min", 0, ModHQPercentMax, false, false,
		func(h HQRules) int64 { return int64(h.BattleLossMin) },
		func(h *HQRules, v int64) { h.BattleLossMin = int(v) }, "Completion percent a HeadQuarters loses when its realm loses a defense."},
	{"battle_loss_jitter", 1, ModHQPercentMax, false, false,
		func(h HQRules) int64 { return int64(h.BattleLossJitter) },
		func(h *HQRules, v int64) { h.BattleLossJitter = int(v) }, "A random 0 up to one less than this is added to that loss."},
	{"price_base", 1, ModHQPriceMax, true, false,
		func(h HQRules) int64 { return h.PriceBase },
		func(h *HQRules, v int64) { h.PriceBase = v }, "Gold a HeadQuarters costs a realm that has played no turns."},
	{"price_per_turn", 0, ModHQPricePerTurnMax, true, false,
		func(h HQRules) int64 { return h.PricePerTurn },
		func(h *HQRules, v int64) { h.PricePerTurn = v }, "Gold the price rises with each turn the realm has played."},
	{"price_jitter", 0, ModHQPriceJitterMax, true, false,
		func(h HQRules) int64 { return h.PriceJitter },
		func(h *HQRules, v int64) { h.PriceJitter = v }, "A random 0 up to one less than this is added to the price. 0 makes the price exact."},
	{"price_cap", 0, ModHQPriceMax, true, true,
		func(h HQRules) int64 { return h.PriceCap },
		func(h *HQRules, v int64) { h.PriceCap = v }, "The price never rises past this, less its own random draw. At least `price_base`."},
	{"price_cap_jitter", 0, ModHQPriceMax, true, true,
		func(h HQRules) int64 { return h.PriceCapJitter },
		func(h *HQRules, v int64) { h.PriceCapJitter = v }, "A random 0 up to one less than this is taken off the cap. Less than `price_cap`."},
}

// HQFieldByKey is the field named key, or nil.
func HQFieldByKey(key string) *HQField {
	for i := range HQFields {
		if HQFields[i].Key == key {
			return &HQFields[i]
		}
	}
	return nil
}

// Validate reports every value outside its limits, so a sysop fixing a mod sees
// all of its mistakes at once rather than one per run.
func (r *Rules) Validate() error {
	var errs []error
	bad := func(g *Good, format string, a ...any) {
		errs = append(errs, fmt.Errorf("%s: %s", g.ID, fmt.Sprintf(format, a...)))
	}
	for _, g := range MilitaryGoods {
		u, ok := r.Units[g]
		if !ok {
			bad(g, "missing")
			continue
		}
		if u.Cost < 1 || u.Cost > ModUnitCostMax {
			bad(g, "build_cost %d is outside 1 to %d", u.Cost, ModUnitCostMax)
		}
		if u.NetWorth < 0 || u.NetWorth > ModUnitNetWorthMax {
			bad(g, "networth %s is outside 0 to %s", thousandths(u.NetWorth), thousandths(ModUnitNetWorthMax))
		}
		if u.UpkeepTenths < 0 || u.UpkeepTenths > ModUnitUpkeepMax {
			bad(g, "upkeep %s is outside 0 to %s", tenths(u.UpkeepTenths), tenths(ModUnitUpkeepMax))
		}
		if u.PriceLo < 1 || u.PriceHi > ModUnitPriceMax {
			bad(g, "price band %d to %d is outside 1 to %d", u.PriceLo, u.PriceHi, ModUnitPriceMax)
		}
		if u.PriceHi-u.PriceLo < ModUnitPriceBandMin {
			bad(g, "price_high must be at least %d above price_low", ModUnitPriceBandMin)
		}
		if u.Offense < 0 || u.Offense > ModUnitStrengthMax {
			bad(g, "offense %s is outside 0 to %s", hundredths(u.Offense), hundredths(ModUnitStrengthMax))
		}
		if g.Force == nil && u.Offense != 0 {
			bad(g, "offense must be 0: a %s is never sent on an attack", g.ID)
		}
		if u.Defense < 0 || u.Defense > ModUnitStrengthMax {
			bad(g, "defense %s is outside 0 to %s", hundredths(u.Defense), hundredths(ModUnitStrengthMax))
		}
		if u.HQBonus < 0 || u.HQBonus > ModUnitHQBonusMax {
			bad(g, "hq_bonus %s is outside 0 to %s", hundredths(u.HQBonus), hundredths(ModUnitHQBonusMax))
		}
		if u.PriceStep < 1 || u.PriceStep > u.PriceHi-u.PriceLo {
			bad(g, "price_step %d is outside 1 to the width of the band", u.PriceStep)
		}
	}
	for _, g := range RegionTypes {
		rr, ok := r.Regions[g]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: missing", g.ID))
			continue
		}
		errs = append(errs, rr.Validate(g)...)
	}
	r.validateHQ(func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("hq: %s", fmt.Sprintf(format, a...)))
	})
	r.validateCombat(func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("combat: %s", fmt.Sprintf(format, a...)))
	})
	r.validateAttacks(func(format string, a ...any) {
		errs = append(errs, fmt.Errorf(format, a...))
	})
	return errors.Join(errs...)
}

// validateHQ reports each HeadQuarters value outside its limits through bad.
// Every jitter that feeds Random(n) is at least 1 where the engine's helper
// would panic on 0; the price jitters may be 0 (a fixed price) because the
// draw they feed yields 0 for a bound of 0.
func (r *Rules) validateHQ(bad func(format string, a ...any)) {
	h := r.HQ
	for _, f := range HQFields {
		if f.Paired {
			continue // checked against each other below
		}
		if v := f.Get(h); v < f.Min || v > f.Max {
			bad("%s %d is outside %d to %d", f.Key, v, f.Min, f.Max)
		}
	}
	if h.PriceCap < h.PriceBase || h.PriceCap > ModHQPriceMax {
		bad("price_cap %d must be at least price_base (%d) and at most %d", h.PriceCap, h.PriceBase, ModHQPriceMax)
	}
	if h.PriceCapJitter < 0 || h.PriceCapJitter >= h.PriceCap {
		bad("price_cap_jitter %d is outside 0 to one below price_cap (%d)", h.PriceCapJitter, h.PriceCap)
	}
}

// thousandths and tenths print a fixed-point figure the way a mod file writes
// it, so a limit in an error message reads like the value that broke it.
func thousandths(v int64) string { return FixedPoint(v, 1000) }
func tenths(v int64) string      { return FixedPoint(v, 10) }
func hundredths(v int64) string  { return FixedPoint(v, 100) }

// FixedPoint prints v, a count of 1/scale units where scale is a power of ten,
// as a decimal with no trailing zeros: FixedPoint(250, 1000) is "0.25".
func FixedPoint(v, scale int64) string {
	places := len(fmt.Sprint(scale)) - 1
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	s := fmt.Sprintf("%s%d.%0*d", sign, v/scale, places, v%scale)
	for s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	return s
}

// ModRef records one mod a world was reset with, including a copy of its
// files: a game's rules change only at a reset, so the world goes on playing
// this copy even if the mod's folder is edited or removed (#202). Hash tells a
// load whether the folder still matches, so it can warn the sysop when not.
type ModRef struct {
	Name    string
	Version string
	Label   string `json:",omitempty"`
	Hash    string
	// Files are not scrubbed when they arrive in a league reset: their tabs and
	// newlines are part of what Hash covers, and none of it reaches a screen
	// unchecked, since the mod reader takes only the file names it knows and
	// refuses a control character in every manifest field it shows. The fields
	// above are scrubbed like the rest of the packet, so a Label or Version a
	// screen shows is safe whatever the order said.
	Files map[string]string `scrub:"-"`
	// Options are the values the sysop chose for this game, by option id.
	// Bools are 0 (False) or 1 (True). They travel with the mod in a league
	// reset order, so every board plays the same game. A missing id means the
	// mod's default, which keeps an order from an older game readable.
	// The keys are option ids checked against the mod's manifest before use,
	// and neither the id nor this map is shown unchecked on a screen.
	Options map[string]int64 `json:",omitempty" scrub:"-"`
}

// DisplayName is what a screen calls the mod: its label, or its name when it
// has none.
func (m ModRef) DisplayName() string {
	if m.Label != "" {
		return m.Label
	}
	return m.Name
}
