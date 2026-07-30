package game

// RegionMix is an empire's land broken down by type. Its Total must always
// equal Empire.Land.
type RegionMix struct {
	Coastal      int
	Mountain     int
	Desert       int
	River        int
	Agricultural int
	Urban        int
	Industrial   int
	Technology   int
}

func (r RegionMix) Total() int {
	return r.Coastal + r.Mountain + r.Desert + r.River +
		r.Agricultural + r.Urban + r.Industrial + r.Technology
}

// fields returns pointers to each count in a stable order, for generic
// proportional add/remove.
func (r *RegionMix) fields() []*int {
	return []*int{&r.Coastal, &r.Mountain, &r.Desert, &r.River,
		&r.Agricultural, &r.Urban, &r.Industrial, &r.Technology}
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
