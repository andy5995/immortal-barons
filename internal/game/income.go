package game

import (
	"encoding/binary"
	"hash/fnv"
	"io"
)

// income.go — what a realm earns and grows in a turn: the per-region draw, the
// industrial gold, the rivers, and what the population and industry produce.

// IncomeBreakdown itemizes a turn's income by source (gold), plus the food
// grown. The income report and the actual gold credit both derive from this,
// so what the player is shown equals what is credited to the last coin. Urban
// and Technology regions produce no direct gold (BRE-verified), so they are
// not listed here.
type IncomeBreakdown struct {
	Taxes, Ore, Tourism, Solar, Rivers, Industrial, Trade int
	Food                                                  int // grown by Agricultural regions
	RiverFood                                             int // fished from rivers this turn (0 on a hydropower turn)
}

// Gold sums the gold-producing sources.
func (b IncomeBreakdown) Gold() int {
	return b.Taxes + b.Ore + b.Tourism + b.Solar + b.Rivers + b.Industrial + b.Trade
}

// CrownTaxBase is the income the crown tax is levied on: the six region/tax
// income lines, excluding Trade. BRE accumulates its base at exactly six sites,
// one per income line, and trading proceeds are not among them.
func (b IncomeBreakdown) CrownTaxBase() int {
	return b.Taxes + b.Ore + b.Tourism + b.Solar + b.Rivers + b.Industrial
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
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(salt))
	h.Write(buf[:])
	io.WriteString(h, e.Name)
	return int(h.Sum32() % uint32(n))
}

// industrialGold is the empire's TOTAL industrial gold this turn (not per
// region). The UNALLOCATED share of production — what is left after the six unit
// percentages — is paid out as gold, so allocating to units trades directly
// against it and a realm at 100% allocation earns none.
//
// Binary-verified (BRE.OVR 0x34545–0x345D1), including the order of operations:
//
//	perRegion = [0, IndustryGoldRate) + IndustryGoldBase
//	total     = perRegion * count / 100 * unallocated%
//
// Dividing by 100 before applying the percentage is BRE's own order, and it is
// why industry is the one region type whose total does not divide evenly by its
// region count — the division discards the remainder.
func (w *World) industrialGold(e *Empire) int {
	allocated := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers
	// Whatever is not building units pays gold, whether the player set it aside
	// on the Gold row or simply left it unallocated.
	unalloc := 100 - allocated
	if unalloc < 0 {
		unalloc = 0
	}
	return w.ProjectedIndustrialGold(e, unalloc)
}

// ProjectedIndustrialGold is the gold that pct% of this empire's industrial
// capacity pays out this turn. Shared with the Set Industries screen so the
// figure a player is shown is the one industrialGold will credit — regionDraw
// varies by empire and game day, not per call, so the two always agree.
func (w *World) ProjectedIndustrialGold(e *Empire, pct int) int {
	if pct <= 0 {
		return 0
	}
	perRegion := w.regionDraw(e, 5, IndustryGoldRate) + IndustryGoldBase
	return perRegion * e.Regions.Industrial / 100 * pct
}

// riverGold is one River region's hydropower gold this turn: the full yield
// less the share taken as food (see RiverFishShare). Rivers have the highest
// base but an occasional "bad year" (a small deterministic chance, keyed off a
// separate yield salt) that halves the take.
func (w *World) riverGold(e *Empire) int {
	yield := w.regionDraw(e, 4, RiverRate) + RiverBase
	if w.regionDraw(e, 40, 100) < RiverDudChancePct {
		yield = RiverBase / 2
	}
	return yield * (100 - RiverFishShare) / 100
}

// IncomeThisTurn itemizes e's income for the current turn. Each region's gold
// is BRE's perRegion = Base + [0, Rate) times its region count; Coastal is
// additionally scaled by a support floor (0.10 + 0.90·support, so tourism never
// zeroes out). TechFactor scales every gold source (the tech-factor's role as
// an income multiplier is otherwise deferred to #20). Products are widened to
// int64 so they stay correct on 32-bit builds even at money-cap scale.
func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown {
	// Region income and population tax draw on DIFFERENT research slots in BRE,
	// so they scale independently.
	gold := int64(e.TechGoldFactor())
	scale := func(n int64) int { return int(n * gold / TechFactorUnit) }
	tax := int64(e.TechTaxFactor())
	scaleTax := func(n int64) int { return int(n * tax / TechFactorUnit) }
	perRegion := func(salt, rate, base int) int { return w.regionDraw(e, salt, rate) + base }

	support := 10 + 90*e.Support/100 // support factor ×100: 0.10 + 0.90·(Support/100)
	riverGold := w.riverGold(e)
	return IncomeBreakdown{
		Taxes:      scaleTax(int64(e.People) * int64(e.Tax) / 100 * TaxGoldPerCapita),
		Ore:        scale(int64(perRegion(1, MountainRate, MountainBase)) * int64(e.Regions.Mountain)),
		Tourism:    scale(int64(perRegion(2, CoastalRate, CoastalBase)) * int64(support) / 100 * int64(e.Regions.Coastal)),
		Solar:      scale(int64(perRegion(3, DesertRate, DesertBase)) * int64(e.Regions.Desert)),
		Rivers:     scale(int64(riverGold) * int64(e.Regions.River)),
		Industrial: scale(int64(w.industrialGold(e))),
		Trade:      w.tradeIncome(e), // trade-treaty bonus (population-scaled)
		Food:       w.FoodProduced(e),
		RiverFood:  w.riverFood(e),
	}
}

