package menu

import (
	"strings"
	"testing"
)

// No menu's box may be narrower than what is inside it, in ANY language.
//
// A declared Width is measured against English by whoever declares it, and a
// translated label is routinely longer than the English it came from — the
// System menu was set to 59 because that is exactly what it measured in English,
// and ran 64 in German and Russian, leaving its box five columns short of its own
// items. Two more menus overran their rule in English alone. The catalogs are
// about a third filled, so most labels here are still English and this gets
// harder as they fill, not easier: that is what makes it worth a test rather
// than a re-measure.
//
// Header and status lines are excluded on purpose. They sit OUTSIDE the rules,
// above and below the box, and the original's do too.
func TestNoMenuIsNarrowerThanItsContents(t *testing.T) {
	menus := func(ms *Menus) map[string]*Menu {
		return map[string]*Menu{
			"Spending": ms.Spending, "Sell": ms.Sell, "Bank": ms.Bank, "Attack": ms.Attack,
			"InterPlanetary": ms.InterPlanetary, "IPSpecial": ms.IPSpecial,
			"TerrorOps": ms.TerrorOps, "Covert": ms.Covert, "Trading": ms.Trading,
			"Diplomacy": ms.Diplomacy, "Messages": ms.Messages, "System": ms.System,
			"Game": ms.Game, "Food": ms.Food,
		}
	}
	for _, lang := range []string{"en", "de", "ru"} {
		for name := range menus(BuildMenus()) {
			t.Run(lang+"/"+name, func(t *testing.T) {
				w := newWorld()
				w.With(func() {
					w.Config.IBBS = true
					p := w.Player()
					p.Language = lang
					// The Covert menu closes itself when the caller has no agents
					// left, so an empty-handed realm never draws it (tree.go,
					// ExitWhen). Give every menu what it needs to be seen.
					p.Agents, p.Gold = 50, 10_000_000
				})
				m := menus(BuildMenus())[name]
				f := &fakeSession{keys: []rune{4}}
				Run(f, w, m)
				lines := strings.Split(stripANSI(f.out.String()), "\n")

				rule, body, drew := 0, 0, false
				for _, ln := range lines {
					ln = strings.TrimRight(ln, " \r")
					n := len([]rune(ln))
					// A rule is the run of box characters that opens and closes it.
					if strings.Count(ln, "─") > 3 {
						drew = true
						if n > rule {
							rule = n
						}
						continue
					}
					// Everything between them, minus the prompt line and the footer,
					// which are not inside the box.
					if strings.HasPrefix(ln, "  ") && n > body {
						body = n
					}
				}
				// The box must have been DRAWN — a menu that rendered nothing would
				// pass every check below while proving nothing.
				if !drew {
					t.Fatalf("%s drew no box at all in %s; the test never reached it", name, lang)
				}
				if body > rule {
					t.Errorf("%s in %s: its widest row is %d columns and its rule is %d — the box is %d short:\n%s",
						name, lang, body, rule, body-rule, stripANSI(f.out.String()))
				}
			})
		}
	}
}
