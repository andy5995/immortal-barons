package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// hqField is one key of values.toml's [hq] section. The HeadQuarters is one
// building per realm, so its figures are one section, not one per unit. Like
// unitFields, this table is the only place a key is named: reading, Diff and
// Values all go through it.
type hqField struct {
	key string
	get func(h game.HQRules) string
	set func(h *game.HQRules, v any) error
}

var hqFields = []hqField{
	{"tank_bonus",
		func(h game.HQRules) string { return game.FixedPoint(int64(h.TankPerHQ), 100) },
		func(h *game.HQRules, v any) (err error) {
			var n int64
			n, err = fixed(v, 100)
			h.TankPerHQ = int(n)
			return
		}},
	{"build_start",
		func(h game.HQRules) string { return fmt.Sprint(h.BuildStart) },
		func(h *game.HQRules, v any) (err error) { h.BuildStart, err = whole(v); return }},
	{"build_per_turn",
		func(h game.HQRules) string { return fmt.Sprint(h.BuildPerTurn) },
		func(h *game.HQRules, v any) (err error) { h.BuildPerTurn, err = whole(v); return }},
	{"battle_loss_min",
		func(h game.HQRules) string { return fmt.Sprint(h.BattleLossMin) },
		func(h *game.HQRules, v any) (err error) { h.BattleLossMin, err = whole(v); return }},
	{"battle_loss_jitter",
		func(h game.HQRules) string { return fmt.Sprint(h.BattleLossJitter) },
		func(h *game.HQRules, v any) (err error) { h.BattleLossJitter, err = whole(v); return }},
	{"price_base",
		func(h game.HQRules) string { return fmt.Sprint(h.PriceBase) },
		func(h *game.HQRules, v any) (err error) { h.PriceBase, err = whole64(v); return }},
	{"price_per_turn",
		func(h game.HQRules) string { return fmt.Sprint(h.PricePerTurn) },
		func(h *game.HQRules, v any) (err error) { h.PricePerTurn, err = whole64(v); return }},
	{"price_jitter",
		func(h game.HQRules) string { return fmt.Sprint(h.PriceJitter) },
		func(h *game.HQRules, v any) (err error) { h.PriceJitter, err = whole64(v); return }},
	{"price_cap",
		func(h game.HQRules) string { return fmt.Sprint(h.PriceCap) },
		func(h *game.HQRules, v any) (err error) { h.PriceCap, err = whole64(v); return }},
	{"price_cap_jitter",
		func(h game.HQRules) string { return fmt.Sprint(h.PriceCapJitter) },
		func(h *game.HQRules, v any) (err error) { h.PriceCapJitter, err = whole64(v); return }},
}

func hqKey(key string) *hqField {
	for i := range hqFields {
		if hqFields[i].key == key {
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
func (m *Mod) applyHQ(r *game.Rules) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(m.values.HQ)) {
		f := hqKey(key)
		if f == nil {
			errs = append(errs, fmt.Errorf("hq.%s: unknown key", key))
			continue
		}
		if err := f.set(&r.HQ, m.values.HQ[key]); err != nil {
			errs = append(errs, fmt.Errorf("hq.%s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

// diffHQ lists the [hq] values that differ between a and b.
func diffHQ(a, b *game.Rules) []string {
	var out []string
	for _, f := range hqFields {
		if from, to := f.get(a.HQ), f.get(b.HQ); from != to {
			out = append(out, fmt.Sprintf("hq.%s: %s -> %s", f.key, from, to))
		}
	}
	return out
}

// writeHQ prints the [hq] section as a values.toml would set it.
func writeHQ(b *strings.Builder, r *game.Rules) {
	b.WriteString("\n[hq]\n")
	for _, f := range hqFields {
		fmt.Fprintf(b, "%s = %s\n", f.key, f.get(r.HQ))
	}
}
