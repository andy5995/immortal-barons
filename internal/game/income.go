package game

import (
	"math"
	"math/big"
)

// income.go — what a realm earns and grows in a turn: the per-region draw, the
// industrial gold, the rivers, and what the population and industry produce.

// IncomeBreakdown itemizes a turn's income by source (gold), plus the food
// grown. The income report and the actual gold credit both derive from this,
// so what the player is shown equals what is credited to the last coin.
//
// Region income is kept per region type, indexed by mixOrder (GoldFrom,
// FoodFrom): which types pay what is a matter of the rules (RegionRules), not
// of fields here. In the base rules five types pay gold (Mountain ore, Coastal
// tourism, Desert solar, River hydropower, Industrial's idle share) and two
// produce food (Agricultural, River); Urban and Technology produce neither
// (BRE-verified).
type IncomeBreakdown struct {
	Taxes, Trade int
	RegionGold   [regionKinds]int
	RegionFood   [regionKinds]int
}

// GoldFrom is the gold r's regions earned.
func (b IncomeBreakdown) GoldFrom(r *Region) int { return b.RegionGold[r.index] }

// FoodFrom is the food r's regions produced.
func (b IncomeBreakdown) FoodFrom(r *Region) int { return b.RegionFood[r.index] }

// regionGold sums the region gold.
func (b IncomeBreakdown) regionGold() int {
	sum := 0
	for _, n := range b.RegionGold {
		sum += n
	}
	return sum
}

// Food is the food every region type produced.
func (b IncomeBreakdown) Food() int {
	sum := 0
	for _, n := range b.RegionFood {
		sum += n
	}
	return sum
}

// Gold sums the gold-producing sources.
func (b IncomeBreakdown) Gold() int {
	return b.Taxes + b.regionGold() + b.Trade
}

// CrownTaxBase is the income the crown tax is levied on: the six region/tax
// income lines, excluding Trade. BRE accumulates its base at exactly six sites,
// one per income line, and trading proceeds are not among them.
func (b IncomeBreakdown) CrownTaxBase() int {
	return b.Taxes + b.regionGold()
}

// CrownTax is the Queen Royale's per-turn tax: a flat share of the turn's gross
// gold income, and a pure sink — no recipient treasury. Binary-verified against
// BRE (cost function at BRE.OVR 0x2EA09), where it reproduces 28 of 28 captured
// charges exactly:
//
//	crownTax = trunc(goldEarnedThisTurn * PlanetaryTaxRate / 100)
//
// The base is income alone — not gold on hand, bank balance, net worth, score,
// or units. BRE stores its rate in tenths of a percent (its default is 50,
// shown as 5.0%); IB stores whole percent instead, so a sysop and the config
// editor deal in one unit rather than two. Same arithmetic, one fewer trap.
func (w *World) CrownTax(e *Empire) int64 {
	return pctOf(int64(w.IncomeThisTurn(e).CrownTaxBase()), w.Config.PlanetaryTaxRate)
}

// regionDraw returns this turn's income draw for empire e's region type
// identified by salt: a uniform integer in [0, n). It is deterministic in
// (w.GameDay, e.Name, salt) — NOT a fresh RNG draw — so IncomeThisTurn stays
// pure and the income report always equals what CollectIncome credits. The
// variance is per game-day: a good/bad "year" lasts the whole day, and each
// region type draws independently.
//
// BRE draws the same way, straight over the Rate: its income sites call
// Random(Rate) and add the Base, so per-region income is Base + [0, Rate). IB
// used to draw a 0–100 percent and scale it, which could only land on multiples
// of Rate/100 — the same range and mean, but coarser. Live play shows the finer
// steps (one ore turn came out at Base + 279, which is not a multiple of 4).
func (w *World) regionDraw(e *Empire, salt, n int) int {
	return newDraw().num(w.GameDay).num(salt).text(e.Name).roll(n)
}

// industrialGold is the empire's TOTAL idle gold this turn (not per region):
// what its Industrial regions pay, under the base rules. The UNALLOCATED share
// of production — what is left after the six unit percentages — is paid out as
// gold, so allocating to units trades directly against it and a realm at 100%
// allocation earns none.
//
// Binary-verified (BRE.OVR 0x34545–0x345D1), including the order of operations:
//
//	perRegion = [0, IndustryGoldRate) + IndustryGoldBase
//	total     = perRegion * count / 100 * unallocated%
//
// Dividing by 100 before applying the percentage is BRE's own order, and it is
// why industry is the one region type whose total does not divide evenly by its
// region count — the division discards the remainder.
func (w *World) industrialGold(e *Empire) int64 {
	return w.ProjectedIndustrialGold(e, e.idleShare())
}

