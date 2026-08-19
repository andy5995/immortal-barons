package game

// Per-turn maintenance obligations. In BRE these are prompted at the start
// of each turn ("Your Armed Forces Require N / How much will you give?",
// "N gold is required to maintain your regions", "N gold is requested to
// boost popular support"). The prompt labels and non-payment consequences
// (desertion, revolt) come from the original's strings; the exact numeric
// rates live in compiled code, so the constants below are reconstructed and
// tunable.
const (
	ArmyDesertRate   = 25 // % of the army that deserts at full non-payment
	RegionRevoltRate = 15 // % of land that revolts at full non-payment

	// Combat effectiveness is MoraleCombatFloor + morale x MoraleCombatSlope/100
	// percent. BINARY-VERIFIED (BRE.OVR 0xF37B): morale x 0.6 + 50, so a fully
	// motivated army fights at 110%, not merely at par — IB previously derived
	// the slope from the floor (100-floor = 50) and capped effectiveness at 100.
	MoraleCombatFloor = 50 // binary: effectiveness % at zero morale
	MoraleCombatSlope = 60 // binary
)

// ForcesUpkeep is the gold the armed forces require this turn. Technology
// regions lower it (same factor income uses); this is the formula the old
// auto-deducted maintenance used.
// Per-unit maintenance follows BRE's Medium table (Trooper 0.60, Jet 1.20,
// Turret 0.90, Bomber 1.30, Tank 0.60, Carrier 0.10 gold/turn), held in tenths
// and truncated once on the total — BRE charged exactly trunc(0.9 × turrets)
// for a turret-only army.
func (e *Empire) ForcesUpkeep() int64 {
	tenths := e.Troopers*MaintTrooperTenths + e.Jets*MaintJetTenths + e.Turrets*MaintTurretTenths + e.Bombers*MaintBomberTenths + e.Tanks*MaintTankTenths + e.Carriers*MaintCarrierTenths
	return techLower(int64(tenths/MaintTenthsPerGold), e.TechMaintFactor())
}

// RegionUpkeep is the gold required to maintain the empire's regions. Technology
// lowers it (same factor military upkeep uses); BRE lists "maintenance costs on
// regions" among technology's effects (#20).
func (e *Empire) RegionUpkeep() int64 {
	return techLower(int64(e.Land)*RegionUpkeepPerLand, e.TechMaintFactor())
}

// FoodProduced is the empire's food-region output this turn. The Technology
// bonus is inside agriFood, on the base, where BRE applies it.
func (w *World) FoodProduced(e *Empire) int {
	return e.Regions.foodProduced(w.agriFood(e))
}

// agriFood is this turn's food yield per Agricultural region: BRE's
// Base + Random(Rate), one roll per turn shared by every Agricultural region
// (so the printed total always divides exactly by the region count). Technology
// raises the base and not the draw — the yield routine returns a tech-scaled
// 300 and its caller adds the draw afterward.
func (w *World) agriFood(e *Empire) int {
	return techRaise(FoodAgriBase, e.TechFoodFactor()) + w.regionDraw(e, 6, FoodAgriRate)
}

// ForcesDue and RegionsDue apply the league's Maintenance Costs knob to the
// base upkeep, on the original's own ladder — a quarter at Low, four times at
// High (Level.MaintCostScaled). These are the amounts actually charged and
// displayed; the Empire methods above give the unscaled baseline.
func (w *World) ForcesDue(e *Empire) int64 {
	return w.Config.MaintCosts.MaintCostScaled(e.ForcesUpkeep() + w.listedForcesUpkeep(e))
}

// listedForcesUpkeep is the maintenance owed on this empire's military units that
// are escrowed on the Trading Market (#17). Listing a unit removes it from
// inventory, but it still costs upkeep — so parking an army on the market to dodge
// maintenance doesn't work. Same per-unit rates and Technology scaling as
// ForcesUpkeep; food and agents have no upkeep.
func (w *World) listedForcesUpkeep(e *Empire) int64 {
	l := w.listedUnits(e)
	tenths := l.Troopers*MaintTrooperTenths + l.Jets*MaintJetTenths +
		l.Turrets*MaintTurretTenths + l.Bombers*MaintBomberTenths +
		l.Tanks*MaintTankTenths + l.Carriers*MaintCarrierTenths
	return techLower(int64(tenths/MaintTenthsPerGold), e.TechMaintFactor())
}

