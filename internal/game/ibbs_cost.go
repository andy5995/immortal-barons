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

// TerrorOpGoldRate is what ONE agent on a terrorist op costs e, the figure the
// InterPlanetary menu prints beside the item. It climbs as a realm buys land
// (stopping large empires from spamming ops for free) and as more ops go out
// that day.
//
// DELIBERATE DIVERGENCE: it is the one-agent charge, so the menu and the bill
// always agree. The original quotes from a separate routine
// (ovr_02aca8_entry_0000, BRE.OVR) in integer arithmetic, clamping the day's
// count to 1..100 first:
//
//	quote := (clamp(opsToday, 1, 100) + 63) × totalRegions, then ×1 / ×0 / ÷5 / ×3
//
// while the charge (below) clamps nothing. So the original advertises the
// day's first op at 64 a region and bills 63, stops advertising a rise past
// the hundredth op while the bill keeps climbing, and on Low or High can quote
// a gold piece off the Real48 bill. IB quotes the bill.
//
// No ceiling: BRE clamps the attack price at AttackCostCap, and nothing in
// the terrorist pricing routine does the same.
func (w *World) TerrorOpGoldRate(e *Empire) int64 {
	return w.TerrorOpGoldCost(e, 1)
}

// TerrorOpGoldCost is what sending agents costs. Each agent is one operation —
// it pays its own way and takes its own slot out of the day's allowance —
// which is why the original's prompt counts DOWN the allowance rather than the
// agents held.
//
// BINARY-VERIFIED (ovr_02aca8_proc_00e5, BRE.OVR 0x2ad8d, called by
// launch_terrorist_operation with the day's count, 0 and the agents sent):
//
//	charge := trunc(totalRegions / 100 × (opsToday + 63) × agents × levelPct)
//
// in Real48, left to right, each step rounded to the runtime's 40 bits; the
// routine computes the same product for 0 agents and subtracts it, which adds
// nothing. opsToday is NOT clamped here, at either end. The level is a percent
// (CostLevel*Pct). Because the regions are divided before anything is
// multiplied, the bill can differ by a gold piece from the exact product:
// 8,957 regions at High is 1,692,872 for one agent, where the arithmetic says
// 1,692,873. IB floored a per-agent rate and multiplied it by the count until
// 2026-10-01, which on Low under-billed a large send by up to a gold piece per
// agent.
//
// CAPTURE-VERIFIED against `cap/eots-ibbs-02.cap`, four sends whose charges
// this reproduces to the gold:
//
//	8 agents, 0 ops used, 8,957 regions -> 4,514,328
//	7 agents, 8 ops used, 8,957 regions -> 4,451,629
//	7 agents, 0 ops used, 6,835 regions -> 3,014,235
//	8 agents, 7 ops used, 6,835 regions -> 3,827,600
func (w *World) TerrorOpGoldCost(e *Empire, agents int) int64 {
	pct := int64(w.Config.TerrorCosts.CostPercent())
	if pct == 0 || agents <= 0 || e.Land <= 0 {
		return 0
	}
	perRegion := int64(e.TerrorOpsToday) + TerrorOpGoldPerRegion
	x := r48Div(r48Int(int64(e.Land)), r48Int(100))
	x = r48Mul(x, r48Int(perRegion))
	x = r48Mul(x, r48Int(int64(agents)))
	x = r48Mul(x, r48Int(pct))
	return r48Trunc(x)
}