// idleShare is the percent of production not allocated to units.
func (e *Empire) idleShare() int {
	allocated := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers
	// Whatever is not building units pays gold, whether the player set it aside
	// on the Gold row or simply left it unallocated.
	return max(0, 100-allocated)
}

// ProjectedIndustrialGold is the idle gold that pct% of this empire's
// production pays out this turn. Shared with the Set Industries screen so the
// figure a player is shown is the one industrialGold will credit — regionDraw
// varies by empire and game day, not per call, so the two always agree.
func (w *World) ProjectedIndustrialGold(e *Empire, pct int) int64 {
	var sum int64
	for _, g := range RegionTypes {
		sum += w.idleGold(e, g, pct)
	}
	return sum
}

// idleGold is what g's regions pay on pct% of idle production.
func (w *World) idleGold(e *Empire, g *Region, pct int) int64 {
	rr := w.regionsOf(e)[g]
	n := *g.Count(&e.Regions)
	if pct <= 0 || n <= 0 || rr.IdleGoldBase+rr.IdleGoldRate == 0 {
		return 0
	}
	perRegion := w.regionDraw(e, g.idleSalt, rr.IdleGoldRate) + rr.IdleGoldBase
	// int64 for the reason every other money line is: `int` is 32 bits on the
	// 32-bit door builds, and this product is a per-region rate in the thousands
	// times a region count, which passes int32 at about 840,000 regions — the
	// same shape of overflow the unit pool had (see unitsMade).
	return int64(perRegion) * int64(n) / 100 * int64(pct)
}

// regionGold is the gold g's regions earn this turn, before technology:
//
//	yield = (GoldBase + [0, GoldRate)) x GoldShare / 100
//	gold  = yield x support factor / 100 x count
//
// The support factor is (100 - SupportShare) + SupportShare x support/100, so
// a type with no SupportShare is unscaled. Under the base rules that is BRE's
// perRegion x count for ore and solar; Coastal tourism scaled by 0.10 +
// 0.90·support; and a River's hydropower, which has the highest base and no
// bad year — BRE's own river either runs hydropower or fishes, with nothing
// else to roll (BRE.OVR, the production routine's river block) — less the
// share taken as food (see RiverFishShare).
func (w *World) regionGold(e *Empire, g *Region) int64 {
	rr := w.regionsOf(e)[g]
	n := *g.Count(&e.Regions)
	if n <= 0 || rr.GoldBase+rr.GoldRate == 0 {
		return 0
	}
	yield := (w.regionDraw(e, g.goldSalt, rr.GoldRate) + rr.GoldBase) * rr.GoldShare / 100
	support := 100 - rr.SupportShare + rr.SupportShare*e.Support/100
	return int64(yield) * int64(support) / 100 * int64(n)
}

// IncomeThisTurn itemizes e's income for the current turn (see regionGold and
// regionFood); the population tax is scaled by support in full, with no floor.
// The gold technology factor scales each REGION gold source, as it does in
// the original; idle gold and the trade bonus are untouched by it. Products
// are widened to int64 so they stay correct on 32-bit builds even at
// money-cap scale.
func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown {
	// Region income draws on a different research slot from the population tax
	// (see taxIncome), so the two scale independently.
	gold := int64(e.TechGoldFactor())
	b := IncomeBreakdown{
		Taxes: e.taxIncome(),
		Trade: w.tradeIncome(e), // trade-treaty bonus (population-scaled)
	}
	idle := e.idleShare()
	for _, g := range RegionTypes {
		// Idle gold is NOT technology-scaled: every other gold block in BRE's
		// production routine opens with technology_factor(1.5, slot 0) and the
		// industrial one does not.
		b.RegionGold[g.index] = int(w.regionGold(e, g)*gold/TechFactorUnit + w.idleGold(e, g, idle))
		b.RegionFood[g.index] = w.regionFood(e, g)
	}
	return b
}

// taxIncome is this turn's population tax (see TaxGoldNumerator):
//
//	trunc(people/PopBREUnitScale x 311 x 0.01 / 90 x tax x support x techTaxFactor)
//
// BRE truncates once, at the end, so this does too: the whole product is one
// exact fraction, split into quotient and remainder only so it cannot overflow
// int64 once the technology factor multiplies in. IB's People go in whole rather
// than being cut to BRE's unit first, which is what keeps IB's finer population
// count from being rounded away.
func (e *Empire) taxIncome() int {
	if e.People <= 0 || e.Tax <= 0 || e.Support <= 0 {
		return 0
	}
	const den = TaxGoldPercent * TaxGoldDivisor * PopBREUnitScale
	n := int64(e.People) * TaxGoldNumerator * int64(e.Tax) * int64(e.Support)
	tech := int64(e.TechTaxFactor())
	return int((n/den*tech + n%den*tech/den) / TechFactorUnit)
}

