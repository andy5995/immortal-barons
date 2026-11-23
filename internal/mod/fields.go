package mod

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// field is one key a [units.<id>] section may set: how to read it from a
// values file, and how to print it back in the same form. Reading, -check-mod's
// list of changes and -print-base-mod all go through this table, so a key is
// named in one place.
type field struct {
	key string
	get func(u game.UnitRules) string
	set func(u *game.UnitRules, v any) error
}

var unitFields = []field{
	{"build_cost",
		func(u game.UnitRules) string { return fmt.Sprint(u.Cost) },
		func(u *game.UnitRules, v any) (err error) { u.Cost, err = whole(v); return }},
	{"networth",
		func(u game.UnitRules) string { return game.FixedPoint(u.NetWorth, 1000) },
		func(u *game.UnitRules, v any) (err error) { u.NetWorth, err = fixed(v, 1000); return }},
	{"upkeep",
		func(u game.UnitRules) string { return game.FixedPoint(u.UpkeepTenths, 10) },
		func(u *game.UnitRules, v any) (err error) { u.UpkeepTenths, err = fixed(v, 10); return }},
	{"offense",
		func(u game.UnitRules) string { return game.FixedPoint(u.Offense, 100) },
		func(u *game.UnitRules, v any) (err error) { u.Offense, err = fixed(v, 100); return }},
	{"defense",
		func(u game.UnitRules) string { return game.FixedPoint(u.Defense, 100) },
		func(u *game.UnitRules, v any) (err error) { u.Defense, err = fixed(v, 100); return }},
	{"price_low",
		func(u game.UnitRules) string { return fmt.Sprint(u.PriceLo) },
		func(u *game.UnitRules, v any) (err error) { u.PriceLo, err = whole(v); return }},
	{"price_high",
		func(u game.UnitRules) string { return fmt.Sprint(u.PriceHi) },
		func(u *game.UnitRules, v any) (err error) { u.PriceHi, err = whole(v); return }},
	{"price_step",
		func(u game.UnitRules) string { return fmt.Sprint(u.PriceStep) },
		func(u *game.UnitRules, v any) (err error) { u.PriceStep, err = whole(v); return }},
}

func unitField(key string) *field {
	for i := range unitFields {
		if unitFields[i].key == key {
			return &unitFields[i]
		}
	}
	return nil
}

// Diff lists every value that differs between a and b, as
// "tank.build_cost: 500 -> 600", unit by unit in MilitaryGoods order.
func Diff(a, b *game.Rules) []string {
	var out []string
	for _, g := range game.MilitaryGoods {
		for _, f := range unitFields {
			from, to := f.get(a.Units[g]), f.get(b.Units[g])
			if from != to {
				out = append(out, fmt.Sprintf("%s.%s: %s -> %s", g.ID, f.key, from, to))
			}
		}
	}
	out = append(out, diffHQ(a, b)...)
	out = append(out, diffRegions(a, b)...)
	if from, to := brainName(a), brainName(b); from != to {
		out = append(out, fmt.Sprintf("pirate raids: %s -> %s", from, to))
	}
	return out
}

// brainName is how Diff names what decides r's pirate raids.
func brainName(r *game.Rules) string {
	if s, ok := r.Pirates.(*pirateScript); ok {
		return s.mod + "/" + PiratesFile + " (" + s.hash + ")"
	}
	return "built in"
}

// Values prints every value a mod can change, as a values.toml would set it.
// header is written first as comment lines.
func Values(r *game.Rules, header string) string {
	var b strings.Builder
	for _, line := range strings.Split(header, "\n") {
		fmt.Fprintf(&b, "# %s\n", line)
	}
	for _, g := range game.MilitaryGoods {
		fmt.Fprintf(&b, "\n[units.%s]\n", g.ID)
		for _, f := range unitFields {
			fmt.Fprintf(&b, "%s = %s\n", f.key, f.get(r.Units[g]))
		}
	}
	writeHQ(&b, r)
	writeRegions(&b, r)
	return b.String()
}
