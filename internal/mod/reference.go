package mod

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// This file writes docs/mod-reference.md: every key a mod may set and every
// value a script is given. The key tables are the ones the reader uses
// (unitFields, game.HQFields, game.RegionFields), so a key added there appears
// in the page, and TestModReferenceIsCurrent fails until the page is rewritten.
// The tables below it (manifestKeys, optionKeys, the script fields) are the
// parts of the format that live in struct tags and Starlark builders instead;
// reference_test.go checks each against the code that reads or builds it.

// docKey is one row of a keys table.
type docKey struct{ key, desc, limits string }

// manifestKeys are the keys of mod.toml, in the order the page lists them.
var manifestKeys = []docKey{
	{"name", "The mod's name. It must match the folder's name, which may use lowercase letters, digits, `-` and `_`, and must start with a letter or digit. `config.json` lists a mod by it.", "required"},
	{"version", "The mod's own version, for its author to change as they like.", "required"},
	{"mod_api", fmt.Sprintf("The version of the mod format the mod was written for. This game reads `mod_api = %d`; see [Mod format versions](mods.md#mod-format-versions).", ModAPI), "required"},
	{"license", "How others may use and share the mod, as an [SPDX identifier](https://spdx.org/licenses/). `-check-mod` prints it.", "required"},
	{"label", "The name the Game Setup screen shows. Blank shows `name`.", "optional"},
	{"description", "A line saying what the mod does.", "optional"},
	{"options", "The [`[options.<id>]`](#options) tables the sysop may set at a reset.", "optional"},
}

// optionKeys are the fields of an [options.<id>] table.
var optionKeys = []docKey{
	{"type", "`number` or `bool`.", "required"},
	{"label", "The short name shown in the settings editors, `-check-mod` and Game Setup.", "required"},
	{"description", "A line saying what the option changes.", "optional"},
	{"default", "The value used when the sysop makes no choice. A number is a whole number. A bool is `true` or `false`; `0` and `1` are refused.", "required"},
	{"min", "Lowest value a number option may take.", "required for a number, refused for a bool"},
	{"max", "Highest value a number option may take.", "required for a number, refused for a bool"},
}

// valuesSections are the top-level tables of values.toml.
var valuesSections = []docKey{
	{"units", "One `[units.<id>]` table per unit. List only what changes.", ""},
	{"hq", "The one `[hq]` table.", ""},
	{"regions", "One `[regions.<id>]` table per region type.", ""},
	{"when", "`[when.<option>.units.<id>]`, `[when.<option>.hq]` and `[when.<option>.regions.<id>]`: the same tables, applied only when the bool option `<option>` is true. Read after the plain values, so they override them.", ""},
}

func codeList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "`" + s + "`"
	}
	return strings.Join(q, ", ")
}

func writeKeyTable(b *strings.Builder, keyHead, limitHead string, rows []docKey) {
	if limitHead == "" {
		fmt.Fprintf(b, "| %s | Meaning |\n| --- | --- |\n", keyHead)
		for _, r := range rows {
			fmt.Fprintf(b, "| `%s` | %s |\n", r.key, r.desc)
		}
		b.WriteString("\n")
		return
	}
	fmt.Fprintf(b, "| %s | Meaning | %s |\n| --- | --- | --- |\n", keyHead, limitHead)
	for _, r := range rows {
		fmt.Fprintf(b, "| `%s` | %s | %s |\n", r.key, r.desc, r.limits)
	}
	b.WriteString("\n")
}

func hqLimits(f game.HQField) string {
	switch f.Key {
	case "price_cap":
		return "`price_base` to " + numfmt.Comma(int(f.Max))
	case "price_cap_jitter":
		return "0 to one less than `price_cap`"
	}
	return numfmt.Comma(int(f.Min)) + " to " + numfmt.Comma(int(f.Max))
}

// Reference is the text of docs/mod-reference.md.
func Reference() string {
	var b strings.Builder
	b.WriteString(`# Mod Reference

This page is written by the game's code: every key, limit and script value
below is read from the tables the game itself uses. It is for the sysop or
author who writes a mod. [Mods](mods.md) explains how a mod works; this page
lists what it may say.

`)

	b.WriteString("## mod.toml\n\n")
	writeKeyTable(&b, "Key", "Needed", manifestKeys)

	b.WriteString("### Options\n\nEach `[options.<id>]` table takes these fields. See [Options](mods.md#options).\n\n")
	writeKeyTable(&b, "Field", "Needed", optionKeys)

	b.WriteString("## values.toml\n\nThese are the tables a `values.toml` may hold. Any other table or key is an error.\n\n")
	writeKeyTable(&b, "Table", "", valuesSections)

	var ids []string
	for _, g := range game.MilitaryGoods {
		ids = append(ids, g.ID)
	}
	b.WriteString("### Units\n\nThe units are " + codeList(ids) + ". Each `[units.<id>]` table takes these keys.\n\n")
	var rows []docKey
	for _, f := range unitFields {
		rows = append(rows, docKey{f.key, f.desc, f.limits})
	}
	writeKeyTable(&b, "Key", "Limits", rows)

	b.WriteString("### HeadQuarters\n\nThe `[hq]` table takes these keys.\n\n")
	rows = nil
	for _, f := range game.HQFields {
		rows = append(rows, docKey{f.Key, f.Desc, hqLimits(f)})
	}
	writeKeyTable(&b, "Key", "Limits", rows)

	b.WriteString("### Regions\n\nThe region types are " + codeList(regionIDs()) + ". Each `[regions.<id>]` table takes these keys, all whole numbers. Every type takes every key. Waste is ruined land: it may change only the keys marked in the last column, and every other key must stay 0.\n\n")
	b.WriteString("| Key | Meaning | Limits | Waste |\n| --- | --- | --- | --- |\n")
	for _, f := range game.RegionFields {
		waste := "must be 0"
		if f.WasteMay {
			waste = "may change"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s to %s | %s |\n", f.Key, f.Desc, numfmt.Comma(f.Min), numfmt.Comma(f.Max), waste)
	}
	b.WriteString("\n")

	return b.String()
}