// regionFood is the food g's regions produce this turn: a tech-raised base
// plus one draw per turn, shared by every region of the type (so a total
// always divides exactly by the region count), taken at FoodShare percent.
//
// BINARY-VERIFIED for Agricultural regions: Technology raises the base and not
// the draw — the yield routine returns a tech-scaled 300 and its caller adds
// the draw afterward. A River's haul has the same shape. Unlike BRE, where a
// river runs hydropower OR fishes and the empire finds out which only when the
// income report prints, IB's rivers always do both — RiverFishShare of the
// yield as food, the rest as gold (see regionGold).
func (w *World) regionFood(e *Empire, g *Region) int {
	rr := w.regionsOf(e)[g]
	n := *g.Count(&e.Regions)
	if n <= 0 || rr.FoodBase+rr.FoodRate == 0 {
		return 0
	}
	perRegion := techRaise(rr.FoodBase, e.TechFoodFactor()) + w.regionDraw(e, g.foodSalt, rr.FoodRate)
	return pctOf(n*perRegion, rr.FoodShare)
}

// AvgFood is g's expected food per turn: regionFood with the draw replaced by
// its mean, so the figure holds still from turn to turn.
func (w *World) AvgFood(e *Empire, g *Region) int {
	rr := w.regionsOf(e)[g]
	n := *g.Count(&e.Regions)
	if n <= 0 || rr.FoodBase+rr.FoodRate == 0 {
		return 0
	}
	base := techRaise(rr.FoodBase, e.TechFoodFactor())
	return pctOf(n*(2*base+max(rr.FoodRate, 1)-1), rr.FoodShare) / 2
}

// FoodGrown is the empire's total food production this turn, every region
// type's together. The single source of truth for produced food, so the turn
// engine, the AI, and the advisors agree.
func (w *World) FoodGrown(e *Empire) int {
	sum := 0
	for _, g := range RegionTypes {
		sum += w.regionFood(e, g)
	}
	return sum
}

// GrowFood credits this turn's food yield (region output + river fishing) at the
// START of the turn, alongside Manufacture (military) and CollectIncome (gold) —
// the three things the start-of-turn income report announces. BRE grows food at
// turn start, so the player can sell or spend this turn's growth the same turn;
// deferring it to processEconomy (the old behavior) meant the growth arrived
// after the food market and always spoiled 5% (verified against BRE, 2026-07-20).
// Consumption and spoilage stay in processEconomy (end of turn).
func (w *World) GrowFood(e *Empire) {
	e.Food += w.FoodGrown(e)
}

// CollectIncome credits this turn's gold income (see IncomeThisTurn) at the
// start of the turn, so it is in hand for maintenance and spending. BRE shows
// the income report then, and its auto-deposit banks only the "extra" gold
// left at the end of the turn. Keeping this out of processEconomy (the
// end-of-turn steps: interest, food, population) is what lets start-of-turn
// maintenance be paid from the income the turn earns, instead of a turn
// behind. manufacture is likewise called at turn start, alongside this, so
// freshly-produced units are on hand the same turn (#71).
func (w *World) CollectIncome(e *Empire) {
	e.TurnStartGoldLost += w.addGold(e, int64(w.IncomeThisTurn(e).Gold()))
}

// productionBoost is the multiplier a realm's land gives to unit
// manufacturing, returned as an exact fraction num/den: each type's
// ProductionBoost times its share of the land, which under the base rules is
// Mountain's. BRE computes 1 + MountainIndustryNum*Mountain/Total in floating
// point and truncates only the final unit count, so rounding it to whole
// percent here costs real units.
//
// BRE ties industrial output to the *share* of the realm that is mountains, so
// the boost dilutes as an empire expands elsewhere — a realm cannot hold the cap
// without buying mountains alongside everything else.
func (w *World) productionBoost(e *Empire) (num, den int) {
	total := e.Regions.Total()
	if total == 0 {
		return 1, 1
	}
	rows := w.regionsOf(e)
	boost := 0
	for _, g := range RegionTypes {
		boost += rows[g].ProductionBoost * *g.Count(&e.Regions)
	}
	// Capped once boost/Total exceeds the cap's fraction.
	if (100 + 100*boost/total) > MountainIndustryCapPct {
		return MountainIndustryCapPct, 100
	}
	return total + boost, total
}

// ProductionBoostPercent reports productionBoost as a whole percent, for the
// Military advisor to quote. Display only — production itself uses the exact
// fraction.
func (w *World) ProductionBoostPercent(e *Empire) int {
	num, den := w.productionBoost(e)
	// int64: the fraction is a pair of raw land counts, and `int` is 32 bits on
	// the 32-bit door builds this project supports.
	return int((int64(num)*100 + int64(den)/2) / int64(den))
}

