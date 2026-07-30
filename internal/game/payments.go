package game

// Per-turn maintenance obligations. In BRE these are prompted at the start
// of each turn ("Your Armed Forces Require N / How much will you give?",
// "N gold is required to maintain your regions", "N gold is requested to
// boost popular support"). The prompt labels and non-payment consequences
// (desertion, revolt) come from the original's strings; the exact numeric
// rates live in compiled code, so the constants below are reconstructed and
// tunable.
const (
	ArmyDesertRate     = 25  // % of the army that deserts at full non-payment
	RegionRevoltRate   = 15  // % of land that revolts at full non-payment
	MoralePerBoostGold = 100 // gold to raise military morale by one point

	// A single turn's payment cannot fully restore morale — paying the whole
	// requested amount only buys a bounded number of points, so recovery takes
	// several turns. Unverified placeholder, unlike its support counterpart
	// (MaxSupportBoostPerTurn and the rest of the support-boost model live in
	// balance.go, where they are binary-verified).
	MaxMoraleBoostPerTurn = 20

	// Military morale effects (placeholders, tunable). Below the floor, combat
	// effectiveness is scaled by moraleFactor; below the desert threshold, a
	// slice of the army deserts each turn morale stays that low.
	MoraleCombatFloor     = 50 // combat effectiveness % at zero morale
	MoraleDesertThreshold = 30 // morale at/below which troops start deserting
	MoraleDesertRate      = 5  // % of the army lost per turn while morale is that low
	MoraleDrift           = 4  // points morale recovers toward 100 per turn
)

// ForcesUpkeep is the gold the armed forces require this turn. Technology
// regions lower it (same factor income uses); this is the formula the old
// auto-deducted maintenance used.
// Per-unit maintenance follows BRE's Medium table (Trooper 0.60, Jet 1.20,
// Turret 0.90, Bomber 1.30, Tank 0.60, Carrier 0.10 gold/turn), held in tenths
// and truncated once on the total — BRE charged exactly trunc(0.9 × turrets)
// for a turret-only army.
func (e *Empire) ForcesUpkeep() int {
	tenths := e.Troopers*MaintTrooperTenths + e.Jets*MaintJetTenths + e.Turrets*MaintTurretTenths + e.Bombers*MaintBomberTenths + e.Tanks*MaintTankTenths + e.Carriers*MaintCarrierTenths
	return techLower(tenths/MaintTenthsPerGold, e.TechMaintFactor())
}

// RegionUpkeep is the gold required to maintain the empire's regions. Technology
// lowers it (same factor military upkeep uses); BRE lists "maintenance costs on
// regions" among technology's effects (#20).
func (e *Empire) RegionUpkeep() int {
	return techLower(e.Land*RegionUpkeepPerLand, e.TechMaintFactor())
}

// FoodProduced is the empire's food-region output this turn, raised by the
// Technology bonus — BRE counts food regions under "increased region output",
// which technology improves (#20). River fishing (riverFood) is a separate
// mechanic and is not scaled here.
func (w *World) FoodProduced(e *Empire) int {
	return techRaise(e.Regions.foodProduced(w.agriFood(e)), e.TechFoodFactor())
}

// agriFood is this turn's food yield per Agricultural region: BRE's
// Base + Random(Rate), one roll per turn shared by every Agricultural region
// (so the printed total always divides exactly by the region count).
func (w *World) agriFood(e *Empire) int {
	return FoodAgriBase + w.regionDraw(e, 6, FoodAgriRate)
}

// ForcesDue and RegionsDue apply the league's Maintenance Costs knob to the
// base upkeep (Medium = 100% = unchanged; None = 0 = free upkeep). These are
// the amounts actually charged and displayed; the Empire methods above give
// the unscaled baseline.
func (w *World) ForcesDue(e *Empire) int {
	return (e.ForcesUpkeep() + w.listedForcesUpkeep(e)) * w.Config.MaintCosts.Percent() / 100
}

// listedForcesUpkeep is the maintenance owed on this empire's military units that
// are escrowed on the Trading Market (#17). Listing a unit removes it from
// inventory, but it still costs upkeep — so parking an army on the market to dodge
// maintenance doesn't work. Same per-unit rates and Technology scaling as
// ForcesUpkeep; food and agents have no upkeep.
func (w *World) listedForcesUpkeep(e *Empire) int {
	tenths := w.MarketForSale(e.Owner, "Trooper")*MaintTrooperTenths +
		w.MarketForSale(e.Owner, "Jet")*MaintJetTenths +
		w.MarketForSale(e.Owner, "Turret")*MaintTurretTenths +
		w.MarketForSale(e.Owner, "Bomber")*MaintBomberTenths +
		w.MarketForSale(e.Owner, "Tank")*MaintTankTenths +
		w.MarketForSale(e.Owner, "Carrier")*MaintCarrierTenths
	return techLower(tenths/MaintTenthsPerGold, e.TechMaintFactor())
}
func (w *World) RegionsDue(e *Empire) int {
	return e.RegionUpkeep() * w.Config.MaintCosts.Percent() / 100
}