// riverFood is the food e's rivers fish this turn. Unlike BRE, where a river
// runs hydropower OR fishes and the empire finds out which only when the income
// report prints, IB's rivers always do both — RiverFishShare of the yield as
// food, the rest as gold (see riverGold). The haul itself has BRE's shape: a
// tech-raised base plus one draw per turn.
func (w *World) riverFood(e *Empire) int {
	perRegion := techRaise(RiverFishFood, e.TechFoodFactor()) + w.regionDraw(e, 7, RiverFishRate)
	return e.Regions.River * perRegion * RiverFishShare / 100
}

// FoodGrown is the empire's total food production this turn: its tech-boosted
// food-region output (FoodProduced) plus river fishing. The single source of
// truth for produced food, so the turn engine, the AI, and the advisors agree.
func (w *World) FoodGrown(e *Empire) int {
	return w.FoodProduced(e) + w.riverFood(e)
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
	w.creditGold(e, int64(w.IncomeThisTurn(e).Gold()), "this turn's income")
}

// Industrial production tuning (v1, tunable — see docs/mechanics-reference.md).
// Industrial gold is credited via IncomeThisTurn (see industrialGold); this
// governs unit production only.
// industryMountainBoost is the multiplier Mountain regions give to unit
// manufacturing, returned as an exact fraction num/den: BRE computes
// 1 + MountainIndustryNum*Mountain/Total in floating point and truncates only
// the final unit count, so rounding it to whole percent here costs real units.
//
// BRE ties industrial output to the *share* of the realm that is mountains, so
// the boost dilutes as an empire expands elsewhere — a realm cannot hold the cap
// without buying mountains alongside everything else.
func industryMountainBoost(r RegionMix) (num, den int) {
	total := r.Total()
	if total == 0 {
		return 1, 1
	}
	// Capped once MountainIndustryNum*Mountain/Total exceeds the cap's fraction.
	if (100 + MountainIndustryNum*100*r.Mountain/total) > MountainIndustryCapPct {
		return MountainIndustryCapPct, 100
	}
	return total + MountainIndustryNum*r.Mountain, total
}

// MountainIndustryPercent reports industryMountainBoost as a whole percent, for
// the Military advisor to quote. Display only — production itself uses the
// exact fraction.
func MountainIndustryPercent(r RegionMix) int {
	num, den := industryMountainBoost(r)
	return (num*100 + den/2) / den
}

// ProjectedProduction computes the units e would manufacture this turn at its
// current Industrial regions, percentages, and specialization — without
// applying them. Order matches the Set Industries screen: Troopers, Jets,
// Turrets, Bombers, Tanks, Carriers.
//
// The percentage split always governs how points are allocated. On top of that,
// specialization applies a per-unit efficiency modifier — a bonus to the
// specialized unit, a penalty to everything else — and Mountain regions boost
// the whole pool (see industryMountainBoost).
//
// Binary-verified against BRE.OVR: the unit pool is UnitPointsPerRegion, which
// is NOT the gold pool, and BRE keeps the whole chain in floating point and
// ROUNDS once at the end. All three matter — using the gold pool overproduces by
// ~24%, rounding the mountain boost to whole percent loses up to 0.3%, and
// truncating instead of rounding is off by one unit whenever the fraction
// exceeds a half (which is what the 1086-industrial live sample caught).
func (w *World) ProjectedProduction(e *Empire) []int {
	boostNum, boostDen := industryMountainBoost(e.Regions)
	made := func(name string, pct, cost int) int {
		spec := 100
		switch {
		case e.Specialized == name:
			spec = 100 + SpecialtyBonusPct
		case e.Specialized != "":
			spec = 100 - SpecialtyPenaltyPct
		}
		n := int64(e.Regions.Industrial) * UnitPointsPerRegion *
			int64(pct) * int64(spec) * int64(boostNum) * int64(e.TechUnitFactor())
		d := int64(cost) * 100 * 100 * int64(boostDen) * TechFactorUnit
		return int((n + d/2) / d)
	}
	out := make([]int, len(MilitaryGoods))
	for i, g := range MilitaryGoods {
		out[i] = made(g.Plural, *g.Prod(e), g.Cost)
	}
	return out
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
		*g.Made(e) = proj[i]
		*g.Count(e) += proj[i]
	}
}
