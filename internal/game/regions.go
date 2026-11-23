package game

// RegionMix is an empire's land broken down by type. Its Total must always
// equal Empire.Land.
//
// Waste is land ruined by a nuclear or chemical strike. It is a region type in
// its own right, not destroyed land: it stays on the books, still costs region
// maintenance, and produces nothing until it is decontaminated. Nothing can buy
// it, and no production path may read it — only Total, remove and the
// decontamination step touch it.
type RegionMix struct {
	Coastal      int
	Mountain     int
	Desert       int
	River        int
	Agricultural int
	Urban        int
	Industrial   int
	Technology   int
	Waste        int `json:"waste,omitempty"`
}

func (r RegionMix) Total() int {
	return r.Coastal + r.Mountain + r.Desert + r.River +
		r.Agricultural + r.Urban + r.Industrial + r.Technology + r.Waste
}

// fields returns pointers to each count in mixOrder, for generic proportional
// add/remove. Waste is included: the original spreads a loss over every type it
// holds, waste among them, so a realm already carrying waste absorbs part of
// the next strike with land that was ruined already.
func (r *RegionMix) fields() []*int {
	f := make([]*int, len(mixOrder))
	for i, reg := range mixOrder {
		f[i] = reg.Count(r)
	}
	return f
}

// remove takes n regions away, spread proportionally across the mix (largest
// first for any remainder), and returns what was removed as its own mix.
// Caller must then resync Empire.Land. Never drives any field below 0; if
// n >= Total, removes everything.
func (r *RegionMix) remove(n int) RegionMix {
	if n <= 0 {
		return RegionMix{}
	}
	total := r.Total()
	if n >= total {
		removed := *r
		*r = RegionMix{}
		return removed
	}

	fields := r.fields()
	removed := make([]int, len(fields))
	remaining := n
	for i, f := range fields {
		take := *f * n / total
		if take > *f {
			take = *f
		}
		*f -= take
		removed[i] = take
		remaining -= take
	}

	// Spread any remainder one-by-one, always taking from the currently
	// largest field so the mix stays proportionally balanced.
	for remaining > 0 {
		maxIdx := -1
		for i, f := range fields {
			if *f <= 0 {
				continue
			}
			if maxIdx == -1 || *f > *fields[maxIdx] {
				maxIdx = i
			}
		}
		if maxIdx == -1 {
			break // nothing left to take (shouldn't happen since n < total)
		}
		*fields[maxIdx]--
		removed[maxIdx]++
		remaining--
	}

	var result RegionMix
	rf := result.fields()
	for i := range rf {
		*rf[i] = removed[i]
	}
	return result
}

// defaultRegionMix distributes `land` regions across types using the same
// proportions a new empire starts with (Coastal 40%, Agricultural 25%,
// Urban 10%, Mountain 10%, Desert 5%, River 10%), giving any rounding
// remainder to Coastal so the total is exactly `land`.
func defaultRegionMix(land int) RegionMix {
	if land <= 0 {
		return RegionMix{}
	}
	m := RegionMix{
		Mountain:     land * 10 / 100,
		Desert:       land * 5 / 100,
		River:        land * 10 / 100,
		Agricultural: land * 25 / 100,
		Urban:        land * 10 / 100,
	}
	m.Coastal = land - (m.Mountain + m.Desert + m.River + m.Agricultural + m.Urban) // remainder -> Coastal
	return m
}

// Region is one type of land a realm holds: its English name, BRE's selection
// letter, and the accessor for a mix's count of it.
//
// This is the RegionMix analogue of Good in units.go, and exists for the same
// reason (#134). internal/menu carried three lists — names, selection keys and
// field pointers — coupled to each other by index, while RegionMix itself
// declared the set a fourth time in a different order. Nothing checked that any
// of them still agreed. Each type is declared once here now, and a screen that
// wants its own order declares a slice of these rows rather than restating the
// names, so a mismatch is a compile error instead of a wrong label.
//
// Name is an identity key as well as a display label, so it stays ENGLISH;
// tr() translates at render time.
type Region struct {
	Name string
	// ID names the type in a mod's values.toml and script: lowercase, and never
	// translated.
	ID string
	// Key is the letter the player presses to choose it. Waste has none — it
	// cannot be bought, sold or allocated, only decontaminated.
	Key   byte
	Count func(m *RegionMix) *int

	// goldSalt, idleSalt and foodSalt pick the type's own per-turn draws
	// (World.regionDraw), one for each kind of yield, so no two yields share a
	// roll. They are fixed, not rules: a draw's salt is what makes a fixed seed
	// replay, and the types the original pays (Mountain ore 1 … River fishing 7)
	// keep the salts they always had.
	goldSalt, idleSalt, foodSalt int

	// GoldText and FoodText are the turn report's line for gold and food this
	// type produced, after the amount.
	GoldText, FoodText string

	// index is the type's place in mixOrder, which IncomeBreakdown's per-type
	// arrays are indexed by.
	index int
}

