package mod

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// field is one key a [units.<id>] section may set: how to read it from a
// values file, and how to print it back in the same form. Reading, -check-mod's
// list of changes and -print-base-mod all go through this table, so a key is
// named in one place.
type field struct {
	key string
	// desc says what the figure means and limits states its bounds; the
	// generated mod reference prints both (reference.go).
	desc, limits string
	get          func(u game.UnitRules) string
	set          func(u *game.UnitRules, v any) error
}

var unitFields = []field{
	{"build_cost",
		"Production points an Industrial region spends to build one unit. This is not the purchase price.",
		"1 to " + numfmt.Comma(game.ModUnitCostMax),
		func(u game.UnitRules) string { return fmt.Sprint(u.Cost) },
		func(u *game.UnitRules, v any) (err error) { u.Cost, err = whole(v); return }},
	{"networth",
		"Net worth of one unit, in gold, with up to 3 decimal places.",
		"0 to " + numfmt.Comma(game.ModUnitNetWorthMax/1000),
		func(u game.UnitRules) string { return game.FixedPoint(u.NetWorth, 1000) },
		func(u *game.UnitRules, v any) (err error) { u.NetWorth, err = fixed(v, 1000); return }},
	{"upkeep",
		"Gold to maintain one unit per turn at the Medium Maintenance Costs setting, with up to 1 decimal place. None charges nothing, Low a quarter of it and High four times it.",
		"0 to " + numfmt.Comma(game.ModUnitUpkeepMax/10),
		func(u game.UnitRules) string { return game.FixedPoint(u.UpkeepTenths, 10) },
		func(u *game.UnitRules, v any) (err error) { u.UpkeepTenths, err = fixed(v, 10); return }},
	{"offense",
		"Strength of one unit when it attacks another realm on this planet, in troopers, with up to 2 decimal places. For a tank, its strength with no HeadQuarters.",
		"0 to " + numfmt.Comma(game.ModUnitStrengthMax/100) + "; 0 for `turret` and `carrier`",
		func(u game.UnitRules) string { return game.FixedPoint(u.Offense, 100) },
		func(u *game.UnitRules, v any) (err error) { u.Offense, err = fixed(v, 100); return }},
	{"defense",
		"Strength of one unit when its realm is attacked from this planet, in troopers, with up to 2 decimal places. For a tank, its strength with no HeadQuarters.",
		"0 to " + numfmt.Comma(game.ModUnitStrengthMax/100),
		func(u game.UnitRules) string { return game.FixedPoint(u.Defense, 100) },
		func(u *game.UnitRules, v any) (err error) { u.Defense, err = fixed(v, 100); return }},
	{"hq_bonus",
		"How much the unit gains for each point of HeadQuarters completion, in troopers, with up to 2 decimal places, when its `offense` or `defense` is 3.5. A unit with another strength gains in the same proportion, so a finished HeadQuarters multiplies its strength by `1 + 100 x hq_bonus / 3.5`: 4.5/3.5 at the tank's 0.01, 13.5/3.5 at 0.1. Without mods only tanks have one.",
		"0 to " + game.FixedPoint(game.ModUnitHQBonusMax, 100),
		func(u game.UnitRules) string { return game.FixedPoint(u.HQBonus, 100) },
		func(u *game.UnitRules, v any) (err error) { u.HQBonus, err = fixed(v, 100); return }},
	{"price_low",
		"Bottom of the range a realm's price for the unit moves inside.",
		"1 to " + numfmt.Comma(game.ModUnitPriceMax) + ", at least " + numfmt.Comma(game.ModUnitPriceBandMin) + " below `price_high`",
		func(u game.UnitRules) string { return fmt.Sprint(u.PriceLo) },
		func(u *game.UnitRules, v any) (err error) { u.PriceLo, err = whole(v); return }},
	{"price_high",
		"Top of the range a realm's price for the unit moves inside.",
		"1 to " + numfmt.Comma(game.ModUnitPriceMax) + ", at least " + numfmt.Comma(game.ModUnitPriceBandMin) + " above `price_low`",
		func(u game.UnitRules) string { return fmt.Sprint(u.PriceHi) },
		func(u *game.UnitRules, v any) (err error) { u.PriceHi, err = whole(v); return }},
	{"price_step",
		"How far the price can move in one turn.",
		"1 to the width of the range",
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
	out = append(out, diffAttacks(a, b)...)
	if from, to := brainName(a), brainName(b); from != to {
		out = append(out, fmt.Sprintf("pirate raids: %s -> %s", from, to))
	}
	for _, g := range game.RegionTypes {
		if from, to := regionBrainName(a, g), regionBrainName(b, g); from != to {
			out = append(out, fmt.Sprintf("%s script: %s -> %s", g.ID, from, to))
		}
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

// regionBrainName is how Diff names the script r runs for type g.
func regionBrainName(r *game.Rules, g *game.Region) string {
	if s, ok := r.RegionBrains[g].(*regionScript); ok {
		return s.mod + "/" + regionScriptFile(g) + " (" + s.hash + ")"
	}
	return "none"
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
	writeAttacks(&b, r)
	return b.String()
}
