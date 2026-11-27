package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A flat section of values.toml ([hq], [combat]) is a game.Field table over one
// struct of Rules. Reading, -check-mod's list of changes and -print-base-mod all
// go through these three, so a section's keys are named in one place.

// whole64 reads v as a whole number. The limits are Rules.Validate's.
func whole64(v any) (int64, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("want a whole number, got %v", v)
	}
	return n, nil
}

// applyFields writes the keys in in, the named section's table, into t.
func applyFields[T any](section string, fields []game.Field[T], in map[string]any, t *T) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(in)) {
		f := game.FieldByKey(fields, key)
		if f == nil {
			errs = append(errs, fmt.Errorf("%s.%s: unknown key", section, key))
			continue
		}
		var n int64
		var err error
		if f.Wide {
			n, err = whole64(in[key])
		} else {
			var i int
			i, err = whole(in[key])
			n = int64(i)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", section, key, err))
			continue
		}
		f.Set(t, n)
	}
	return errors.Join(errs...)
}

// diffFields lists the figures that differ between a and b.
func diffFields[T any](section string, fields []game.Field[T], a, b T) []string {
	var out []string
	for _, f := range fields {
		if from, to := f.Get(a), f.Get(b); from != to {
			out = append(out, fmt.Sprintf("%s.%s: %d -> %d", section, f.Key, from, to))
		}
	}
	return out
}

// writeFields prints the section as a values.toml would set it.
func writeFields[T any](b *strings.Builder, section string, fields []game.Field[T], t T) {
	fmt.Fprintf(b, "\n[%s]\n", section)
	for _, f := range fields {
		fmt.Fprintf(b, "%s = %d\n", f.Key, f.Get(t))
	}
}