// FoodUpkeep is the food the population and army eat per turn. The population's
// share is scaled (BRE bills "People Need ~150 food", not one-per-person); the
// army eats ~1 food per ArmyFoodDivisor units, with jets and tanks weighing
// double (crews plus fuel/rations).
func (e *Empire) FoodUpkeep() int {
	return e.People*PeopleFoodPerThousand/1000 + (e.Troopers+e.Jets*2+e.Tanks*2)/ArmyFoodDivisor
}

// FoodUpkeepAtCapacity projects FoodUpkeep to the empire's population carrying
// capacity — the food it will consume once a still-growing populace fills out
// (military upkeep unchanged). Population capacity is support-driven and
// decoupled from food (as in BRE), so a high-support realm can sit at a food
// surplus now yet outgrow its production later; the advisor uses this to warn
// before that happens. Never projects below the current population.
func (e *Empire) FoodUpkeepAtCapacity() int {
	people := e.People
	if cap := e.popCapacity(); cap > people {
		people = cap
	}
	return people*PeopleFoodPerThousand/1000 + (e.Troopers+e.Jets*2+e.Tanks*2)/ArmyFoodDivisor
}

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

// PayCrownTax applies a payment toward the Queen Royale's per-turn tax. The gold
// is a pure sink — it leaves the economy with no recipient.
//
// A shortfall costs popular support, up to CrownTaxSupportPenalty points.
// Binary-verified, including the operation order: BRE computes
// (1 - ratio) * K, not 1 - ratio*K. (Its own region-maintenance branch has the
// operands the other way round, which goes negative for any shortfall under 98%
// and would *raise* support — a parenthesisation bug in the original. IB follows
// the tax branch, which is the correct one.)
//
// In BRE a solvent baron cannot underpay at all: the prompt's minimum is the
// amount required. IB lets a player underpay any obligation, as it already does
// for forces and regions, and takes the penalty instead.
func (w *World) PayCrownTax(e *Empire, given int) {
	req := w.CrownTax(e)
	given = e.clampGive(given)
	if req <= 0 || given >= req {
		return
	}
	// Deferred to turn rollover (see PendingSupportPenalty), matching BRE.
	e.PendingSupportPenalty += (req - given) * CrownTaxSupportPenalty / (req + 1)
}

// PayForces applies a payment toward armed-forces upkeep. A shortfall makes
// units desert proportionally and lowers popular support. Returns the number
// of units lost to desertion.
func (w *World) PayForces(e *Empire, given int) int {
	req := w.ForcesDue(e)
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	fracPct := (req - given) * 100 / req // req > 0 here (else given >= req)
	desertPct := fracPct * ArmyDesertRate / 100
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
	e.adjustMorale(-fracPct / 4) // unpaid troops lose heart faster than the public
	return lost
}

// PayRegions applies a payment toward region maintenance. A shortfall makes
// regions revolt (land is lost) and lowers popular support. Returns the
// number of regions lost.
func (w *World) PayRegions(e *Empire, given int) int {
	req := w.RegionsDue(e)
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

// supportBoostDeficit is the number of support points this turn's boost can buy
// back — the shortfall from 100, capped at MaxSupportBoostPerTurn.
func (e *Empire) supportBoostDeficit() int {
	return min(100-e.Support, MaxSupportBoostPerTurn)
}

// SupportBoostCost is the gold the crown requests to restore this turn's
// recoverable support. Zero when support is already full.
func (e *Empire) SupportBoostCost() int {
	return e.supportBoostDeficit() * (SupportBoostPerPerson*e.People + SupportBoostFlat)
}

// SupportBoostMax is the most a baron may put toward the boost. Paying past the
// request does buy more support, but support caps at 100, so the surplus is
// usually wasted.
func (e *Empire) SupportBoostMax() int {
	return e.SupportBoostCost() * SupportBoostMaxPct / 100
}

// BoostSupport spends gold to raise popular support (the optional "requested"
// obligation). Paying the full request buys the whole deficit; paying part of it
// buys proportionally less. One turn's boost is capped, so a badly unpopular
// realm takes several turns to recover. Returns the support points gained.
//
// Binary-verified, including the +1 on each side of the ratio (BRE.OVR 0x2F740),
// which is the same shape the crown-tax penalty uses.
func (w *World) BoostSupport(e *Empire, given int) int {
	cost := e.SupportBoostCost()
	given = e.clampGive(given)
	if cost <= 0 {
		return 0
	}
	pts := e.supportBoostDeficit() * (given + 1) / (cost + 1)
	before := e.Support
	e.adjustSupport(pts)
	return e.Support - before
}

// BoostMorale spends gold to raise military morale, mirroring BoostSupport
// (capped per turn). Returns the morale points gained.
func (w *World) BoostMorale(e *Empire, given int) int {
	given = e.clampGive(given)
	pts := min(given/MoralePerBoostGold, MaxMoraleBoostPerTurn)
	e.adjustMorale(pts)
	return pts
}

// adjustSupport moves support by delta, clamped to [0, 100].
func (e *Empire) adjustSupport(delta int) {
	e.Support = clampPct(e.Support + delta)
}

// adjustMorale moves morale by delta, clamped to [0, 100].
func (e *Empire) adjustMorale(delta int) {
	e.Morale = clampPct(e.Morale + delta)
}

// clampPct clamps a percentage-style stat to [0, 100].
func clampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
