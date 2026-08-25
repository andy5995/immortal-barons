package game

import "math"

// tech.go — the Technology research slots and the factors they scale. The
// curve and which slot feeds which effect are BRE-verified; the figures are in
// balance_costs.go.

// techFactor is the multiplier slot `slot` currently grants, given that effect's
// ceiling in hundredths (150 == x1.50), returned in TechFactorUnit fixed point.
//
// BRE: factor = 1 + (cap-1) * (1 - exp(-level / (totalRegions+1))). The
// denominator is why expanding dilutes technology, and why the same research
// reads as more effective in a smaller realm.
func (e *Empire) techFactor(slot, capPct int) int {
	lvl := e.TechSlots[slot]
	total := e.Regions.Total()
	if lvl <= 0 || capPct <= 100 || total < 0 {
		return TechFactorUnit
	}
	x := float64(lvl) / float64(total+1)
	if x > TechExpClamp {
		x = TechExpClamp
	}
	f := 1 + (float64(capPct)/100-1)*(1-math.Exp(-x))
	return int(f * TechFactorUnit)
}

// The eight Technology effects. Three of them share slot 0 with different
// ceilings, exactly as the original does.
func (e *Empire) TechGoldFactor() int { return e.techFactor(TechSlotGold, TechCapGold) }

func (e *Empire) TechFoodFactor() int { return e.techFactor(TechSlotGold, TechCapFood) }

func (e *Empire) TechUnitFactor() int { return e.techFactor(TechSlotGold, TechCapUnits) }

func (e *Empire) TechSDIFactor() int { return e.techFactor(TechSlotSDI, TechCapSDI) }

func (e *Empire) TechTaxFactor() int { return e.techFactor(TechSlotTax, TechCapTax) }

func (e *Empire) TechMaintFactor() int { return e.techFactor(TechSlotMaint, TechCapMaint) }

func (e *Empire) TechDecayFactor() int { return e.techFactor(TechSlotDecay, TechCapDecay) }

func (e *Empire) TechMilitaryFactor() int {
	return e.techFactor(TechSlotMilitary, TechCapMilitary)
}

// techRaise scales v up by a Technology factor; techLower scales it down (used
// for costs and decay, which the original divides by the factor rather than
// multiplying).
// int64 intermediates: both multiply by a factor around 10,000, so any v past
// ~215,000 overflows int32 on a 32-bit build. Region upkeep reaches that at 235
// regions — one report on a 32-bit door showed a 710-region realm billed
// -210,763 gold. The 64-bit build was never affected, which is why it survived
// so long. Same for NetWorth (see World.NetWorth).
func techRaise[T number](v T, factor int) T { return T(int64(v) * int64(factor) / TechFactorUnit) }

func techLower[T number](v T, factor int) T { return T(int64(v) * TechFactorUnit / int64(factor)) }

// TechPercent renders a factor the way the in-game advisor does: a raised effect
// as round(100*f), a lowered one as round(100/f).
func TechPercent(factor int, lowered bool) int {
	if lowered {
		return (TechFactorUnit*100 + factor/2) / factor
	}
	return (factor*100 + TechFactorUnit/2) / TechFactorUnit
}

// advanceTech runs one turn of research. Called once per turn played.
//
// BINARY-VERIFIED against BRE.OVR process_economic_production (unit ovr_033b64
// +0x2fe): points are quadratic in Technology regions and only inverse-linear in
// realm size, so a dense tech block in a small realm out-researches the same
// block in a large one. Each point lands in a slot chosen uniformly at random,
// and nine of the fifteen slots do nothing — the waste is the mechanic, not a
// bug.
//
// The partner term is the same expression over the SAME denominator — the
// researcher's own total regions, not the partner's — bounded by the smaller of
// the two Technology REGION counts, and it does not get the multiplier. The
// original walks realms A..Y, skips its own slot, takes the relation from the
// researcher's own row, and skips a partner that holds no Technology or whose
// record slot is not in use; alliesOf's Alive filter is that second gate. Both
// terms round separately before they are summed.
//
// With no Technology regions this returns immediately: research stops and the
// banked levels FREEZE. They are never decremented anywhere.
func (w *World) advanceTech(e *Empire) {
	tech := e.Regions.Technology
	total := e.Regions.Total()
	if tech <= 0 || total <= 0 {
		return
	}
	points := TechResearchMul * techResearchPoints(tech, total)
	for _, ally := range w.alliesOf(e, "Technology Agreement") {
		points += techResearchPoints(min(tech, ally.Regions.Technology), total)
	}
	for range points {
		e.TechSlots[w.rng.Intn(TechSlotCount)]++
	}
}

// techResearchPoints is round((n^2 / total)^0.75), the original's own shape. It
// computes the power as exp(ln(x) * 3 / 4) and rounds half away from zero, which
// is what math.Pow and math.Round do here.
func techResearchPoints(n, total int) int {
	if n <= 0 || total <= 0 {
		return 0
	}
	return int(math.Round(math.Pow(float64(n)*float64(n)/float64(total), 0.75)))
}
