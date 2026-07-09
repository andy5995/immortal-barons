package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// allianceStrength shows the player's combined offense and defense with their
// Full Defense Alliance partners.
func allianceStrength(s session.Session, w *ctx) Result {
	p := w.Player()
	var off, def int
	var allies []string
	w.With(func() { off, def, allies = w.AllianceStrength(p) })
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Alliance Strength"), ansi.Reset)
	if len(allies) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "You have no defense allies."))
	} else {
		fmt.Fprintf(s, "  "+tr(s, "Allies: %s")+"\n", strings.Join(allies, ", "))
	}
	fmt.Fprintf(s, "  "+tr(s, "Combined offense: %d")+"\n  "+tr(s, "Combined defense: %d")+"\n", off, def)
	pause(s)
	return Stay
}

// negotiateTreaty returns a Diplomacy menu action for one BRE treaty type
// (#68): pick a target empire, then propose it, accept a matching offer from
// them, or break it if already held. Treaty types are now direct menu items
// instead of hiding behind a single "Modify Diplomacy" item.
func negotiateTreaty(ttype string) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		type row struct {
			e      *game.Empire
			name   string
			suffix string
		}
		var rows []row
		w.With(func() {
			for _, e := range w.Empires {
				if !e.Alive || e == p {
					continue
				}
				suffix := ""
				if w.World.HasTreaty(p, e, ttype) {
					suffix = " — " + tr(s, "held")
				} else {
					for _, o := range offersFrom(p, e.Name) {
						if o == ttype {
							suffix = " — " + tr(s, "offers this to you")
						}
					}
				}
				rows = append(rows, row{e, e.Name, suffix})
			}
		})
		if len(rows) == 0 {
			ok(s, "There is no one to negotiate with.")
			return Stay
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
		for i, r := range rows {
			fmt.Fprintf(s, "  %d) %s%s\n", i+1, r.name, r.suffix)
		}
		i := promptInt(s, "Negotiate with which empire (0 to cancel)?")
		if i < 1 || i > len(rows) {
			return Stay
		}
		negotiateWithType(s, w, p, rows[i-1].e, ttype)
		return Stay
	}
}

// negotiateWithType performs the one action that applies to ttype between p
// and e: propose it if neither side has it, accept it if e has already
// offered it, or break it (with confirmation) if p already holds it.
func negotiateWithType(s session.Session, w *ctx, p, e *game.Empire, ttype string) {
	var held, offered bool
	w.With(func() {
		held = w.World.HasTreaty(p, e, ttype)
		if !held {
			for _, o := range offersFrom(p, e.Name) {
				if o == ttype {
					offered = true
				}
			}
		}
	})
	switch {
	case held:
		fmt.Fprintf(s, "\n%s"+tr(s, "You hold a %s with %s.")+"%s\n", ansi.FgBrightCyan, ttype, e.Name, ansi.Reset)
		if !askYesNo(s, "Break this treaty?", false) {
			return
		}
		w.With(func() { w.World.BreakTreaty(p, e, ttype) })
		ok(s, "You broke the %s with %s.", ttype, e.Name)
	case offered:
		fmt.Fprintf(s, "\n%s"+tr(s, "%s offers you a %s.")+"%s\n", ansi.FgBrightCyan, e.Name, ttype, ansi.Reset)
		if !askYesNo(s, "Accept this treaty?", false) {
			return
		}
		w.With(func() { w.World.AcceptTreaty(p, e.Name, ttype) })
		ok(s, "You accepted the %s with %s.", ttype, e.Name)
	default:
		w.With(func() { w.World.ProposeTreaty(p, e, ttype) })
		ok(s, "Proposed a %s to %s.", ttype, e.Name)
	}
}

// declareWar is BRE's Declaration Of War: pick a target and, on confirmation,
// break every treaty currently held with them in one action. Per
// docs/mechanics-reference.md this is meant to skip the internal unrest a
// normal treaty break causes — but IB does not yet model unrest on treaty
// breaks at all, so there is no behavioral difference from breaking each
// treaty individually today; this is a placeholder for when that lands.
func declareWar(s session.Session, w *ctx) Result {
	p := w.Player()
	type row struct {
		e    *game.Empire
		name string
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive || e == p {
				continue
			}
			rows = append(rows, row{e, e.Name})
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to declare war on.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %s\n", i+1, r.name)
	}
	i := promptInt(s, "Declare war on which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	if !askYesNo(s, "Declare war? This breaks all treaties with them.", false) {
		return Stay
	}
	var broke []string
	w.With(func() {
		for _, tt := range w.World.TreatiesBetween(p, target) {
			w.World.BreakTreaty(p, target, tt)
			broke = append(broke, tt)
		}
	})
	if len(broke) == 0 {
		ok(s, "You declared war on %s.", target.Name)
	} else {
		ok(s, "You declared war on %s. Treaties broken: %s", target.Name, strings.Join(broke, ", "))
	}
	return Stay
}

// offersFrom returns the treaty types `from` has offered to p.
func offersFrom(p *game.Empire, from string) []string {
	var out []string
	for _, o := range p.TreatyOffers {
		if o.From == from {
			out = append(out, o.Type)
		}
	}
	return out
}

// pickFromList shows a numbered list and returns the chosen entry, or "".
func pickFromList(s session.Session, msg string, list []string) string {
	for i, x := range list {
		fmt.Fprintf(s, "    %d) %s\n", i+1, x)
	}
	i := promptInt(s, msg+" (0 to cancel)?")
	if i < 1 || i > len(list) {
		return ""
	}
	return list[i-1]
}

func viewDiplomacy(s session.Session, w *ctx) Result {
	p := w.Player()
	type row struct{ name, treaties string }
	var rows []row
	var offers []game.TreatyOffer
	w.With(func() {
		for _, e := range w.Empires {
			if e == p || !e.Alive {
				continue
			}
			if held := w.TreatiesBetween(p, e); len(held) > 0 {
				rows = append(rows, row{e.Name, strings.Join(held, ", ")})
			}
		}
		offers = append([]game.TreatyOffer(nil), p.TreatyOffers...)
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Your treaties:"), ansi.Reset)
	if len(rows) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "(none)"))
	} else {
		for _, r := range rows {
			fmt.Fprintf(s, "  %s: %s\n", r.name, r.treaties)
		}
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Pending offers received:"), ansi.Reset)
	if len(offers) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "(none)"))
	} else {
		for _, o := range offers {
			fmt.Fprintf(s, "  %s — %s\n", o.From, o.Type)
		}
	}
	pause(s)
	return Stay
}
