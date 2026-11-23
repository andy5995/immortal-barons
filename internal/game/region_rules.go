package game

import "fmt"

// RegionRules is everything one region type does, as figures a mod can change
// (#202). Every type carries every field, so what a type does is a matter of
// which figures are set rather than of which type it is: in the base rules only
// River fishes and only Industrial pays idle gold, but a mod may give a Desert
// food or an Urban region gold, and the engine computes it the same way.
//
// The engine's formulas read these and nothing else, so the base rules
// (baseRegionRules, from the balance*.go constants) reproduce the original's
// region behavior exactly.
type RegionRules struct {
	// GoldBase and GoldRate are a region's gold each turn before scaling: Base
	// plus a draw in [0, Rate). GoldShare is the percent of that paid as gold
	// (a River fishes the rest). SupportShare is the percent of the gold that
	// follows popular support (see CoastalSupportShare). Region gold is raised by
	// the gold technology factor.
	GoldBase, GoldRate, GoldShare, SupportShare int
	// IdleGoldBase and IdleGoldRate are the gold a region pays on the share of
	// production not allocated to units, per percent of it, as Industrial does
	// (World.industrialGold). Technology does not raise it.
	IdleGoldBase, IdleGoldRate int
	// FoodBase and FoodRate are a region's food each turn: Base, raised by the
	// food technology factor, plus a draw in [0, Rate). FoodShare is the percent
	// of that produced.
	FoodBase, FoodRate, FoodShare int
	// PopCap is the type's weight in the population capacity, in BRE's unit of
	// a million people (see popCapacity).
	PopCap int
	// Upkeep is the gold one region costs to maintain each turn, before
	// technology and the Maintenance Costs setting.
	Upkeep int
	// UnitPoints is the production a region adds to the realm's unit pool.
	UnitPoints int
	// ProductionBoost raises the whole unit pool by this many times the type's
	// share of the realm's land, as Mountain does, up to MountainIndustryCapPct.
	ProductionBoost int
	// Research is the type's weight in the realm's research count, as
	// Technology's (advanceTech).
	Research int
}

// baseRegionRules are the original's regions, from the constants.
func baseRegionRules() map[*Region]RegionRules {
	row := func(r RegionRules) RegionRules {
		r.Upkeep = RegionUpkeepPerLand
		if r.GoldShare == 0 {
			r.GoldShare = 100
		}
		if r.FoodShare == 0 {
			r.FoodShare = 100
		}
		return r
	}
	return map[*Region]RegionRules{
		Coastal: row(RegionRules{GoldBase: CoastalBase, GoldRate: CoastalRate,
			SupportShare: CoastalSupportShare, PopCap: PopCapCoastal}),
		Mountain: row(RegionRules{GoldBase: MountainBase, GoldRate: MountainRate,
			PopCap: PopCapMountain, ProductionBoost: MountainIndustryNum}),
		Desert: row(RegionRules{GoldBase: DesertBase, GoldRate: DesertRate,
			PopCap: PopCapDesert}),
		River: row(RegionRules{GoldBase: RiverBase, GoldRate: RiverRate, GoldShare: 100 - RiverFishShare,
			FoodBase: RiverFishFood, FoodRate: RiverFishRate, FoodShare: RiverFishShare,
			PopCap: PopCapRiver}),
		Agricultural: row(RegionRules{FoodBase: FoodAgriBase, FoodRate: FoodAgriRate,
			PopCap: PopCapAgricultural}),
		Urban: row(RegionRules{PopCap: PopCapUrban}),
		Industrial: row(RegionRules{IdleGoldBase: IndustryGoldBase, IdleGoldRate: IndustryGoldRate,
			UnitPoints: UnitPointsPerRegion, PopCap: PopCapIndustrial}),
		Technology: row(RegionRules{Research: 1, PopCap: PopCapTechnology}),
		Waste:      row(RegionRules{PopCap: PopCapWaste}),
	}
}

// RegionField is one figure of RegionRules as a mod names it: the key in a
// values.toml [regions.<id>] section and in a region script, its limits, and
// how to read and write it. Reading a mod, checking one, printing the base
// values and validating the result all go through RegionFields, so a figure is
// named in one place.
type RegionField struct {
	Key      string
	Min, Max int
	Get      func(r RegionRules) int
	Set      func(r *RegionRules, v int)
}

