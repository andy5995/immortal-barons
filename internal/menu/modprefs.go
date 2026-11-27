package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/mod"
	"github.com/andy5995/immortal-barons/internal/session"
)

// modPrefRow is one preference of a mod in play with the key it is listed under.
type modPrefRow struct {
	key rune
	mod.PrefRow
}

// modPrefRows are the preferences of the mods this world plays. A realm that is
// gone (abdicated) has none to set. Called with the world lock held.
func modPrefRows(g *ctx) []modPrefRow {
	if g.Player() == nil || len(g.World.Mods) == 0 {
		return nil
	}
	var out []modPrefRow
	for i, r := range mod.PrefRows(g.World.Mods) {
		if i >= len(mod.PrefMenuKeys) {
			break
		}
		out = append(out, modPrefRow{key: rune(mod.PrefMenuKeys[i]), PrefRow: r})
	}
	return out
}

// modPrefItems adds the mods' preferences to the Preferences menu, after the
// built-in toggles. Their labels are the mod's own text, so they are shown as
// written; only Yes and No are translated. They share the built-in rows' value
// column, which widens to the longest label of either.
func modPrefItems(g *ctx, group []string) []Item {
	var items []Item
	for _, r := range modPrefRows(g) {
		items = append(items, Item{
			Key: r.key,
			LabelFn: onOffRow(group, modPrefLabels, r.Label, false, func(g *ctx) *bool {
				v := g.Player().ModPref(r.Mod, r.ID, r.Default)
				return &v
			}),
			Do: func(_ session.Session, g *ctx) Result {
				g.With(func() {
					e := g.Player()
					e.SetModPref(r.Mod, r.ID, !e.ModPref(r.Mod, r.ID, r.Default))
				})
				return Stay
			},
		})
	}
	return items
}

// modPrefLabels is every mod preference label showing, for the shared width.
func modPrefLabels(g *ctx) []string {
	var out []string
	for _, r := range modPrefRows(g) {
		out = append(out, r.Label)
	}
	return out
}

// modPrefNotice lists each described preference under the menu, wrapped to the
// box.
func modPrefNotice(g *ctx, width int) string {
	var b strings.Builder
	for _, r := range modPrefRows(g) {
		if r.Description == "" {
			continue
		}
		first := fmt.Sprintf("(%c) ", r.key)
		desc := breakLongWords(r.Description, width-utf8.RuneCountInString(first))
		fmt.Fprintf(&b, "%s%s%s\n", ansi.FgWhite,
			wrapHangingTo(desc, first, strings.Repeat(" ", utf8.RuneCountInString(first)), width), ansi.Reset)
	}
	return b.String()
}

// breakLongWords splits any run of non-space characters longer than n, which
// the wrapper could only break at a space and would print past the screen.
func breakLongWords(text string, n int) string {
	if n < 1 {
		return text
	}
	words := strings.Fields(text)
	for i, w := range words {
		r := []rune(w)
		if len(r) <= n {
			continue
		}
		var parts []string
		for len(r) > n {
			parts, r = append(parts, string(r[:n])), r[n:]
		}
		words[i] = strings.Join(append(parts, string(r)), " ")
	}
	return strings.Join(words, " ")
}
