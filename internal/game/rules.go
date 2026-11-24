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
}

// HQRules are the HeadQuarters figures. The strength it adds to each unit is a
// unit's figure (UnitRules.HQBonus). What only interplanetary play does to
// a HeadQuarters is not here — the resolver's own tank curve
// (RemoteTankStrength*), the fifth a winning invasion takes
// (InvasionHQLossDivisor) and a landed Sabotage HQ agent's points
// (TerrorHQSabotagePoints): those only happen in a league, and a league board
// refuses mods, so a key for them could never take effect.
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
	HQ: HQRules{
		BuildStart: HQBuildStart, BuildPerTurn: HQBuildPerTurn,
		BattleLossMin: HQBattleLossMin, BattleLossJitter: HQBattleLossJitter,
		PriceBase: HQPriceBase, PricePerTurn: HQPricePerTurn, PriceJitter: HQPriceJitter,
		PriceCap: HQPriceCap, PriceCapJitter: HQPriceCapJitter,
	},
}

// BaseRules returns a copy of the base rules for a mod to change.
func BaseRules() *Rules {
	return &Rules{Units: maps.Clone(baseRules.Units), Regions: maps.Clone(baseRules.Regions), HQ: baseRules.HQ}
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
	return errors.Join(errs...)
}

// validateHQ reports each HeadQuarters value outside its limits through bad.
// Every jitter that feeds Random(n) is at least 1 where the engine's helper
// would panic on 0; the price jitters may be 0 (a fixed price) because the
// draw they feed yields 0 for a bound of 0.
func (r *Rules) validateHQ(bad func(format string, a ...any)) {
	h := r.HQ
	if h.BuildStart < 1 || h.BuildStart > ModHQPercentMax {
		bad("build_start %d is outside 1 to %d", h.BuildStart, ModHQPercentMax)
	}
	if h.BuildPerTurn < 1 || h.BuildPerTurn > ModHQPercentMax {
		bad("build_per_turn %d is outside 1 to %d", h.BuildPerTurn, ModHQPercentMax)
	}
	if h.BattleLossMin < 0 || h.BattleLossMin > ModHQPercentMax {
		bad("battle_loss_min %d is outside 0 to %d", h.BattleLossMin, ModHQPercentMax)
	}
	if h.BattleLossJitter < 1 || h.BattleLossJitter > ModHQPercentMax {
		bad("battle_loss_jitter %d is outside 1 to %d", h.BattleLossJitter, ModHQPercentMax)
	}
	if h.PriceBase < 1 || h.PriceBase > ModHQPriceMax {
		bad("price_base %d is outside 1 to %d", h.PriceBase, ModHQPriceMax)
	}
	if h.PricePerTurn < 0 || h.PricePerTurn > ModHQPricePerTurnMax {
		bad("price_per_turn %d is outside 0 to %d", h.PricePerTurn, ModHQPricePerTurnMax)
	}
	if h.PriceJitter < 0 || h.PriceJitter > ModHQPriceJitterMax {
		bad("price_jitter %d is outside 0 to %d", h.PriceJitter, ModHQPriceJitterMax)
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
}

// DisplayName is what a screen calls the mod: its label, or its name when it
// has none.
func (m ModRef) DisplayName() string {
	if m.Label != "" {
		return m.Label
	}
	return m.Name
}
