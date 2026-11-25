package mod

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The [attacks.<kind>.<side>] sections of values.toml. A side is "target" (the
// realm attacked) or "attacker" (blowback on the realm that attacked), and a
// kind is a dotted name: [attacks.nuclear.target] is kind nuclear,
// [attacks.conventional.won.target] is kind conventional.won. The reader walks
// the nested tables down to the side, so a kind's depth is not fixed here. The
// keys and their limits are game.AttackFields'.

// attackSection is one [attacks.<kind>.<side>] table.
type attackSection struct {
	kind game.EffectKind
	side string
	keys map[string]any
}

// attackSections finds the sections in raw, the decoded [attacks] table.
func attackSections(raw map[string]any) ([]attackSection, []error) {
	var out []attackSection
	var errs []error
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for _, name := range slices.Sorted(maps.Keys(m)) {
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			sub, ok := m[name].(map[string]any)
			switch {
			case !ok:
				errs = append(errs, fmt.Errorf("attacks.%s: want a table; effects go in [attacks.<kind>.target] or [attacks.<kind>.attacker]", path))
			case slices.Contains(game.AttackSides, name):
				if prefix == "" {
					errs = append(errs, fmt.Errorf("attacks.%s: name the kind first, as in [attacks.nuclear.%s]", name, name))
					continue
				}
				out = append(out, attackSection{kind: game.EffectKind(prefix), side: name, keys: sub})
			default:
				walk(path, sub)
			}
		}
	}
	walk("", raw)
	return out, errs
}

// wholes reads v as an array of n whole numbers.
func wholes(v any, n int) ([]int, bool) {
	a, ok := v.([]any)
	if !ok || len(a) != n {
		return nil, false
	}
	out := make([]int, n)
	for i, x := range a {
		var err error
		if out[i], err = whole(x); err != nil {
			return nil, false
		}
	}
	return out, true
}

// readRoll reads a whole number as a flat percent, or [pct, up, down].
func readRoll(v any) (game.Roll, error) {
	if _, ok := v.(int64); ok {
		n, err := whole(v)
		return game.Roll{Pct: n}, err
	}
	n, ok := wholes(v, 3)
	if !ok {
		return game.Roll{}, fmt.Errorf("want a whole number or [pct, up, down], got %v", v)
	}
	return game.Roll{Pct: n[0], Up: n[1], Down: n[2]}, nil
}

// readFraction reads [num, den].
func readFraction(v any) (game.Fraction, error) {
	n, ok := wholes(v, 2)
	if !ok {
		return game.Fraction{}, fmt.Errorf("want [num, den], got %v", v)
	}
	return game.Fraction{Num: n[0], Den: n[1]}, nil
}

// applyAttacks writes the [attacks] sections' keys into r.
func (v values) applyAttacks(r *game.Rules) error {
	sections, errs := attackSections(v.Attacks)
	for _, s := range sections {
		fx, ok := r.Attacks[s.kind]
		if !ok {
			errs = append(errs, fmt.Errorf("attacks.%s: not an attack a mod can change (%s)", s.kind, strings.Join(kindNames(), ", ")))
			continue
		}
		set := fx.Side(s.side)
		for _, key := range slices.Sorted(maps.Keys(s.keys)) {
			where := fmt.Sprintf("attacks.%s.%s.%s", s.kind, s.side, key)
			f := game.AttackFieldByKey(key)
			if f == nil {
				var known []string
				for _, f := range game.AttackFields {
					known = append(known, f.Key)
				}
				errs = append(errs, fmt.Errorf("%s: unknown key (%s)", where, strings.Join(known, ", ")))
				continue
			}
			var err error
			if f.Roll != nil {
				*f.Roll(set), err = readRoll(s.keys[key])
			} else {
				*f.Fraction(set), err = readFraction(s.keys[key])
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", where, err))
			}
		}
		r.Attacks[s.kind] = fx
	}
	return errors.Join(errs...)
}

// resolveAttacks replaces each option reference in the [attacks] tables with
// its chosen value. An option stands for a whole number, so it fits a key that
// takes one flat percent; an array holds plain numbers.
func (m *Mod) resolveAttacks(raw map[string]any, path string, inSide bool) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	out := map[string]any{}
	for key, val := range raw {
		where := path + "." + key
		if inSide {
			got, err := m.optionValue(val, where)
			if err != nil {
				return nil, err
			}
			out[key] = got
			continue
		}
		sub, ok := val.(map[string]any)
		if !ok {
			out[key] = val // reported by attackSections
			continue
		}
		got, err := m.resolveAttacks(sub, where, slices.Contains(game.AttackSides, key))
		if err != nil {
			return nil, err
		}
		out[key] = got
	}
	return out, nil
}

func formatRoll(r game.Roll) string {
	if r.Up == 0 && r.Down == 0 {
		return fmt.Sprint(r.Pct)
	}
	return fmt.Sprintf("[%d, %d, %d]", r.Pct, r.Up, r.Down)
}

func formatFraction(f game.Fraction) string { return fmt.Sprintf("[%d, %d]", f.Num, f.Den) }

func attackValue(f game.AttackField, s *game.EffectSet) string {
	if f.Roll != nil {
		return formatRoll(*f.Roll(s))
	}
	return formatFraction(*f.Fraction(s))
}

// diffAttacks lists the attack figures that differ between a and b.
func diffAttacks(a, b *game.Rules) []string {
	var out []string
	for _, kind := range game.EffectKinds() {
		for _, side := range game.AttackSides {
			fa, fb := a.Attacks[kind], b.Attacks[kind]
			sa, sb := *fa.Side(side), *fb.Side(side)
			for _, f := range game.AttackFields {
				if from, to := attackValue(f, &sa), attackValue(f, &sb); from != to {
					out = append(out, fmt.Sprintf("attacks.%s.%s.%s: %s -> %s", kind, side, f.Key, from, to))
				}
			}
		}
	}
	return out
}

// writeAttacks prints a [attacks.<kind>.<side>] section per side as a
// values.toml would set it.
func writeAttacks(b *strings.Builder, r *game.Rules) {
	for _, kind := range game.EffectKinds() {
		for _, side := range game.AttackSides {
			fx := r.Attacks[kind]
			set := *fx.Side(side)
			fmt.Fprintf(b, "\n[attacks.%s.%s]\n", kind, side)
			for _, f := range game.AttackFields {
				fmt.Fprintf(b, "%s = %s\n", f.Key, attackValue(f, &set))
			}
		}
	}
}
