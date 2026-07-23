package game

// Per-turn maintenance obligations. In BRE these are prompted at the start
// of each turn ("Your Armed Forces Require N / How much will you give?",
// "N gold is required to maintain your regions", "N gold is requested to
// boost popular support"). The prompt labels and non-payment consequences
// (desertion, revolt) come from the original's strings; the exact numeric
// rates live in compiled code, so the constants below are reconstructed and
// tunable.
const (
	ArmyDesertRate      = 25  // % of the army that deserts at full non-payment
	RegionRevoltRate    = 15  // % of land that revolts at full non-payment
	SupportPerBoostGold = 100 // gold to raise popular support by one point
	MoralePerBoostGold  = 100 // gold to raise military morale by one point

	// A single turn's payment cannot fully restore support/morale — paying the
	// whole requested amount only buys a bounded number of points, so recovery
	// takes several turns (per observed BRE behavior). Placeholder — tunable.
	MaxSupportBoostPerTurn = 20
	MaxMoraleBoostPerTurn  = 20

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
// Turret 0.90, Bomber 1.30, Tank 0.60, Carrier 0.10 gold/turn) scaled ×10, so
// the ratios match the original exactly. Bombers were previously omitted.
func (e *Empire) ForcesUpkeep() int {
	return (e.Troopers*MaintTrooper + e.Jets*MaintJet + e.Turrets*MaintTurret + e.Bombers*MaintBomber + e.Tanks*MaintTank + e.Carriers*MaintCarrier) * (100 - e.TechFactor()) / 100
}

// RegionUpkeep is the gold required to maintain the empire's regions. Technology
// lowers it (same factor military upkeep uses); BRE lists "maintenance costs on
// regions" among technology's effects (#20).
func (e *Empire) RegionUpkeep() int {
	return e.Land * RegionUpkeepPerLand * (100 - e.TechFactor()) / 100
}

// FoodProduced is the empire's food-region output this turn, raised by the
// Technology bonus — BRE counts food regions under "increased region output",
// which technology improves (#20). River fishing (riverFood) is a separate
// mechanic and is not scaled here.
func (e *Empire) FoodProduced() int {
	return e.Regions.foodProduced() * (100 + e.TechFactor()) / 100
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
	up := w.MarketForSale(e.Owner, "Trooper")*MaintTrooper +
		w.MarketForSale(e.Owner, "Jet")*MaintJet +
		w.MarketForSale(e.Owner, "Turret")*MaintTurret +
		w.MarketForSale(e.Owner, "Bomber")*MaintBomber +
		w.MarketForSale(e.Owner, "Tank")*MaintTank +
		w.MarketForSale(e.Owner, "Carrier")*MaintCarrier
	return up * (100 - e.TechFactor()) / 100
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

// PayForces applies a payment toward armed-forces upkeep. A shortfall makes
// units desert proportionally and lowers popular support. Returns the number
// of units lost to desertion.
func (w *World) PayForces(e *Empire, given int) int {
	req := w.ForcesDue(e)
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	e.MaintUnderpaid = true              // underpaid this obligation; blocks the well-run support boost
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
	e.MaintUnderpaid = true              // underpaid this obligation; blocks the well-run support boost
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
// "requested" obligation). One turn's boost is capped, so it takes several
// turns of payment to fully recover. Returns the support points gained.
func (w *World) BoostSupport(e *Empire, given int) int {
	given = e.clampGive(given)
	pts := min(given/SupportPerBoostGold, MaxSupportBoostPerTurn)
	e.adjustSupport(pts)
	return pts
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
