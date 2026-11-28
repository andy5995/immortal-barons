package mod

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Option types a mod may declare. A bool is written true or false in mod.toml
// and stored as 0 or 1, a number as the whole value the sysop chose (within
// min..max).
const (
	OptionBool   = "bool"
	OptionNumber = "number"
)

// Option is one [options.<id>] table in mod.toml. The pointers tell a missing
// default or bound from a written zero.
type Option struct {
	Type        string `toml:"type"`
	Label       string `toml:"label"`
	Description string `toml:"description"`
	Default     any    `toml:"default"`
	Min         *int64 `toml:"min"`
	Max         *int64 `toml:"max"`
}

// Value is the option's default as a stored value (0 or 1 for a bool), the
// value a missing entry in config.json means. It reads Default itself, so a
// manifest that was decoded without check (ManifestFromRef) gives the same.
func (o *Option) Value() int64 {
	switch d := o.Default.(type) {
	case bool:
		if d {
			return 1
		}
	case int64:
		return d
	}
	return 0
}

// check refuses an option a script or a values file could not use safely.
func (o *Option) check(id, file string) error {
	where := fmt.Sprintf("%s: options.%s", file, id)
	if !validName.MatchString(id) {
		return fmt.Errorf("%s: %q may hold only lowercase letters, digits, - and _", where, id)
	}
	if strings.TrimSpace(o.Label) == "" {
		return fmt.Errorf("%s: label is required", where)
	}
	for key, v := range map[string]string{"label": o.Label, "description": o.Description} {
		if hasUnsafe(v) {
			return fmt.Errorf("%s: %s may not hold control characters", where, key)
		}
	}
	switch o.Type {
	case OptionBool:
		if o.Min != nil || o.Max != nil {
			return fmt.Errorf("%s: min and max are for number options only", where)
		}
		switch d := o.Default.(type) {
		case nil:
			return fmt.Errorf("%s: default is required", where)
		case bool:
		default:
			return fmt.Errorf("%s: default of a bool must be true or false, not %s", where, shownValue(d))
		}
	case OptionNumber:
		if o.Min == nil || o.Max == nil {
			return fmt.Errorf("%s: min and max are required for a number", where)
		}
		if *o.Min > *o.Max {
			return fmt.Errorf("%s: min %d is greater than max %d", where, *o.Min, *o.Max)
		}
		switch d := o.Default.(type) {
		case nil:
			return fmt.Errorf("%s: default is required", where)
		case int64:
			if d < *o.Min || d > *o.Max {
				return fmt.Errorf("%s: default %d is outside %d to %d", where, d, *o.Min, *o.Max)
			}
		default:
			return fmt.Errorf("%s: default of a number must be a whole number, not %s", where, shownValue(d))
		}
	default:
		return fmt.Errorf("%s: type %q is unknown; use %q or %q", where, o.Type, OptionBool, OptionNumber)
	}
	return nil
}

// checkOptions validates every declared option before anything applies one.
func checkOptions(options map[string]*Option, file string) error {
	for _, id := range slices.Sorted(maps.Keys(options)) {
		if err := options[id].check(id, file); err != nil {
			return err
		}
	}
	return nil
}

// chosenValues fills every option's default, then overrides it with the
// recorded or configured value. A missing entry means the default; an entry
// for an option the mod does not declare, or of the wrong type or outside its
// range, is refused rather than silently ignored.
func chosenValues(options map[string]*Option, given map[string]int64, file string) (map[string]int64, error) {
	out := make(map[string]int64, len(options))
	for id, o := range options {
		out[id] = o.Value()
	}
	for _, id := range slices.Sorted(maps.Keys(given)) {
		o, ok := options[id]
		if !ok {
			return nil, fmt.Errorf("%s: option %q does not exist in this mod", file, id)
		}
		v := given[id]
		switch o.Type {
		case OptionBool:
			if v != 0 && v != 1 {
				return nil, fmt.Errorf("%s: option %q is a bool; use 0 (False) or 1 (True), got %d", file, id, v)
			}
		case OptionNumber:
			if v < *o.Min || v > *o.Max {
				return nil, fmt.Errorf("%s: option %q must be from %d to %d, got %d", file, id, *o.Min, *o.Max, v)
			}
		}
		out[id] = v
	}
	return out, nil
}

// Chosen is the value the game plays for id: its default when the sysop made
// no choice. Unknown ids answer 0; every caller names an id from the manifest.
func (m *Mod) Chosen(id string) int64 { return m.chosen[id] }

// SortedOptionIDs is the option ids in a stable order, for screens and reports.
func (m *Mod) SortedOptionIDs() []string { return slices.Sorted(maps.Keys(m.Manifest.Options)) }

