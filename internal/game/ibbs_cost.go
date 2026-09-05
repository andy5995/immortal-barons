package game

// Pricing for the two interplanetary operations the sysop's cost Levels govern:
// an individual attack force (Config.AttackCosts) and a terrorist op
// (Config.TerrorCosts). Both quote the price before charging it, so the menu
// and the launcher must ask the same function.

// AttackGoldCost is what sending f across space costs e. BRE quotes it before
// asking to confirm ("This attack will cost 100 gold."), scales it by the
// league's Attack Costs level, and clamps it at AttackCostCap.
//
// BINARY-VERIFIED (`configure_attack_forces`, BRE.OVR 0x02b83c). The price is
// per unit type and rises with the LAUNCHER's own size, the same self-limiting
// shape the terror-op price has:
//
//	cost = Σ (totalRegions/divisor + add) × count, over the four types
//
// with the divisor and addend from AttackPricePerUnit. Then the Attack Costs
// level (None 0, Low 20, Medium 100, High 300 percent — real_divide by 5 at
// 0x481 and real_multiply by 3 at 0x4ab), then the AttackCostCap clamp at
// 0x4b9, and only then the truncation to whole gold.
//
// Confirmed against `cap/eots-ibbs-02.cap`: a realm of 9,003 regions joining a
// group attack with 12,141 troopers, 12,378,520 jets, no tanks and 105,520
// bombers is quoted 99,572,437 gold, which this reproduces exactly.
//
// ONE routine prompts for the four counts and prices them for all three paths —
// its callers are `create_group_attack`, `create_individual_attack` and
// `run_interbbs_attack_menu` — so joining a group is charged the same way as
// sending a strike alone (#252).
func (w *World) AttackGoldCost(e *Empire, f AttackForce) int64 {
	regions := int64(e.Regions.Total())
	var cost int64
	for _, u := range AttackPricePerUnit {
		// The per-unit rate is FRACTIONAL — the original holds it in a Real48 — so
		// the count multiplies before the divide. Dividing first floors the rate
		// and then multiplies the error by the whole detachment: the capture's
		// 12.4 million jets would come out 24,757 gold short on their own.
		n := int64(u.Count(f))
		cost += regions*n/u.Divisor + u.Add*n
	}
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
	cost := (ops + TerrorOpGoldPerRegion - 1) * int64(e.Land)
	return cost * int64(w.Config.TerrorCosts.CostPercent()) / 100
}
