package game

import (
	"fmt"
)

// Strike/SDI gold costs (ChemCost, BioCost, AnnihilatorCost, SDIStep, and the
// nuclear pricing constants) live in balance.go; the nuclear price itself is
// computed per target, by NukeCostForLand below. SDIMax is a level cap, not a
// cost, so it stays here.
// SDIMax caps SDI at 50%: BRE's SDI destroys "up to 50%" of incoming missiles
// (breins.txt), so the level tops out there.
const SDIMax = 50

// The SDI program is a pot of gold rather than a level bought outright: gold
// goes in a turn at a time against an allowance, the pot carries an upkeep every
// turn, and the strength is read off the total. The allowance and the upkeep are
// the original's (see balance.go); how the total converts to a percentage is
// NOT — the captured game's region count moved under it too much to read a curve
// off seventeen points, so IB keeps its own SDIStep until the original's rule is
// disassembled.

// SDIMaintenance is the upkeep e owes on its program this turn.
func (w *World) SDIMaintenance(e *Empire) int64 { return pctOf(e.SDIFunding, SDIMaintPct) }

// SDIFundingPerRegion is the program's spending spread over the land it covers,
// which the original prints on the same screen. It printed 0 there in every
// capture, at every funding level — unexplained, and most likely a defect in the
// original, so IB shows the figure the label describes.
func (w *World) SDIFundingPerRegion(e *Empire) int64 {
	if e.Land <= 0 {
		return 0
	}
	return e.SDIFunding / int64(e.Land)
}

// SDISpendAllowance is the most gold e may still put into the program this turn:
// a share of what is already in it, never less than the floor, less whatever has
// gone in already this turn.
func (w *World) SDISpendAllowance(e *Empire) int64 {
	allowed := pctOf(e.SDIFunding, SDISpendPct)
	if allowed < SDIMinSpend {
		allowed = SDIMinSpend
	}
	return max(0, allowed-e.TurnProgress.SDIFunded)
}

// FundSDI puts gold into the program, in whole thousands and no more than the
// turn's allowance allows. Returns the new SDI strength.
func (w *World) FundSDI(e *Empire, gold int64) (int, error) {
	gold = min(gold, w.SDISpendAllowance(e))
	gold -= gold % SDIIncrement
	if gold <= 0 {
		return e.SDI, nil
	}
	if e.Gold < gold {
		return 0, ErrCantAfford
	}
	e.Gold -= gold
	e.SDIFunding += gold
	e.TurnProgress.SDIFunded += gold
	e.syncSDI()
	return e.SDI, nil
}

// PaySDI settles the turn's upkeep with the gold given. Paying short scales the
// program back to what was funded — the original was never observed underpaying
// it, so the shortfall rule is IB's: without one the upkeep would be optional,
// and a program nobody maintains would defend as well as one they do.
func (w *World) PaySDI(e *Empire, gold int64) {
	due := w.SDIMaintenance(e)
	gold = min(gold, e.Gold)
	if gold < 0 {
		gold = 0
	}
	e.Gold -= gold
	if gold >= due || due <= 0 {
		return
	}
	// int64 throughout: a program funded into the millions times a part-payment
	// in the thousands passes 2^31, which wrapped to a negative funding total on
	// a 32-bit door.
	e.SDIFunding = e.SDIFunding * gold / due
	e.syncSDI()
}

// syncSDI recomputes the strength from the funding total. Every place that
// changes SDIFunding must call it.
func (e *Empire) syncSDI() {
	e.SDI = min(SDIMax, int(e.SDIFunding/SDIStep))
}

// EnsureSDIFunding backfills the funding pool on a save written before the
// program kept one. Without it the first thing to call syncSDI would read a
// zero pool and wipe an SDI the player had already paid for.
func (e *Empire) EnsureSDIFunding() {
	if e.SDIFunding == 0 && e.SDI > 0 {
		e.SDIFunding = int64(e.SDI) * SDIStep
	}
	e.syncSDI() // funding is the authoritative figure once it exists
}

