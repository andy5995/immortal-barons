package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// hqField is one key of values.toml's [hq] section: a game.HQField with the
// reader that writes its value. The HeadQuarters is one building per realm, so
// its figures are one section, not one per unit. Keys, limits, Diff and Values
// all come from game.HQFields.
type hqField struct {
	game.HQField
	set func(h *game.HQRules, v any) error
}

var hqFields = func() []hqField {
	out := make([]hqField, 0, len(game.HQFields))
	for _, f := range game.HQFields {
		parse := func(v any) (int64, error) { n, err := whole(v); return int64(n), err }
		if f.Wide {
			parse = whole64
		}
		out = append(out, hqField{
			HQField: f,
			set: func(h *game.HQRules, v any) error {
				n, err := parse(v)
				if err != nil {
					return err
				}
				f.Set(h, n)
				return nil
			},
		})
	}
	return out
}()

func hqKey(key string) *hqField {
	for i := range hqFields {
		if hqFields[i].Key == key {
			return &hqFields[i]
		}
	}
	return nil
}

// whole64 reads v as a whole number. The limits are Rules.Validate's.
func whole64(v any) (int64, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("want a whole number, got %v", v)
	}
	return n, nil
}

// applyHQ writes the [hq] section's keys into r.
func (v values) applyHQ(r *game.Rules) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(v.HQ)) {
		f := hqKey(key)
		if f == nil {
			errs = append(errs, fmt.Errorf("hq.%s: unknown key", key))
			continue
		}
		if err := f.set(&r.HQ, v.HQ[key]); err != nil {
			errs = append(errs, fmt.Errorf("hq.%s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

// diffHQ lists the [hq] values that differ between a and b.
func diffHQ(a, b *game.Rules) []string {
	var out []string
	for _, f := range hqFields {
		if from, to := f.Get(a.HQ), f.Get(b.HQ); from != to {
			out = append(out, fmt.Sprintf("hq.%s: %d -> %d", f.Key, from, to))
		}
	}
	return out
}

// writeHQ prints the [hq] section as a values.toml would set it.
func writeHQ(b *strings.Builder, r *game.Rules) {
	b.WriteString("\n[hq]\n")
	for _, f := range hqFields {
		fmt.Fprintf(b, "%s = %d\n", f.Key, f.Get(r.HQ))
	}
}
