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

// The arms dealer prices every warhead off the TARGET, so the three functions
// below take the target's figures rather than an *Empire: the attack screen can
// then quote a price straight from its snapshotted target list, without
// re-resolving the rival under the world lock.

// NukeCostForLand is what a nuclear missile aimed at a realm of `land` regions
// costs — a bigger realm needs a bigger warhead, up to the shared cap.
func NukeCostForLand(land int) int64 {
	return min(int64(land)*NukeCostPerRegion, StrikeCostCap)
}

// ChemCostForTarget is what a chemical missile aimed at a realm of `people`
// people holding `land` regions costs. The population term is what makes a
// crowded realm expensive to gas.
func ChemCostForTarget(people, land int) int64 {
	return min(brePop(people)*ChemCostPerPop+int64(land)*ChemCostPerRegion, StrikeCostCap)
}

// BioCostForTarget is what a biological missile aimed at a realm of `troopers`
// troopers and `people` people holding `land` regions costs. All three of the
// figures it hurts are in its price.
func BioCostForTarget(troopers, people, land int) int64 {
	return min(int64(troopers)*BioCostPerTrooper+
		brePop(people)*BioCostPerPop+
		int64(land)*BioCostPerRegion, StrikeCostCap)
}

// brePop converts IB's head count into the original's unit, which is one
// million people. Every per-population PRICE below was read out of a binary
// that counts in millions, so it has to be applied to the converted figure;
// the damage percentages need no conversion, being unit-free.
func brePop(people int) int64 { return int64(people) / PopBREUnitScale }

// NukeCost is what a nuclear missile aimed at d costs.
func (w *World) NukeCost(d *Empire) int64 { return NukeCostForLand(d.Land) }

// ChemCost is what a chemical missile aimed at d costs.
func (w *World) ChemCost(d *Empire) int64 { return ChemCostForTarget(d.People, d.Land) }

// BioCost is what a biological missile aimed at d costs.
func (w *World) BioCost(d *Empire) int64 {
	return BioCostForTarget(d.Troopers, d.People, d.Land)
}

// payArmsDealer settles a warhead's price against gold in hand and then against
// the bank. Drawing on the bank is the original's rule, not a convenience: the
// affordability test it runs before confirming the sale compares the price
// against gold PLUS bank, and the deduction that follows takes whatever gold in
// hand cannot cover straight out of the bank rather than refusing the sale.
func payArmsDealer(a *Empire, cost int64) error {
	if a.Gold+a.Bank < cost {
		return ErrCantAfford
	}
	a.Gold -= cost
	if a.Gold < 0 {
		a.Bank += a.Gold
		a.Gold = 0
	}
	return nil
}

// ruinToWaste turns pct of d's regions into waste and returns how many. The
// removal spreads over every type the target holds, waste included, so land
// already ruined absorbs part of the strike, and the same count goes straight
// back on as waste — the realm's total never moves. Both the nuclear and the
// chemical missile reach this through one shared helper in the original.
func ruinToWaste(d *Empire, pct int) int {
	ruined := d.Regions.remove(d.Land * pct / 100).Total()
	d.Regions.Waste += ruined
	d.syncLand()
	return ruined
}

// roundDiv is a/b rounded to nearest with halves going away from zero, which is
// what the original's Real48 round does (verified against the linked runtime:
// 1.5 rounds to 2, 2.5 to 3). Callers pass non-negative values.
func roundDiv(a, b int) int { return (2*a + b) / (2 * b) }

// NuclearStrike ruins a band of the defender's regions, converting them to
// waste. No land changes hands and nobody dies: the defender keeps every region
// on its books, still pays upkeep on them, and earns nothing from them until
// they are decontaminated (see DecontaminateAllowance).
//
// The strike is not intercepted. Nothing in the original's local nuclear path
// consults the target's SDI or turrets — SDI shoots down interplanetary
// missiles, not a neighbour's.
func (w *World) NuclearStrike(a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.NukeCost(d)); err != nil {
		return "", err
	}

	// 7% ± a two-draw jitter, so the band is 5-9% and the extremes are rarer
	// than the middle.
	pct := NukeWastePct + w.rng.Intn(NukeWasteJitter) - w.rng.Intn(NukeWasteJitter)
	regions := ruinToWaste(d, pct)

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