// The limits a mod's region figures must stay inside. Each keeps the engine's
// arithmetic safe on the 32-bit door builds, where a region count (up to 2^31)
// times a figure is computed: every such product is taken in int64 or big.Int
// and a total saturates at what an int holds (fitInt), and these bounds keep
// the per-region figure itself, and a share or a sum of two, inside an int.
const (
	ModRegionGoldMax       = 1_000_000 // gold a region a turn, base or rate
	ModRegionFoodMax       = 100_000   // food a region a turn, base or rate
	ModRegionPopCapMax     = 10_000
	ModRegionUpkeepMax     = 100_000
	ModRegionUnitPointsMax = 100_000
	ModRegionBoostMax      = 100
	ModRegionResearchMax   = 100
)

// RegionFields are the figures a mod can set on a region type, in the order
// -print-base-mod writes them.
var RegionFields = []RegionField{
	{"gold_base", 0, ModRegionGoldMax, func(r RegionRules) int { return r.GoldBase }, func(r *RegionRules, v int) { r.GoldBase = v }},
	{"gold_rate", 0, ModRegionGoldMax, func(r RegionRules) int { return r.GoldRate }, func(r *RegionRules, v int) { r.GoldRate = v }},
	{"gold_share", 0, 100, func(r RegionRules) int { return r.GoldShare }, func(r *RegionRules, v int) { r.GoldShare = v }},
	{"support_share", 0, 100, func(r RegionRules) int { return r.SupportShare }, func(r *RegionRules, v int) { r.SupportShare = v }},
	{"idle_gold_base", 0, ModRegionGoldMax, func(r RegionRules) int { return r.IdleGoldBase }, func(r *RegionRules, v int) { r.IdleGoldBase = v }},
	{"idle_gold_rate", 0, ModRegionGoldMax, func(r RegionRules) int { return r.IdleGoldRate }, func(r *RegionRules, v int) { r.IdleGoldRate = v }},
	{"food_base", 0, ModRegionFoodMax, func(r RegionRules) int { return r.FoodBase }, func(r *RegionRules, v int) { r.FoodBase = v }},
	{"food_rate", 0, ModRegionFoodMax, func(r RegionRules) int { return r.FoodRate }, func(r *RegionRules, v int) { r.FoodRate = v }},
	{"food_share", 0, 100, func(r RegionRules) int { return r.FoodShare }, func(r *RegionRules, v int) { r.FoodShare = v }},
	{"pop_cap", 0, ModRegionPopCapMax, func(r RegionRules) int { return r.PopCap }, func(r *RegionRules, v int) { r.PopCap = v }},
	{"upkeep", 0, ModRegionUpkeepMax, func(r RegionRules) int { return r.Upkeep }, func(r *RegionRules, v int) { r.Upkeep = v }},
	{"unit_points", 0, ModRegionUnitPointsMax, func(r RegionRules) int { return r.UnitPoints }, func(r *RegionRules, v int) { r.UnitPoints = v }},
	{"production_boost", 0, ModRegionBoostMax, func(r RegionRules) int { return r.ProductionBoost }, func(r *RegionRules, v int) { r.ProductionBoost = v }},
	{"research", 0, ModRegionResearchMax, func(r RegionRules) int { return r.Research }, func(r *RegionRules, v int) { r.Research = v }},
}

// RegionFieldByKey is the field named key, or nil.
func RegionFieldByKey(key string) *RegionField {
	for i := range RegionFields {
		if RegionFields[i].Key == key {
			return &RegionFields[i]
		}
	}
	return nil
}

// RegionByID is the region type with that ID, or nil.
func RegionByID(id string) *Region {
	for _, r := range RegionTypes {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// Validate reports every figure of g's rules outside its limits. Waste is held
// to producing nothing: it is land a strike ruined, and decontamination is the
// only thing that turns it back into land that yields.
func (rr RegionRules) Validate(g *Region) []error {
	var errs []error
	for _, f := range RegionFields {
		v := f.Get(rr)
		if v < f.Min || v > f.Max {
			errs = append(errs, fmt.Errorf("%s: %s %d is outside %d to %d", g.ID, f.Key, v, f.Min, f.Max))
		}
		if g == Waste && f.Key != "upkeep" && f.Key != "gold_share" && f.Key != "food_share" && v != 0 {
			errs = append(errs, fmt.Errorf("waste: %s must be 0: ruined land produces nothing", f.Key))
		}
	}
	return errs
}
