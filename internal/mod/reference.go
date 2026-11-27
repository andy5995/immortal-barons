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
	{"combat", "The one `[combat]` table.", ""},
	{"regions", "One `[regions.<id>]` table per region type.", ""},
	{"attacks", "One `[attacks.<kind>.target]` and one `[attacks.<kind>.attacker]` table per kind of attack.", ""},
	{"when", "`[when.<option>.units.<id>]`, `[when.<option>.hq]`, `[when.<option>.combat]`, `[when.<option>.regions.<id>]` and `[when.<option>.attacks.<kind>.<side>]`: the same tables, applied only when the bool option `<option>` is true. Read after the plain values, so they override them.", ""},
}

func kindNames() []string {
	var out []string
	for _, k := range game.EffectKinds() {
		out = append(out, string(k))
	}
	return out
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

	b.WriteString("### Combat\n\nThe `[combat]` table redefines what the sysop's Attack Rewards and Attack Damage levels mean, and what a winning same-planet attack pays. The sysop still chooses the level. Every key is a whole number. `capture_*` and `ip_capture_*` are percents of the defender's regions by Attack Rewards level, for a same-planet and an interplanetary attack. An interplanetary share is then scaled by the attack's type and, for an individual strike, doubled. `retreat_*` are percents of a side's own force it loses before it breaks off a same-planet battle, by Attack Damage level; an interplanetary battle's loss stays the attack type's own rate rescaled by the level.\n\n")
	rows = nil
	for _, f := range game.CombatFields {
		rows = append(rows, docKey{f.Key, f.Desc, numfmt.Comma(int(f.Min)) + " to " + numfmt.Comma(int(f.Max))})
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

	b.WriteString("### Attacks\n\nThe kinds of attack are " + codeList(kindNames()) + ". Each kind has two tables, except the missile kinds below. `[attacks.<kind>.target]` is what the attack does to the realm it hits. `[attacks.<kind>.attacker]` is what it does back to the realm that launched it, and is empty in the base game. The target's effects are applied first. Each table takes these keys, in the order they are applied; list only what changes. The two `conventional` kinds do not replace the same-planet attack's battle, its capture or its losses, whose figures are set in `[combat]`: they add to it, `conventional.won` when the attacker wins and `conventional.lost` when it is repelled, and are empty in the base game. The `ip.attack.won` and `ip.attack.lost` kinds add to an interplanetary attack of any type, Group Attack included, that fought a battle; the resolver's battle, capture and losses are not set here; `[combat]` changes the capture share. The `target` table runs on the defending board after the battle, and a `capture` there is carried home with the battle's own. The `attacker` table runs on the attacker's board when the result comes home, on each realm of that board that contributed, as if it had attacked alone; it cannot `capture`. The one part of an `ip.attack` `attacker` table that runs on the defending board is its unit capture, since that is where the arriving force is. The `ip.nuclear`, `ip.chemical` and `sabre.*` kinds are missiles that arrive from another board and act on the receiving board, so only their `target` table is used: a non-zero `attacker` table or a `capture` on them is an error.\n\n")
	b.WriteString("A key marked as a percent takes a whole number, a flat percent, or `[pct, up, down]`: the percent drawn as `pct + Random(up) - Random(down)` and held to 0 to 100. A draw is made only when `up` or `down` is above 0. Every `pct`, `up` and `down` is 0 to " + fmt.Sprint(game.ModAttackPctMax) + ". `morale` and `support` take `[num, den]`, with `den` 1 to " + numfmt.Comma(game.ModAttackDenMax) + " and `num` 0 to `den`. In the attacker's table, `capture` gives the target that share of the attacker's regions. The `capture_<unit>` and `capture_random` keys give the other side that share of the military units still alive after the battle's kills, and only the four battle kinds (`conventional.won`, `conventional.lost`, `ip.attack.won`, `ip.attack.lost`) may set them or `capture_from_stock`. An attack sends no turrets or carriers, so an `attacker` table cannot capture them. On an interplanetary win the captured units travel home with the result and are shared among the contributors like the regions.\n\n")
	rows = nil
	for _, f := range game.AttackFields {
		limits := "percent"
		switch {
		case f.Fraction != nil:
			limits = "`[num, den]`"
		case f.Whole != nil:
			limits = "0 or 1"
		case f.Points:
			limits = "points"
		}
		rows = append(rows, docKey{f.Key, f.Desc, limits})
	}
	writeKeyTable(&b, "Key", "Form", rows)

	writeScripts(&b)
	return b.String()
}
