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
}

// baseRules is the original game, built once from the constants.
var baseRules = &Rules{Units: map[*Good]UnitRules{
	Trooper: {Cost: CostTrooper, NetWorth: NetWorthTrooper, UpkeepTenths: MaintTrooperTenths,
		PriceLo: PriceLoTrooper, PriceHi: PriceHiTrooper, PriceStep: PriceStepTrooper},
	Jet: {Cost: CostJet, NetWorth: NetWorthJet, UpkeepTenths: MaintJetTenths,
		PriceLo: PriceLoJet, PriceHi: PriceHiJet, PriceStep: PriceStepJet},
	Turret: {Cost: CostTurret, NetWorth: NetWorthTurret, UpkeepTenths: MaintTurretTenths,
		PriceLo: PriceLoTurret, PriceHi: PriceHiTurret, PriceStep: PriceStepTurret},
	Bomber: {Cost: CostBomber, NetWorth: NetWorthBomber, UpkeepTenths: MaintBomberTenths,
		PriceLo: PriceLoBomber, PriceHi: PriceHiBomber, PriceStep: PriceStepBomber},
	Tank: {Cost: CostTank, NetWorth: NetWorthTank, UpkeepTenths: MaintTankTenths,
		PriceLo: PriceLoTank, PriceHi: PriceHiTank, PriceStep: PriceStepTank},
	Carrier: {Cost: CostCarrier, NetWorth: NetWorthCarrier, UpkeepTenths: MaintCarrierTenths,
		PriceLo: PriceLoCarrier, PriceHi: PriceHiCarrier, PriceStep: PriceStepCarrier},
	Agent: {NetWorth: NetWorthAgent},
	Food:  {},
}}

// BaseRules returns a copy of the base rules for a mod to change.
func BaseRules() *Rules {
	return &Rules{Units: maps.Clone(baseRules.Units)}
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
			bad(g, "cost %d is outside 1 to %d", u.Cost, ModUnitCostMax)
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
		if u.PriceStep < 1 || u.PriceStep > u.PriceHi-u.PriceLo {
			bad(g, "price_step %d is outside 1 to the width of the band", u.PriceStep)
		}
	}
	return errors.Join(errs...)
}

// thousandths and tenths print a fixed-point figure the way a mod file writes
// it, so a limit in an error message reads like the value that broke it.
func thousandths(v int64) string { return FixedPoint(v, 1000) }
func tenths(v int64) string      { return FixedPoint(v, 10) }

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
	Files   map[string]string
}

// DisplayName is what a screen calls the mod: its label, or its name when it
// has none.
func (m ModRef) DisplayName() string {
	if m.Label != "" {
		return m.Label
	}
	return m.Name
}