// unitPoints is the realm's unit pool before the boost: each type's
// UnitPoints times its count, Industrial's alone under the base rules.
func (w *World) unitPoints(e *Empire) int64 {
	rows := w.regionsOf(e)
	var points int64
	for _, g := range RegionTypes {
		points += int64(rows[g].UnitPoints) * int64(*g.Count(&e.Regions))
	}
	return points
}

// ProjectedProduction computes the units e would manufacture this turn at its
// current Industrial regions, percentages, and specialization — without
// applying them. Order matches the Set Industries screen: Troopers, Jets,
// Turrets, Bombers, Tanks, Carriers.
//
// The percentage split always governs how points are allocated. On top of that,
// specialization applies a per-unit efficiency modifier — a bonus to the
// specialized unit, a penalty to everything else — and Mountain regions boost
// the whole pool (see productionBoost).
//
// Binary-verified against BRE.OVR: the unit pool is UnitPointsPerRegion, which
// is NOT the gold pool, and BRE keeps the whole chain in floating point and
// ROUNDS once at the end. All three matter — using the gold pool overproduces by
// ~24%, rounding the mountain boost to whole percent loses up to 0.3%, and
// truncating instead of rounding is off by one unit whenever the fraction
// exceeds a half (which is what the 1086-industrial live sample caught).
func (w *World) ProjectedProduction(e *Empire) []int {
	boostNum, boostDen := w.productionBoost(e)
	points := w.unitPoints(e)
	made := func(name string, pct, cost int) int {
		spec := 100
		switch {
		case e.Specialized == name:
			spec = 100 + SpecialtyBonusPct
		case e.Specialized != "":
			spec = 100 - SpecialtyPenaltyPct
		}
		return unitsMade(points, pct, spec, boostNum, boostDen, e.TechUnitFactor(), cost)
	}
	out := make([]int, len(MilitaryGoods))
	for i, g := range MilitaryGoods {
		out[i] = made(g.Plural, *g.Prod(e), w.unit(g).Cost)
	}
	return out
}

// unitsMade is that chain, evaluated exactly and ROUNDED ONCE at the end.
//
// It is arbitrary-precision because int64 is not wide enough. Six factors are
// multiplied before anything is divided, and two of them — the mountain boost's
// numerator and denominator — are RAW LAND COUNTS rather than a reduced
// fraction, so the product grows with the SQUARE of the realm's size. It passed
// int64 at about 14,400 industrial regions and wrapped negative, which a realm
// that size reaches: the original's own capture in docs/dev/bre-screens.md shows
// one holding 14,203. Production then came back negative and was SUBTRACTED from
// the army.
//
// Dividing in stages instead would have fixed the overflow and broken the
// rounding, which is binary-verified: BRE keeps the whole chain in floating
// point and rounds once (see ProjectedProduction). This keeps every figure the
// engine produced below the old ceiling exactly as it was.
//
// points is the pool before the boost (World.unitPoints): Industrial regions
// times UnitPointsPerRegion under the base rules.
func unitsMade(points int64, pct, spec, boostNum, boostDen, tech, cost int) int {
	if points <= 0 || pct <= 0 || cost <= 0 || boostDen <= 0 {
		return 0
	}
	n := big.NewInt(points)
	for _, f := range []int64{int64(pct), int64(spec), int64(boostNum), int64(tech)} {
		n.Mul(n, big.NewInt(f))
	}
	d := big.NewInt(int64(cost))
	for _, f := range []int64{100, 100, int64(boostDen), TechFactorUnit} {
		d.Mul(d, big.NewInt(f))
	}
	// (n + d/2) / d — the round-half-up the whole chain is verified against.
	n.Add(n, new(big.Int).Rsh(d, 1))
	n.Div(n, d)
	if !n.IsInt64() {
		// Unreachable on any realm the game can build; a count that does not fit
		// is held at the top rather than wrapped, which is the failure this
		// function exists to end.
		return math.MaxInt
	}
	made := n.Int64()
	if made > math.MaxInt {
		return math.MaxInt
	}
	return int(made)
}

// Manufacture converts e's Industrial regions into production points and spends
// them on units per e.ProdXxx percentages (see ProjectedProduction).
// Specialization applies a per-unit efficiency bonus/penalty on top; it never
// overrides the percentage split. Called at turn start, alongside CollectIncome
// (#71). Industrial GOLD is not credited here — it flows through IncomeThisTurn
// (see industrialGold).
func (w *World) Manufacture(e *Empire) {
	proj := w.ProjectedProduction(e)
	for i, g := range MilitaryGoods {
		before := *g.Count(e)
		*g.Count(e) = addUnits(before, proj[i])
		*g.Made(e) = *g.Count(e) - before // what arrived, not what the cap turned away
	}
}
