package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// applyCombat writes the [combat] section's keys into r. Every key is a whole
// number; the limits are Rules.Validate's, from game.CombatFields.
func (v values) applyCombat(r *game.Rules) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(v.Combat)) {
		f := game.CombatFieldByKey(key)
		if f == nil {
			errs = append(errs, fmt.Errorf("combat.%s: unknown key", key))
			continue
		}
		n, err := whole(v.Combat[key])
		if err != nil {
			errs = append(errs, fmt.Errorf("combat.%s: %w", key, err))
			continue
		}
		f.Set(&r.Combat, int64(n))
	}
	return errors.Join(errs...)
}

// diffCombat lists the [combat] values that differ between a and b.
func diffCombat(a, b *game.Rules) []string {
	var out []string
	for _, f := range game.CombatFields {
		if from, to := f.Get(a.Combat), f.Get(b.Combat); from != to {
			out = append(out, fmt.Sprintf("combat.%s: %d -> %d", f.Key, from, to))
		}
	}
	return out
}

// writeCombat prints the [combat] section as a values.toml would set it.
func writeCombat(b *strings.Builder, r *game.Rules) {
	b.WriteString("\n[combat]\n")
	for _, f := range game.CombatFields {
		fmt.Fprintf(b, "%s = %d\n", f.Key, f.Get(r.Combat))
	}
}