// ChemicalStrike ruins a narrower band of the defender's regions than a nuke
// does, and gasses a flat fifth of its people on top. Morale and popular
// support both fall as well, which is the part that outlasts the casualties:
// a gassed realm produces and fights worse for as long as it takes to win the
// two back.
//
// Like the nuclear strike it is not intercepted — nothing in the original's
// local chemical path reads the target's SDI, turrets or tanks — and it cannot
// eliminate a realm: the land stays on the target's books as waste, and a
// percentage of a population never reaches zero.
func (w *World) ChemicalStrike(a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.ChemCost(d)); err != nil {
		return "", err
	}

	// 3% ± a two-draw jitter, so 1-5% — a third of the nuclear band.
	pct := ChemWastePct + w.rng.Intn(ChemWasteJitter) - w.rng.Intn(ChemWasteJitter)
	regions := ruinToWaste(d, pct)

	// The population kill has no roll behind it at all: it is a flat share.
	// int64 through the multiply — a big urban realm's head count times a
	// percentage passes 2^31 on the 32-bit door builds.
	people := int(int64(d.People) * ChemPopKillPct / 100)
	d.People -= people
	d.Morale = roundDiv(d.Morale*ChemMoraleKeepNum, ChemMoraleKeepDen)
	d.Support = roundDiv(d.Support*StrikeSupportKeepNum, StrikeSupportKeepDen)

	addScore(a, w.rng.Intn(ChemScoreAward))

	d.addEvent(fmt.Sprintf("%s hit you with a chemical strike: %d regions reduced to waste and %d dead. Famine follows.", a.Name, regions, people))

	w.postStrikeNews(a, d, "chemical")
	return fmt.Sprintf("Chemical strike! %d regions of %s are now waste.\nThe gas killed %d of its people, and its morale and support are broken.", regions, d.Name, people), nil
}

// BiologicalStrike kills people and troopers and halves military morale. It
// leaves the land alone entirely — the original's routine never reaches the
// region-to-waste helper the other two share — and, like them, it cannot
// eliminate a realm.
func (w *World) BiologicalStrike(a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.BioCost(d)); err != nil {
		return "", err
	}
	// The Score lands on the sale, before any damage is rolled, so a plague
	// that kills nobody still pays.
	addScore(a, w.rng.Intn(BioScoreAward))

	popPct := BioPopKillPct + w.rng.Intn(BioPopKillJitterUp) - w.rng.Intn(BioPopKillJitterDown)
	people := int(int64(d.People) * int64(popPct) / 100) // int64: see ChemicalStrike
	d.People -= people

	troopPct := BioTroopKillPct + w.rng.Intn(BioTroopKillJitterUp) - w.rng.Intn(BioTroopKillJitterDn)
	troops := d.Troopers * troopPct / 100
	d.Troopers -= troops

	// Morale is halved by an integer divide here, not the rounded real the
	// chemical strike uses — the plague hits the barracks hardest.
	d.Morale /= BioMoraleDivisor
	d.Support = roundDiv(d.Support*StrikeSupportKeepNum, StrikeSupportKeepDen)

	d.addEvent(fmt.Sprintf("%s hit you with a biological strike: %d troopers and %d civilians dead. Famine follows.", a.Name, troops, people))

	w.postStrikeNews(a, d, "biological")
	return fmt.Sprintf("Biological strike! The plague killed %d troopers of %s.\nIt also carried off %d civilians, and half the realm's morale with them.", troops, d.Name, people), nil
}