// OptionLine describes one option as -check-mod prints it: its type, default
// and, for a number, its range.
func (m *Mod) OptionLine(id string) string {
	o := m.Manifest.Options[id]
	if o.Type == OptionNumber {
		return fmt.Sprintf("%s: number, default %d, from %d to %d", o.Label, o.Value(), *o.Min, *o.Max)
	}
	return fmt.Sprintf("%s: bool, default %s", o.Label, boolWord(o.Value()))
}

// OptionDiff lists the options whose chosen value differs from the mod's
// default, as "label: default -> value". It is empty for an untouched mod.
func (m *Mod) OptionDiff() []string {
	var out []string
	for _, id := range m.SortedOptionIDs() {
		o := m.Manifest.Options[id]
		if got := m.Chosen(id); got != o.Value() {
			out = append(out, fmt.Sprintf("%s: %s -> %s", o.Label, OptionValueWord(o, o.Value()), OptionValueWord(o, got)))
		}
	}
	return out
}

func boolWord(v int64) string {
	if v != 0 {
		return "True"
	}
	return "False"
}

// OptionValueWord renders a chosen value the way OptionDiff and the editors do.
func OptionValueWord(o *Option, v int64) string {
	if o.Type == OptionBool {
		return boolWord(v)
	}
	return fmt.Sprint(v)
}

// OptionValueLine describes one option with its chosen value, for editor rows.
func OptionValueLine(o *Option, v int64) string {
	return fmt.Sprintf("%s: %s", o.Label, OptionValueWord(o, v))
}

// optionValues is what a script sees: each option's type and the value chosen
// for this game. A script's top level runs at load, so both are fixed there
// rather than read from config.json on every call.
type optionValues struct {
	defs   map[string]*Option
	chosen map[string]int64
}

// scriptOptions is the script-facing form of a read mod's chosen options.
func (m *Mod) scriptOptions() *optionValues {
	return &optionValues{defs: m.Manifest.Options, chosen: m.chosen}
}

// builtin is the option() function predeclared in both kinds of script.
func (v *optionValues) builtin() starlark.Value {
	return starlark.NewBuiltin("option", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id string
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &id); err != nil {
			return nil, err
		}
		o, ok := v.defs[id]
		if !ok {
			return nil, fmt.Errorf("option %q does not exist in this mod", id)
		}
		n := v.chosen[id]
		if o.Type == OptionBool {
			return starlark.Bool(n != 0), nil
		}
		return starlark.MakeInt64(n), nil
	})
}

// key adds the chosen option values to a compiled script's cache key. Two
// games playing the same mod with different options must not share a top-level
// computation.
func (v *optionValues) key() string {
	var b strings.Builder
	for _, id := range slices.Sorted(maps.Keys(v.defs)) {
		fmt.Fprintf(&b, "\x00%s:%s=%d", id, v.defs[id].Type, v.chosen[id])
	}
	return b.String()
}

// optionValue resolves a values.toml inline table, { option = "id" }, to the
// chosen value. A plain figure is returned untouched. The chosen value is a
// game.Rules limit check's business, not this reader's: it goes through
// Validate like any figure written directly in the file.
func (m *Mod) optionValue(v any, where string) (any, error) {
	table, ok := v.(map[string]any)
	if !ok {
		return v, nil
	}
	if len(table) != 1 {
		return nil, fmt.Errorf("%s: an option reference holds only option = \"id\"", where)
	}
	id, ok := table["option"].(string)
	if !ok {
		return nil, fmt.Errorf("%s: option must be the option's id", where)
	}
	o, ok := m.Manifest.Options[id]
	if !ok {
		return nil, fmt.Errorf("%s: option %q does not exist in this mod", where, id)
	}
	if o.Type != OptionNumber {
		return nil, fmt.Errorf("%s: option %q is a bool, not a number", where, id)
	}
	return m.Chosen(id), nil
}

