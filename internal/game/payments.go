package game

// Per-turn maintenance obligations. In BRE these are prompted at the start
// of each turn ("Your Armed Forces Require N / How much will you give?",
// "N gold is required to maintain your regions", "N gold is requested to
// boost popular support"). The prompt labels and non-payment consequences
// (desertion, revolt) come from the original's strings; the exact numeric
// rates live in compiled code, so the constants below are reconstructed and
// tunable.
const (
	RegionUpkeepPerLand = 2   // gold per region of land, per turn
	DesertRate          = 25  // % of the army that deserts at full non-payment
	RegionRevoltRate    = 15  // % of land that revolts at full non-payment
	SupportPerBoostGold = 100 // gold to raise popular support by one point
)

// ForcesUpkeep is the gold the armed forces require this turn. Technology
// regions lower it (same factor income uses); this is the formula the old
// auto-deducted maintenance used.
func (e *Empire) ForcesUpkeep() int {
	return (e.Troopers*6 + e.Jets*12 + e.Turrets*9 + e.Tanks*6 + e.Carriers*1) * (100 - e.techFactor()) / 100
}

// RegionUpkeep is the gold required to maintain the empire's regions.
func (e *Empire) RegionUpkeep() int { return e.Land * RegionUpkeepPerLand }

// clampGive limits a payment to what the empire can actually afford and
// deducts it, recording it in the turn's LastGoldPaid tally. Returns the
// amount actually paid.
func (e *Empire) clampGive(given int) int {
	if given > e.Gold {
		given = e.Gold
	}
	if given < 0 {
		given = 0
	}
	e.Gold -= given
	e.LastGoldPaid += given
	return given
}

// PayForces applies a payment toward armed-forces upkeep. A shortfall makes
// units desert proportionally and lowers popular support. Returns the number
// of units lost to desertion.
func (w *World) PayForces(e *Empire, given int) int {
	req := e.ForcesUpkeep()
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	fracPct := (req - given) * 100 / req // req > 0 here (else given >= req)
	desertPct := fracPct * DesertRate / 100
	lost := 0
	desert := func(n *int) {
		d := *n * desertPct / 100
		*n -= d
		lost += d
	}
	desert(&e.Troopers)
	desert(&e.Jets)
	desert(&e.Turrets)
	desert(&e.Tanks)
	e.adjustSupport(-fracPct / 5)
	return lost
}

// PayRegions applies a payment toward region maintenance. A shortfall makes
// regions revolt (land is lost) and lowers popular support. Returns the
// number of regions lost.
func (w *World) PayRegions(e *Empire, given int) int {
	req := e.RegionUpkeep()
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	fracPct := (req - given) * 100 / req // req > 0 here (else given >= req)
	lost := e.Land * fracPct / 100 * RegionRevoltRate / 100
	if lost > 0 {
		e.Regions.remove(lost)
		e.syncLand()
	}
	e.adjustSupport(-fracPct / 10)
	return lost
}

// BoostSupport spends gold to raise popular support (the optional
// "requested" obligation). Returns the number of support points gained.
func (w *World) BoostSupport(e *Empire, given int) int {
	given = e.clampGive(given)
	pts := given / SupportPerBoostGold
	e.adjustSupport(pts)
	return pts
}

// adjustSupport moves support by delta, clamped to [0, 100].
func (e *Empire) adjustSupport(delta int) {
	e.Support += delta
	if e.Support < 0 {
		e.Support = 0
	}
	if e.Support > 100 {
		e.Support = 100
	}
}