// listedUnits is the military this empire has escrowed on the Trading Market.
// Both the gold bill and the food bill charge escrowed units, so they read them
// through this rather than each spelling out the six market names.
//
// It hands the counts back in an otherwise empty Empire, so the canonical
// table's own accessors can fill it (#134) and a caller reads e.Troopers on
// both sides of the sum. Nothing else on the value is set or meaningful.
func (w *World) listedUnits(e *Empire) Empire {
	var listed Empire
	for _, g := range MilitaryGoods {
		*g.Count(&listed) = w.MarketForSale(e.Name, g.Singular)
	}
	return listed
}
func (w *World) RegionsDue(e *Empire) int64 {
	return w.Config.MaintCosts.MaintCostScaled(e.RegionUpkeep())
}

// PeopleFoodUpkeep is the food the population eats per turn — the first of BRE's
// two food obligations ("Your People Need N units of food"). BRE charges 1.5 per
// unit of population and counts population in millions, so the conversion to
// IB's counter runs through PopBREUnitScale.
func (e *Empire) PeopleFoodUpkeep() int {
	return peopleFood(e.People)
}

func peopleFood(people int) int {
	return int(int64(people) * FoodPerBREPopUnitTenths / (10 * PopBREUnitScale))
}

// forcesFoodWeighted is the armed forces' food bill in ten-thousandths, before
// the single truncation BRE applies to the whole sum. Every military unit type
// is charged; the counts are weighted, not divided one type at a time, so the
// fractions add up before they are dropped.
func forcesFoodWeighted(troopers, jets, turrets, bombers, tanks, carriers int) int64 {
	return int64(troopers)*ForcesFoodTrooperWeight +
		int64(jets+turrets+bombers+tanks+carriers)*ForcesFoodUnitWeight
}

// ForcesFoodUpkeep is the food the armed forces eat per turn — BRE's second,
// separately truncated obligation ("Your Armed Forces Require N units of food").
// This is the baseline for the units the empire holds; World.ForcesFoodDue adds
// the ones escrowed on the Trading Market, which BRE charges too.
func (e *Empire) ForcesFoodUpkeep() int {
	return int(forcesFoodWeighted(e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Tanks, e.Carriers) / ForcesFoodWeightScale)
}

// ForcesFoodDue is the armed forces' food including the units this empire has
// listed on the Trading Market. BRE reads each type from both its home field and
// its market escrow into the same sum, so parking an army on the market no more
// dodges its rations than it dodges its maintenance (see listedForcesUpkeep).
func (w *World) ForcesFoodDue(e *Empire) int {
	held := forcesFoodWeighted(e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Tanks, e.Carriers)
	l := w.listedUnits(e)
	listed := forcesFoodWeighted(l.Troopers, l.Jets, l.Turrets, l.Bombers, l.Tanks, l.Carriers)
	return int((held + listed) / ForcesFoodWeightScale)
}

// FoodUpkeep is this turn's whole food bill for the units the empire holds. The
// two obligations are truncated separately and then added — that is what BRE
// does, and a single accumulator reads one food high whenever both terms have a
// fractional part.
func (e *Empire) FoodUpkeep() int {
	return e.PeopleFoodUpkeep() + e.ForcesFoodUpkeep()
}

// FoodDue is FoodUpkeep with the Trading Market escrow counted, and is the
// figure actually consumed and displayed (same relationship ForcesDue has to
// ForcesUpkeep).
func (w *World) FoodDue(e *Empire) int {
	return e.PeopleFoodUpkeep() + w.ForcesFoodDue(e)
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
	return peopleFood(people) + e.ForcesFoodUpkeep()
}

