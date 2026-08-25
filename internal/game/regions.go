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

// foodProduced is the per-turn food GROWN by Agricultural regions, given this
// turn's per-region yield (World.agriFood draws it). River fishing food is
// separate and conditional (World.riverFood, #29), since rivers fish OR run
// hydropower each turn, not both.
func (r RegionMix) foodProduced(perRegion int) int {
	return r.Agricultural * perRegion
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

// addMix adds another mix's counts into this one.
func (r *RegionMix) addMix(m RegionMix) {
	rf := r.fields()
	mf := (&m).fields()
	for i := range rf {
		*rf[i] += *mf[i]
	}
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
	// Key is the letter the player presses to choose it. Waste has none — it
	// cannot be bought, sold or allocated, only decontaminated.
	Key   byte
	Count func(m *RegionMix) *int
}

// The rows. Referred to by variable everywhere, so a screen's order cannot
// drift out of step with the field it labels.
var (
	Coastal      = &Region{Name: "Coastal", Key: 'C', Count: func(m *RegionMix) *int { return &m.Coastal }}
	Mountain     = &Region{Name: "Mountain", Key: 'M', Count: func(m *RegionMix) *int { return &m.Mountain }}
	Desert       = &Region{Name: "Desert", Key: 'D', Count: func(m *RegionMix) *int { return &m.Desert }}
	River        = &Region{Name: "River", Key: 'R', Count: func(m *RegionMix) *int { return &m.River }}
	Agricultural = &Region{Name: "Agricultural", Key: 'A', Count: func(m *RegionMix) *int { return &m.Agricultural }}
	Urban        = &Region{Name: "Urban", Key: 'U', Count: func(m *RegionMix) *int { return &m.Urban }}
	Industrial   = &Region{Name: "Industrial", Key: 'I', Count: func(m *RegionMix) *int { return &m.Industrial }}
	Technology   = &Region{Name: "Technology", Key: 'T', Count: func(m *RegionMix) *int { return &m.Technology }}
	Waste        = &Region{Name: "Waste", Count: func(m *RegionMix) *int { return &m.Waste }}
)

// mixOrder is the order RegionMix declares its fields in, and the order a
// proportional add or remove walks them. It is NOT arbitrary: remove() breaks a
// remainder tie in favour of the field it meets first, so reordering this
// changes which region loses a unit when two are the same size.
var mixOrder = []*Region{Coastal, Mountain, Desert, River, Agricultural, Urban, Industrial, Technology, Waste}

// BuyableRegions is the eight types a player may buy, in BRE's Buy Regions
// screen order (verified live, #17 menu audit). Waste is absent because nothing
// can buy it.
var BuyableRegions = []*Region{Coastal, River, Agricultural, Desert, Industrial, Urban, Mountain, Technology}
