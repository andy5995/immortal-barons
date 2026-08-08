package game

import (
	"fmt"
)

// Strike/SDI gold costs (NukeCost, ChemCost, BioCost, AnnihilatorCost, SDIStep)
// live in balance.go. SDIMax is a level cap, not a cost, so it stays here.
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
func (w *World) SDIMaintenance(e *Empire) int { return pctOf(e.SDIFunding, SDIMaintPct) }

// SDIFundingPerRegion is the program's spending spread over the land it covers,
// which the original prints on the same screen. It printed 0 there in every
// capture, at every funding level — unexplained, and most likely a defect in the
// original, so IB shows the figure the label describes.
func (w *World) SDIFundingPerRegion(e *Empire) int {
	if e.Land <= 0 {
		return 0
	}
	return e.SDIFunding / e.Land
}

// SDISpendAllowance is the most gold e may still put into the program this turn:
// a share of what is already in it, never less than the floor, less whatever has
// gone in already this turn.
func (w *World) SDISpendAllowance(e *Empire) int {
	allowed := pctOf(e.SDIFunding, SDISpendPct)
	if allowed < SDIMinSpend {
		allowed = SDIMinSpend
	}
	return max(0, allowed-e.TurnProgress.SDIFunded)
}

// FundSDI puts gold into the program, in whole thousands and no more than the
// turn's allowance allows. Returns the new SDI strength.
func (w *World) FundSDI(e *Empire, gold int) (int, error) {
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
func (w *World) PaySDI(e *Empire, gold int) {
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
	e.SDIFunding = int(int64(e.SDIFunding) * int64(gold) / int64(due))
	e.syncSDI()
}

// syncSDI recomputes the strength from the funding total. Every place that
// changes SDIFunding must call it.
func (e *Empire) syncSDI() {
	e.SDI = min(SDIMax, e.SDIFunding/SDIStep)
}

// EnsureSDIFunding backfills the funding pool on a save written before the
// program kept one. Without it the first thing to call syncSDI would read a
// zero pool and wipe an SDI the player had already paid for.
func (e *Empire) EnsureSDIFunding() {
	if e.SDIFunding == 0 && e.SDI > 0 {
		e.SDIFunding = e.SDI * SDIStep
	}
	e.syncSDI() // funding is the authoritative figure once it exists
}

// NuclearStrike destroys some of the defender's regions, turning them to
// waste. It is a v1 gold-cost strike (no missile inventory yet).
func (w *World) NuclearStrike(a, d *Empire) (string, error) {
	if a.Gold < NukeCost {
		return "", ErrCantAfford
	}
	a.Gold -= NukeCost

	regions := w.jitter(d.Land/10) + 1
	regions = regions * (100 - d.SDI) / 100
	if regions > d.Land {
		regions = d.Land
	}
	d.Regions.remove(regions)
	d.syncLand()
	if d.Land <= 0 || d.People <= 0 {
		d.Alive = false
		d.DiedDay = w.GameDay
	}

	d.addEvent(fmt.Sprintf("%s hit you with a nuclear strike: %d regions reduced to waste.", a.Name, regions))

	w.postStrikeNews(a, d, "nuclear")
	report := fmt.Sprintf("Nuclear strike! %d regions of %s reduced to waste.", regions, d.Name)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
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