// NukeCostForLand is what a nuclear missile aimed at a realm of `land` regions
// costs. The arms dealer prices the weapon off the target: a bigger realm needs
// a bigger warhead, up to a cap (see balance.go). Taking land rather than an
// *Empire lets the attack screen price the shot from its target list without
// re-resolving the rival under the lock.
func NukeCostForLand(land int) int64 {
	return min(int64(land)*NukeCostPerRegion, NukeCostCap)
}

// NukeCost is what a nuclear missile aimed at d costs.
func (w *World) NukeCost(d *Empire) int64 { return NukeCostForLand(d.Land) }

// NuclearStrike ruins a band of the defender's regions, converting them to
// waste. No land changes hands and nobody dies: the defender keeps every region
// on its books, still pays upkeep on them, and earns nothing from them until
// they are decontaminated (see DecontaminateAllowance).
//
// The strike is not intercepted. Nothing in the original's local nuclear path
// consults the target's SDI or turrets — SDI shoots down interplanetary
// missiles, not a neighbour's.
func (w *World) NuclearStrike(a, d *Empire) (string, error) {
	cost := w.NukeCost(d)
	if a.Gold < cost {
		return "", ErrCantAfford
	}
	a.Gold -= cost

	// 7% ± a two-draw jitter, so the band is 5-9% and the extremes are rarer
	// than the middle.
	pct := NukeWastePct + w.rng.Intn(NukeWasteJitter) - w.rng.Intn(NukeWasteJitter)
	// remove spreads the loss over every type the target holds, waste included,
	// and the same count goes back on as waste — so land already ruined absorbs
	// part of the strike and the realm's total never moves.
	regions := d.Regions.remove(d.Land * pct / 100).Total()
	d.Regions.Waste += regions
	d.syncLand()

	// The award is a flat draw, not a share of the damage: a strike on a small
	// realm that ruined nothing still scores.
	addScore(a, w.rng.Intn(NukeScoreAward))

	d.addEvent(fmt.Sprintf("%s hit you with a nuclear strike: %d regions reduced to waste.", a.Name, regions))

	w.postStrikeNews(a, d, "nuclear")
	return fmt.Sprintf("Nuclear strike! %d regions of %s are now waste.", regions, d.Name), nil
}

// DecontaminateAllowance is the most waste regions e may clean this turn: a
// share of the pile, floored so a small mess is never left to trickle, and
// never more waste than it actually holds.
func (w *World) DecontaminateAllowance(e *Empire) int {
	if e.Regions.Waste <= 0 {
		return 0
	}
	return min(max(e.Regions.Waste/WasteDecontamDivisor, WasteDecontamFloor), e.Regions.Waste)
}

// DecontaminatePrice is the gold each cleaned region costs — the going region
// price, halved, and reduced further by technology.
//
// The factor is the FOOD one, unintuitively: the original divides by
// `technology_factor(2.0, slot 0)`, which is the pair its agricultural yield
// uses, not the (1.4, slot 3) pair its maintenance costs use. Reading the four
// sibling call sites is what settled it — the cost being maintenance-shaped is
// not evidence about which research pays for it.
//
// One truncation, not two: the original divides the price by twice the factor
// in a single real expression, so halving first would round a second time.
func (w *World) DecontaminatePrice(e *Empire) int64 {
	return int64(w.LandPrice(e)) * TechFactorUnit /
		(WasteDecontamPriceDiv * int64(e.TechFoodFactor()))
}

// DecontaminateCost is the bill for cleaning this turn's whole allowance.
func (w *World) DecontaminateCost(e *Empire) int64 {
	return int64(w.DecontaminateAllowance(e)) * w.DecontaminatePrice(e)
}

