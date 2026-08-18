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
// the launcher's own realm, scaled by the league's Terrorism Costs level.
// The price climbs as a realm buys land (stopping large empires from spamming
// ops for free) and as more ops are launched that day.
//
// BINARY-VERIFIED formula (ovr_02aca8_entry_0000, BRE.OVR):
//
//	capped := clamp(terrorOpsToday, 1, 100)
//	cost   := (capped + 63) * totalRegions * configMult
//
// For opsToday ≤ 1 the per-region cost is TerrorOpGoldPerRegion (64); each
// subsequent op raises it by 1, up to 163 at the cap of 100.
//
// No ceiling: BRE clamps the attack price at AttackCostCap, and nothing in
// the terrorist pricing routine does the same.
func (w *World) TerrorOpGoldCost(e *Empire) int64 {
	ops := int64(e.TerrorOpsToday)
	switch {
	case ops < 1:
		ops = 1
	case ops > 100:
		ops = 100
	}
	cost := (ops + 63) * int64(e.Land)
	return cost * int64(w.Config.TerrorCosts.CostPercent()) / 100
}