// resolveValues replaces every option reference in a section with its chosen
// value. A section that a bool option switches off is still resolved, so a
// broken reference in it is reported even when it does not apply.
func (m *Mod) resolveValues(v values) (values, error) {
	out := values{}
	if v.Units != nil {
		out.Units = map[string]map[string]any{}
		for unit, fields := range v.Units {
			out.Units[unit] = map[string]any{}
			for key, raw := range fields {
				got, err := m.optionValue(raw, "units."+unit+"."+key)
				if err != nil {
					return values{}, err
				}
				out.Units[unit][key] = got
			}
		}
	}
	if v.HQ != nil {
		out.HQ = map[string]any{}
		for key, raw := range v.HQ {
			got, err := m.optionValue(raw, "hq."+key)
			if err != nil {
				return values{}, err
			}
			out.HQ[key] = got
		}
	}
	if v.Combat != nil {
		out.Combat = map[string]any{}
		for key, raw := range v.Combat {
			got, err := m.optionValue(raw, "combat."+key)
			if err != nil {
				return values{}, err
			}
			out.Combat[key] = got
		}
	}
	if v.Pirates != nil {
		out.Pirates = map[string]any{}
		for key, raw := range v.Pirates {
			got, err := m.optionValue(raw, "pirates."+key)
			if err != nil {
				return values{}, err
			}
			out.Pirates[key] = got
		}
	}
	if v.Regions != nil {
		out.Regions = map[string]map[string]any{}
		for region, fields := range v.Regions {
			out.Regions[region] = map[string]any{}
			for key, raw := range fields {
				got, err := m.optionValue(raw, "regions."+region+"."+key)
				if err != nil {
					return values{}, err
				}
				out.Regions[region][key] = got
			}
		}
	}
	if v.Attacks != nil {
		var err error
		if out.Attacks, err = m.resolveAttacks(v.Attacks, "attacks", false); err != nil {
			return values{}, err
		}
	}
	return out, nil
}

// applyWhen writes into r the [when.<id>] sections whose bool option is True,
// after the mod's plain values. The check on the option's type and the section's
// references happens even for a section that does not apply, so a misspelled
// id is not hidden by the option's current value.
func (m *Mod) applyWhen(r *game.Rules) []error {
	var errs []error
	for _, id := range slices.Sorted(maps.Keys(m.values.When)) {
		o, ok := m.Manifest.Options[id]
		if !ok {
			errs = append(errs, fmt.Errorf("when.%s: option %q does not exist in this mod", id, id))
			continue
		}
		if o.Type != OptionBool {
			errs = append(errs, fmt.Errorf("when.%s: only a bool option selects a section", id))
			continue
		}
		section, err := m.resolveValues(m.values.When[id])
		if err != nil {
			errs = append(errs, fmt.Errorf("when.%s: %w", id, err))
			continue
		}
		if m.Chosen(id) == 0 {
			// Nothing in a section switched off reaches the game this time, but
			// its errors are reported as if it were on, so turning the option on
			// later cannot be what first refuses the mod.
			errs = append(errs, offSectionErrors(id, section, r, m)...)
			continue
		}
		errs = append(errs, section.apply(r, m)...)
	}
	return errs
}

// apply writes one already-resolved section's figures into r. Scripts belong
// to the mod itself, never to a when section.
func (v values) apply(r *game.Rules, m *Mod) []error {
	var errs []error
	ids := slices.Sorted(maps.Keys(v.Units))
	for _, id := range ids {
		g := moddableUnit(id)
		if g == nil {
			var known []string
			for _, g := range game.MilitaryGoods {
				known = append(known, g.ID)
			}
			errs = append(errs, fmt.Errorf("units.%s: not a unit a mod can change (%s)", id, strings.Join(known, ", ")))
			continue
		}
		u := r.Units[g]
		for _, key := range slices.Sorted(maps.Keys(v.Units[id])) {
			f := unitField(key)
			if f == nil {
				errs = append(errs, fmt.Errorf("units.%s.%s: unknown key", id, key))
				continue
			}
			if err := f.set(&u, v.Units[id][key]); err != nil {
				errs = append(errs, fmt.Errorf("units.%s.%s: %w", id, key, err))
			}
		}
		r.Units[g] = u
	}
	if err := applyFields("hq", game.HQFields, v.HQ, &r.HQ); err != nil {
		errs = append(errs, err)
	}
	if err := applyFields("combat", game.CombatFields, v.Combat, &r.Combat); err != nil {
		errs = append(errs, err)
	}
	if err := v.applyPirates(r); err != nil {
		errs = append(errs, err)
	}
	if err := v.applyRegions(r); err != nil {
		errs = append(errs, err)
	}
	if err := v.applyAttacks(r); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// offSectionErrors applies a section whose option is off to a copy of r and
// returns what would be refused were it on: its conversion errors, and the
// limits it breaks that r alone does not.
func offSectionErrors(id string, section values, r *game.Rules, m *Mod) []error {
	off := r.Clone()
	if errs := section.apply(off, m); len(errs) > 0 {
		return errs
	}
	already := map[string]bool{}
	for _, e := range joined(r.Validate()) {
		already[e.Error()] = true
	}
	var errs []error
	for _, e := range joined(off.Validate()) {
		if !already[e.Error()] {
			errs = append(errs, fmt.Errorf("when.%s: %w", id, e))
		}
	}
	return errs
}

// joined is the errors err was joined from, err alone when it was not joined,
// or none.
func joined(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	if err == nil {
		return nil
	}
	return []error{err}
}