// The rows. Referred to by variable everywhere, so a screen's order cannot
// drift out of step with the field it labels.
var (
	Coastal = &Region{Name: "Coastal", ID: "coastal", Key: 'C', Count: func(m *RegionMix) *int { return &m.Coastal },
		goldSalt: 2, idleSalt: 8, foodSalt: 9,
		GoldText: msgid("gold was earned in Tourism."),
		FoodText: msgid("Food units were produced by Coastal regions.")}
	Mountain = &Region{Name: "Mountain", ID: "mountain", Key: 'M', Count: func(m *RegionMix) *int { return &m.Mountain },
		goldSalt: 1, idleSalt: 10, foodSalt: 11,
		GoldText: msgid("gold was produced from the Ore Mines."),
		FoodText: msgid("Food units were produced by Mountain regions.")}
	Desert = &Region{Name: "Desert", ID: "desert", Key: 'D', Count: func(m *RegionMix) *int { return &m.Desert },
		goldSalt: 3, idleSalt: 12, foodSalt: 13,
		GoldText: msgid("gold was earned by Solar Power Generators."),
		FoodText: msgid("Food units were produced by Desert regions.")}
	River = &Region{Name: "River", ID: "river", Key: 'R', Count: func(m *RegionMix) *int { return &m.River },
		goldSalt: 4, idleSalt: 14, foodSalt: 7,
		GoldText: msgid("gold was earned from Rivers."),
		FoodText: msgid("Food units were fished from the rivers.")}
	Agricultural = &Region{Name: "Agricultural", ID: "agricultural", Key: 'A', Count: func(m *RegionMix) *int { return &m.Agricultural },
		goldSalt: 15, idleSalt: 16, foodSalt: 6,
		GoldText: msgid("gold was earned from Agricultural regions."),
		FoodText: msgid("Food units were grown.")}
	Urban = &Region{Name: "Urban", ID: "urban", Key: 'U', Count: func(m *RegionMix) *int { return &m.Urban },
		goldSalt: 17, idleSalt: 18, foodSalt: 19,
		GoldText: msgid("gold was earned from Urban regions."),
		FoodText: msgid("Food units were produced by Urban regions.")}
	Industrial = &Region{Name: "Industrial", ID: "industrial", Key: 'I', Count: func(m *RegionMix) *int { return &m.Industrial },
		goldSalt: 20, idleSalt: 5, foodSalt: 21,
		GoldText: msgid("gold was earned from Industrial Zones."),
		FoodText: msgid("Food units were produced by Industrial regions.")}
	Technology = &Region{Name: "Technology", ID: "technology", Key: 'T', Count: func(m *RegionMix) *int { return &m.Technology },
		goldSalt: 22, idleSalt: 23, foodSalt: 24,
		GoldText: msgid("gold was earned from Technology regions."),
		FoodText: msgid("Food units were produced by Technology regions.")}
	// Waste produces nothing (Rules.Validate holds it to that), so it has no
	// draws and no report lines.
	Waste = &Region{Name: "Waste", ID: "waste", Count: func(m *RegionMix) *int { return &m.Waste }}
)

// mixOrder is the order RegionMix declares its fields in, and the order a
// proportional add or remove walks them. It is NOT arbitrary: remove() breaks a
// remainder tie in favor of the field it meets first, so reordering this
// changes which region loses a unit when two are the same size.
var mixOrder = []*Region{Coastal, Mountain, Desert, River, Agricultural, Urban, Industrial, Technology, Waste}

// BuyableRegions is the eight types a player may buy, in BRE's Buy Regions
// screen order (verified live, #17 menu audit). Waste is absent because nothing
// can buy it.
var BuyableRegions = []*Region{Coastal, River, Agricultural, Desert, Industrial, Urban, Mountain, Technology}

// RegionTypes is every region type, Waste included, in RegionMix's order: the
// rows a mod's [regions.<id>] sections and a region script name.
var RegionTypes = mixOrder

// regionKinds is how many region types there are, for IncomeBreakdown's
// per-type arrays.
const regionKinds = 9

// The turn report lists gold and food in these orders: the original's own for
// the five gold lines and two food lines it prints, then the types that pay
// nothing without a mod.
var (
	GoldReportOrder = []*Region{Mountain, Coastal, Desert, River, Industrial, Agricultural, Urban, Technology}
	FoodReportOrder = []*Region{Agricultural, River, Coastal, Mountain, Desert, Urban, Industrial, Technology}
)

func init() {
	if len(mixOrder) != regionKinds {
		panic("regionKinds does not match mixOrder")
	}
	for i, r := range mixOrder {
		r.index = i
	}
}