// Decontaminate spends `gold` on cleaning waste and returns the regions
// restored. Paying short cleans proportionally less rather than nothing — the
// original divides the gold given by the full bill and scales the count by it,
// so a baron who cannot cover the whole allowance still makes progress.
//
// Cleaned land has no type of its own in the original, which parks it in a pool
// of unallocated regions until the owner names the types. IB has no such pool,
// so the land comes back as Coastal and a caller with someone at the keyboard
// offers to re-type it (see allocateDecontaminated). Restoring it here rather
// than leaving it untyped is deliberate: gold has already changed hands, and a
// session that drops between the payment and the picker must not take the land
// with it.
func (w *World) Decontaminate(e *Empire, gold int64) int {
	price := w.DecontaminatePrice(e)
	gold = min(gold, e.Gold)
	if gold <= 0 || price <= 0 {
		return 0
	}
	n := min(int(gold/price), w.DecontaminateAllowance(e))
	if n <= 0 {
		return 0
	}
	e.Gold -= int64(n) * price
	e.Regions.Waste -= n
	e.Regions.Coastal += n
	e.syncLand()
	return n
}

// ChemicalStrike kills people and troopers and damages some land.
func (w *World) ChemicalStrike(a, d *Empire) (string, error) {
	if a.Gold < ChemCost {
		return "", ErrCantAfford
	}
	a.Gold -= ChemCost

	people := w.jitter(d.People*15/100) * (100 - d.SDI) / 100
	troops := w.jitter(d.Troopers*20/100) * (100 - d.SDI) / 100
	regions := d.Land / 20 * (100 - d.SDI) / 100

	people, troops, regions = clamp(d.People, people), clamp(d.Troopers, troops), clamp(d.Land, regions)
	d.People -= people
	d.Troopers -= troops
	d.Regions.remove(regions)
	d.syncLand()
	// The gas also breaks the survivors: a quarter off military morale and a
	// third off popular support (binary-verified, BRE.OVR 0x110AE / 0x11109).
	d.Morale = roundDiv(d.Morale*ChemMoraleKeepNum, ChemMoraleKeepDen)
	d.Support = roundDiv(d.Support*WMDSupportKeepNum, WMDSupportKeepDen)

	if d.Land <= 0 || d.People <= 0 {
		d.Alive = false
		d.DiedDay = w.GameDay
	}

	d.addEvent(fmt.Sprintf("%s hit you with a chemical strike: %d people, %d troopers, and %d regions lost.", a.Name, people, troops, regions))

	w.postStrikeNews(a, d, "chemical")
	report := fmt.Sprintf("Chemical strike! %s lost %d people, %d troopers, and %d regions.", d.Name, people, troops, regions)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
}

// BiologicalStrike kills people and troopers but leaves land untouched.
func (w *World) BiologicalStrike(a, d *Empire) (string, error) {
	if a.Gold < BioCost {
		return "", ErrCantAfford
	}
	a.Gold -= BioCost

	people := w.jitter(d.People*15/100) * (100 - d.SDI) / 100
	troops := w.jitter(d.Troopers*20/100) * (100 - d.SDI) / 100

	people, troops = clamp(d.People, people), clamp(d.Troopers, troops)
	d.People -= people
	d.Troopers -= troops
	// A plague is worse for the army than gas and the same for the public: morale
	// halved, support cut by a third (BRE.OVR 0x115FE / 0x11645).
	d.Morale /= BioMoraleDivisor
	d.Support = roundDiv(d.Support*WMDSupportKeepNum, WMDSupportKeepDen)

	if d.People <= 0 {
		d.Alive = false
		d.DiedDay = w.GameDay
	}

	d.addEvent(fmt.Sprintf("%s hit you with a biological strike: %d people and %d troopers lost.", a.Name, people, troops))

	w.postStrikeNews(a, d, "biological")
	report := fmt.Sprintf("Biological strike! %s lost %d people and %d troopers.", d.Name, people, troops)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
}

// clamp caps n so subtracting it from total never goes negative.
func clamp(total, n int) int {
	if n > total {
		return total
	}
	if n < 0 {
		return 0
	}
	return n
}