// clampGive limits a payment to what the empire can actually afford and
// deducts it, recording it in the turn's LastGoldPaid tally. Returns the
// amount actually paid.
func (e *Empire) clampGive(given int64) int64 {
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
// leaves the payer's hands for the Queen's own purse (World.RefundPool), which
// she pays back out a share at a time — see QueenRefund. BINARY-VERIFIED that
// what she banks is the gold ACTUALLY PAID, not the sum demanded: the same
// figure the shortfall penalty is computed from is the one added to the pool
// (BRE.OVR 0x2FAF1).
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
func (w *World) PayCrownTax(e *Empire, given int64) {
	req := w.CrownTax(e)
	given = e.clampGive(given)
	w.RefundPool += given
	if req <= 0 || given >= req {
		return
	}
	// Deferred to turn rollover (see PendingSupportPenalty), matching BRE.
	e.PendingSupportPenalty += shortfallPenalty(req, given, CrownTaxSupportPenalty)
}

// QueenRefund pays e its share of the Queen's purse and returns the amount, or
// 0 when there is nothing to give. The caller decides WHEN this happens: BRE
// calls it once per realm per game day, from the "since your last play" recap of
// the day's first session, and the routine itself holds no random draw — so a
// realm that logs in is paid.
//
// The share is QueenRefundRate percent of the pool, rising to QueenRefundHighRate
// once the pool is over QueenRefundHighPool, and is capped at QueenRefundCap
// while the realm is still under New Realm Protection. See the constants for the
// binary provenance.
func (w *World) QueenRefund(e *Empire) int64 {
	if w.RefundPool <= 0 {
		return 0
	}
	rate := QueenRefundRate
	if w.RefundPool > QueenRefundHighPool {
		rate = QueenRefundHighRate
	}
	pay := pctOf(w.RefundPool, rate)
	if e.Protection > 0 && pay > QueenRefundCap {
		pay = QueenRefundCap
	}
	if pay <= 0 {
		return 0
	}
	w.RefundPool -= pay
	w.creditGold(e, pay, "the Queen's refund")
	return pay
}

// PayForces applies a payment toward armed-forces upkeep. A shortfall makes
// units desert proportionally and costs military morale — only morale: the
// original's armed-forces branch touches that stat and no other. Returns the
// number of units lost to desertion.
func (w *World) PayForces(e *Empire, given int64) int {
	req := w.ForcesDue(e)
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	fracPct := int((req - given) * 100 / req) // req > 0 here (else given >= req)
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
	// Deferred to turn rollover, as BRE defers every payment-stage penalty: the
	// whole stage accumulates into two signed bytes on the empire record (+0x2b9
	// morale, +0x2ba support) which the end-of-turn routines then apply.
	e.PendingMoralePenalty += shortfallPenalty(req, given, ForcesShortfallMoraleScale)
	return lost
}

// PayRegions applies a payment toward region maintenance. A shortfall makes
// regions revolt (land is lost), costs popular support, and — below
// RegionCivilWarThresholdPct of what was due — files a civil war. That last
// trigger is far easier to hit than famine's, so letting land upkeep slide is
// the fastest way to tear a realm apart.
//
// IB follows the CORRECT operation order here, as it does for the crown tax:
// BRE's own region branch computes `1 − ratio×50` rather than `(1 − ratio)×50`,
// which goes negative for any shortfall under 98% and would RAISE support.
// Returns the number of regions lost.
func (w *World) PayRegions(e *Empire, given int64) int {
	req := w.RegionsDue(e)
	given = e.clampGive(given)
	if given >= req {
		return 0
	}
	fracPct := int((req - given) * 100 / req) // req > 0 here (else given >= req)
	lost := e.Land * fracPct / 100 * RegionRevoltRate / 100
	if lost > 0 {
		e.Regions.remove(lost)
		e.syncLand()
	}
	e.PendingSupportPenalty += shortfallPenalty(req, given, RegionShortfallSupportScale)
	e.CivilWarSeverity += civilWarSeverity(req, given, RegionCivilWarThresholdPct)
	return lost
}

// supportBoostDeficit is the number of support points this turn's boost can buy
// back — the shortfall from 100, capped at MaxSupportBoostPerTurn.
func (e *Empire) supportBoostDeficit() int {
	return min(100-e.Support, MaxSupportBoostPerTurn)
}

// SupportBoostCost is the gold the crown requests to restore this turn's
// recoverable support. Zero when support is already full.
func (e *Empire) SupportBoostCost() int64 {
	// SupportBoostPerPerson was fitted to BRE's population figure, which counts
	// millions — so the population goes back into BRE's unit before it is
	// applied, or the crown charges twenty times what it should. See
	// PopBREUnitScale.
	people := e.People / PopBREUnitScale
	return int64(e.supportBoostDeficit()) * int64(SupportBoostPerPerson*people+SupportBoostFlat)
}

// SupportBoostMax is the most a baron may put toward the boost. Paying past the
// request does buy more support, but support caps at 100, so the surplus is
// usually wasted.
func (e *Empire) SupportBoostMax() int64 {
	return e.SupportBoostCost() * SupportBoostMaxPct / 100
}

// BoostSupport spends gold to raise popular support (the optional "requested"
// obligation). Paying the full request buys the whole deficit; paying part of it
// buys proportionally less. One turn's boost is capped, so a badly unpopular
// realm takes several turns to recover. Returns the support points gained.
//
// Binary-verified, including the +1 on each side of the ratio (BRE.OVR 0x2F740),
// which is the same shape the crown-tax penalty uses.
func (w *World) BoostSupport(e *Empire, given int64) int {
	cost := e.SupportBoostCost()
	given = e.clampGive(given)
	if cost <= 0 {
		return 0
	}
	pts := int(int64(e.supportBoostDeficit()) * (given + 1) / (cost + 1))
	before := e.Support
	e.adjustSupport(pts)
	return e.Support - before
}

// moraleBoostDeficit is the number of morale points this turn's boost is priced
// for — the shortfall from 100, capped at MaxMoraleBoostPerTurn.
func (e *Empire) moraleBoostDeficit() int {
	return min(100-e.Morale, MaxMoraleBoostPerTurn)
}

// moraleBoostUnits is the per-point price of morale in hundredths of a gold:
// the army's weighted size, counting units escrowed on the Trading Market.
// Bombers and carriers carry no weight in the original.
func (w *World) moraleBoostUnits(e *Empire) int64 {
	held := func(good string, n int) int64 {
		return int64(n + w.MarketForSale(e.Name, good))
	}
	return held("Trooper", e.Troopers)*MoraleBoostTrooper +
		held("Jet", e.Jets)*MoraleBoostJet +
		held("Turret", e.Turrets)*MoraleBoostTurret +
		held("Tank", e.Tanks)*MoraleBoostTank
}

// MoraleBoostCost is the gold the crown requests to restore this turn's morale.
// Zero when morale is already full.
func (w *World) MoraleBoostCost(e *Empire) int64 {
	d := e.moraleBoostDeficit()
	if d <= 0 {
		return 0
	}
	return int64(d)*w.moraleBoostUnits(e)/MoraleBoostWeightUnit + MoraleBoostFlat
}

// MoraleBoostMax is the most a baron may put toward the morale boost. As with
// support, overpaying really does buy more than the charged deficit.
func (w *World) MoraleBoostMax(e *Empire) int64 {
	return w.MoraleBoostCost(e) * MoraleBoostMaxPct / 100
}

// BoostMorale spends gold to raise military morale, on the same ratio
// BoostSupport uses. Returns the morale points gained.
func (w *World) BoostMorale(e *Empire, given int64) int {
	cost := w.MoraleBoostCost(e)
	given = e.clampGive(given)
	if cost <= 0 {
		return 0
	}
	pts := int(int64(e.moraleBoostDeficit()) * (given + 1) / (cost + 1))
	before := e.Morale
	e.adjustMorale(pts)
	return e.Morale - before
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
