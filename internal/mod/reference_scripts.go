package mod

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// The script half of docs/mod-reference.md: what each entry point is given and
// may return, and the built-in functions. reference_test.go checks each table
// against the code that builds the values a script sees.

// scriptField is one name a script is given: Names may hold several fields
// that share a type and meaning.
type scriptField struct {
	names []string
	typ   string
	desc  string
}

// spoilNames are the unit holdings a pirate view carries, from the spoil
// table: "troopers", "jets", and so on. Gold is its own field.
func spoilNames() []string {
	var out []string
	for _, s := range game.PirateSpoils() {
		if s.Good() != nil {
			out = append(out, spoilName(s))
		}
	}
	return out
}

// regionIDList is the region type IDs, in RegionTypes order.

// regionFieldKeys is every key of a region row.
func regionFieldKeys() []string {
	var out []string
	for _, f := range game.RegionFields {
		out = append(out, f.Key)
	}
	return out
}

func pirateRealmFields() []scriptField {
	return []scriptField{
		{[]string{"name"}, "text", "The realm's name."},
		{[]string{"regions"}, "number", "How many regions the realm has."},
		{[]string{"gold"}, "number", "The realm's gold in hand."},
		{spoilNames(), "number", "How many of each unit the realm holds."},
		{[]string{"pref"}, "function", "`realm.pref(id)` is `True` or `False`: the player's answer to a preference the mod declares, or its default when they have made none. An id the mod does not declare is an error."},
		{[]string{"market"}, "struct", "What the realm has for sale on the Trading Market, as a struct with the same unit names: `realm.market.jets`."},
	}
}

func pirateFactionFields() []scriptField {
	return []scriptField{
		{[]string{"slot"}, "number", "The faction's number, starting at 0. A raid names a faction by it."},
		{[]string{"name"}, "text", "The faction's name."},
		{[]string{"regions"}, "number", "How many regions the faction has."},
		{[]string{"gold"}, "number", "The faction's gold."},
		{spoilNames(), "number", "How many of each unit the faction holds."},
	}
}

func regionRealmFields() []scriptField {
	return []scriptField{
		{[]string{"name"}, "text", "The realm's name."},
		{[]string{"day"}, "number", "The game day."},
		{[]string{"land"}, "number", "The realm's total land."},
		{[]string{"regions"}, "struct", "How many regions of each type the realm has, by the type's ID: " + "`realm.regions.mountain`. The IDs are " + codeList(regionIDs()) + "."},
		{[]string{"people"}, "number", "The realm's population."},
		{[]string{"support"}, "number", "The realm's popular support."},
		{[]string{"tax"}, "number", "The realm's tax rate, in percent."},
		{[]string{"turns_played"}, "number", "How many turns the realm has played."},
		{[]string{"pref"}, "function", "`realm.pref(id)` is `True` or `False`: the player's answer to a preference the mod declares, or its default when they have made none. An id the mod does not declare is an error."},
	}
}

func regionRowFields() []scriptField {
	return []scriptField{
		{regionFieldKeys(), "number", "The type's figures as every realm plays them: the base game's with `values.toml` applied. Each is described under [Regions](#regions)."},
	}
}

// builtin is a name every script of a kind may call.
type builtin struct{ sig, desc, kinds string }

var builtins = []builtin{
	{"rand(n)", "A whole number from 0 to n - 1, drawn from the game's own random numbers. `n` must be at least 1.", "pirates"},
	{"day()", "The game day.", "pirates"},
	{"option(id)", "The value the sysop chose for an option the mod declares. A number is a whole number; a bool is `True` or `False`.", "pirates and regions"},
}

func writeFieldTable(b *strings.Builder, fields []scriptField) {
	b.WriteString("| Field | Type | Meaning |\n| --- | --- | --- |\n")
	for _, f := range fields {
		fmt.Fprintf(b, "| %s | %s | %s |\n", codeList(f.names), f.typ, f.desc)
	}
	b.WriteString("\n")
}

// writeScripts writes the page's Scripts section.
func writeScripts(b *strings.Builder) {
	b.WriteString("## Scripts\n\nA script is a [Starlark](https://github.com/bazelbuild/starlark) file. The rules every script follows are under [Scripts](mods.md#scripts). Each entry point is called with the arguments below and must return the value described.\n\n")

	b.WriteString("### Pirate script\n\n`pirates.star` defines `raid(realm, factions)`. It is called once per realm per turn, after the usual checks. `realm` is a read-only struct:\n\n")
	writeFieldTable(b, pirateRealmFields())
	b.WriteString("`factions` is a frozen list with one struct per pirate faction, in slot order:\n\n")
	writeFieldTable(b, pirateFactionFields())
	fmt.Fprintf(b, "`raid` returns a list of at most %d raids, or `[]` for none. Each raid is a dict:\n\n", game.ModPirateOrdersMax)
	var spoils []string
	for _, s := range game.PirateSpoils() {
		spoils = append(spoils, spoilName(s))
	}
	b.WriteString("| Key | Type | Meaning |\n| --- | --- | --- |\n")
	b.WriteString("| `faction` | number | The `slot` of the faction that raids. Required. |\n")
	b.WriteString("| `take` | text | What the raid takes: one of " + codeList(spoils) + ". Required. |\n")
	b.WriteString("| `market` | bool | `True` takes from the realm's Trading Market listing instead of what it holds. Optional. Gold is not sold there, so `gold` cannot be taken that way. |\n")
	fmt.Fprintf(b, "| `share` | number | Percent of a normal raid's take the raid carries off, 0 to %d. 0 takes nothing. Left out, the raid takes a normal share. Optional. |\n", game.ModPirateShareMax)
	fmt.Fprintf(b, "| `land` | number | Regions the raid grants its faction, 0 to %s. Held to `cap_regions` in `[pirates]`. A raid that names it grants it even when it took nothing. Left out, a raid that took something grants the usual random amount (`raid_land_max`) and one that took nothing grants none. Optional. |\n\n", numfmt.Comma(game.ModPirateRaidLandMax))

	b.WriteString("### Region script\n\n`regions/<type>.star` defines `region(realm, row)`. It is called for one realm at a time, whenever the game needs the figures of the region type the file is named for. `realm` is a read-only struct:\n\n")
	writeFieldTable(b, regionRealmFields())
	b.WriteString("`row` is a read-only struct of the type's figures:\n\n")
	writeFieldTable(b, regionRowFields())
	b.WriteString("`region` returns a dict of the figures it changes, with the keys of `row`, or `{}` to change nothing. A key it leaves out keeps its value in `row`. Every figure must stay inside its limit above, and a script for waste may change only the keys waste may change.\n\n")

	b.WriteString("### Built-in functions\n\nThese are added to Starlark's own. A region script has no `rand()` or `day()`: it must give the same answer every time it is asked the same question.\n\n")
	b.WriteString("| Function | Meaning | Available to |\n| --- | --- | --- |\n")
	for _, f := range builtins {
		fmt.Fprintf(b, "| `%s` | %s | %s |\n", f.sig, f.desc, f.kinds)
	}
	b.WriteString("\n")
}
