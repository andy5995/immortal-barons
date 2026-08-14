package game

// Pricing for the two interplanetary operations the sysop's cost Levels govern:
// an individual attack force (Config.AttackCosts) and a terrorist op
// (Config.TerrorCosts). Both quote the price before charging it, so the menu
// and the launcher must ask the same function.

// AttackGoldCost is what launching f as an individual interplanetary strike
// costs. BRE quotes it before asking to confirm ("This attack will cost 100
// gold."), scales it by the league's Attack Costs level, and clamps it at
// AttackCostCap.
func (w *World) AttackGoldCost(f AttackForce) int64 {
	cost := int64(f.units()) * IndividualAttackGoldPerUnit
	cost = cost * int64(w.Config.AttackCosts.CostPercent()) / 100
	if cost > AttackCostCap {
		return AttackCostCap
	}
	return cost
}

// TerrorOpGoldCost is what a terrorist op costs e to launch: BRE prices it off
// the launcher's own realm, at TerrorOpGoldPerRegion per region, scaled by the
// league's Terrorism Costs level. The price therefore climbs as a realm buys
// land, which is what stops a large empire spamming ops for free.
//
// No ceiling: BRE clamps the attack price at AttackCostCap, and nothing in the
// terrorist pricing routine does the same. Unverified rather than proven
// absent — the routine was read, but only as far as the level table and the
// per-region term.
func (w *World) TerrorOpGoldCost(e *Empire) int64 {
	cost := int64(e.Land) * TerrorOpGoldPerRegion
	return cost * int64(w.Config.TerrorCosts.CostPercent()) / 100
}
