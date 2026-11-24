package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The [regions.<id>] sections of values.toml, one per region type. The keys and
// their limits are game.RegionFields': this file only reads, diffs and prints
// them, so a figure is named in one place.

// regionIDs are the region types a values file may name, in RegionTypes order.
func regionIDs() []string {
	var ids []string
	for _, g := range game.RegionTypes {
		ids = append(ids, g.ID)
	}
	return ids
}

// applyRegions writes the [regions.*] sections' keys into r.
func (v values) applyRegions(r *game.Rules) error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(v.Regions)) {
		g := game.RegionByID(id)
		if g == nil {
			errs = append(errs, fmt.Errorf("regions.%s: not a region type (%s)", id, strings.Join(regionIDs(), ", ")))
			continue
		}
		rr := r.Regions[g]
		for _, key := range slices.Sorted(maps.Keys(v.Regions[id])) {
			f := game.RegionFieldByKey(key)
			if f == nil {
				var known []string
				for _, f := range game.RegionFields {
					known = append(known, f.Key)
				}
				errs = append(errs, fmt.Errorf("regions.%s.%s: unknown key (%s)", id, key, strings.Join(known, ", ")))
				continue
			}
			n, err := whole(v.Regions[id][key])
			if err != nil {
				errs = append(errs, fmt.Errorf("regions.%s.%s: %w", id, key, err))
				continue
			}
			f.Set(&rr, n)
		}
		r.Regions[g] = rr
	}
	return errors.Join(errs...)
}

// diffRegions lists the region figures that differ between a and b.
func diffRegions(a, b *game.Rules) []string {
	var out []string
	for _, g := range game.RegionTypes {
		for _, f := range game.RegionFields {
			if from, to := f.Get(a.Regions[g]), f.Get(b.Regions[g]); from != to {
				out = append(out, fmt.Sprintf("%s.%s: %d -> %d", g.ID, f.Key, from, to))
			}
		}
	}
	return out
}

// writeRegions prints a [regions.<id>] section per type as a values.toml would
// set it.
func writeRegions(b *strings.Builder, r *game.Rules) {
	for _, g := range game.RegionTypes {
		fmt.Fprintf(b, "\n[regions.%s]\n", g.ID)
		for _, f := range game.RegionFields {
			fmt.Fprintf(b, "%s = %d\n", f.Key, f.Get(r.Regions[g]))
		}
	}
}
