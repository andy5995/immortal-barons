package game

import "maps"

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
