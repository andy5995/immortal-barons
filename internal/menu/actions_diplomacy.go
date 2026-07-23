package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// reviewTreatyOffers surfaces each pending treaty offer to the player at turn
// start, BRE-style: the proposer's Regions / Net Worth / Score inline with an
// Accept? (Y/n) prompt, instead of making the player hunt for it in the Diplomacy
// menu. Accepting forms the treaty; declining removes the offer so it does not
// re-prompt. Wording matches the original (driven live); colors are IB house
// style, since capture-pane cannot read BRE's.
func reviewTreatyOffers(s session.Session, w *ctx) {
	var offers []game.TreatyOffer
	withPlayer(w, func(p *game.Empire) {
		offers = append([]game.TreatyOffer(nil), p.TreatyOffers...)
	})
	for _, o := range offers {
		var regions, netWorth, score int
		var gone bool
		w.With(func() {
			from := findRealm(w, o.From)
			if from == nil {
				gone = true
				return
			}
			regions, netWorth, score = from.Land, w.NetWorth(from), from.Score
		})
		if gone {
			// Proposer eliminated before we got here: drop the stale offer silently.
			withPlayer(w, func(p *game.Empire) { w.World.DeclineTreaty(p, o.From, o.Type) })
			continue
		}
		fmt.Fprintf(s, "\n%s"+tr(s, "%s proposes a %s.")+"%s\n",
			ansi.FgBrightCyan, o.From, tr(s, o.Type), ansi.Reset)
		if o.Message != "" {
			fmt.Fprintf(s, "  %s\"%s\"%s\n", ansi.Dim, o.Message, ansi.Reset)
		}
		// No trailing newline: AskYesNo begins on its own line, so this avoids a
		// blank gap between the stats and the "Accept? (Y/n)" prompt.
		fmt.Fprintf(s, "  "+tr(s, "Regions: %s; Net Worth: %s; Score: %s"),
			comma(regions), comma(netWorth), comma(score))
		if AskYesNo(s, "Accept?", true) {
			withPlayer(w, func(p *game.Empire) { w.World.AcceptTreaty(p, o.From, o.Type) })
		} else {
			withPlayer(w, func(p *game.Empire) { w.World.DeclineTreaty(p, o.From, o.Type) })
		}
	}
}

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
	fmt.Fprintf(s, "  %s\n  %s\n", hiNums(fmt.Sprintf(tr(s, "Combined offense: %d"), off)), hiNums(fmt.Sprintf(tr(s, "Combined defense: %d"), def)))
	pause(s)
	return Stay
}

// treatyDescriptions explains, in plain English, what each pact does in IB —
// shown when the player opens that treaty type's negotiation (BRE shows a pact
// blurb before the send-to list). Wording is IB's own (not copied from BRE) and
// describes IB's actual mechanics (see internal/game/diplomacy.go).
var treatyDescriptions = map[string]string{
	"Full Defense Alliance":  "Neither realm may attack the other, and your armies stand together — your combined offense and defense count in every battle.",
	"Tariff Trade Agreement": "Opens a taxed trade route. Both realms earn a modest income each turn, scaled to population.",
	"Free Trade Agreement":   "Opens an open trade route. Both realms earn a larger income each turn — about double a tariff — scaled to population.",
	"Protective Trade":       "Shields both realms' trade routes and markets from covert sabotage.",
	"Terrorist Prevention":   "Pools covert agents for defense, making both realms harder to spy on and sabotage.",
	"Intelligence Alliance":  "Shares intelligence — partner agents strengthen your covert operations, both attacking and defending.",
	"Technology Agreement":   "Shares technology — the partner with less advanced tech is pulled up toward the more advanced one.",
}

// negotiateTreaty returns a Diplomacy menu action for one BRE treaty type
// (#68): pick a target empire, then propose it, accept a matching offer from
// them, or break it if already held. Treaty types are now direct menu items
// instead of hiding behind a single "Modify Diplomacy" item.
func negotiateTreaty(ttype string) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		type row struct {
			name      string
			relations string // treaty types held with this empire (BRE's Relations column), or "None"
			suffix    string
		}
		var rows []row
		w.With(func() {
			for _, e := range w.Empires {
				if !e.Alive || e == p {
					continue
				}
				relations := tr(s, "None")
				if held := w.World.TreatiesBetween(p, e); len(held) > 0 {
					named := make([]string, len(held))
					for i, t := range held {
						named[i] = tr(s, t)
					}
					relations = strings.Join(named, ", ")
				}
				suffix := ""
				for _, o := range offersFrom(p, e.Name) {
					if o == ttype {
						suffix = "  " + tr(s, "(offers this to you)")
					}
				}
				rows = append(rows, row{e.Name, relations, suffix})
			}
		})
		if len(rows) == 0 {
			ok(s, "There is no one to negotiate with.")
			return Stay
		}
		// Show what this pact does, so the player sees its effect before choosing a
		// partner (BRE shows a blurb here).
		if desc := treatyDescriptions[ttype]; desc != "" {
			fmt.Fprintf(s, "\n%s%s%s\n%s  %s%s\n",
				ansi.FgBrightYellow, tr(s, ttype), ansi.Reset, ansi.Dim, tr(s, desc), ansi.Reset)
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
		// BRE's send-to list shows a Relations column (your standing with each
		// empire) so you can see who you already have pacts with before choosing.
		for i, r := range rows {
			fmt.Fprintf(s, "  %d) %-22s %s%s%s%s\n",
				i+1, r.name, ansi.Dim, tr(s, "Relations: ")+r.relations, ansi.Reset, r.suffix)
		}
		i := promptInt(s, "Negotiate with which empire (0 to cancel)?")
		if i < 1 || i > len(rows) {
			return Stay
		}
		negotiateWithType(s, w, rows[i-1].name, ttype)
		return Stay
	}
}

// negotiateWithType performs the one action that applies to ttype between the
// player and the empire named ename: propose it if neither side has it, accept
// it if ename has already offered it, or break it (with confirmation) if the
// player already holds it. Both empires are re-resolved by name inside every
// mutating transaction, so a concurrent node that eliminates ename (or the
// player) between the prompt and the write aborts cleanly instead of mutating a
// stale/rebound pointer. AcceptTreaty is idempotent — a second accept of an
// already-consumed offer forms no duplicate treaty.
func negotiateWithType(s session.Session, w *ctx, ename, ttype string) {
	var held, offered, gone bool
	w.With(func() {
		p := w.Player()
		e := findRealm(w, ename)
		if p == nil || e == nil {
			gone = true
			return
		}
		held = w.World.HasTreaty(p, e, ttype)
		if !held {
			for _, o := range offersFrom(p, e.Name) {
				if o == ttype {
					offered = true
				}
			}
		}
	})
	if gone {
		fail(s, errTargetGone)
		return
	}
	switch {
	case held:
		fmt.Fprintf(s, "\n%s"+tr(s, "You hold a %s with %s.")+"%s\n", ansi.FgBrightCyan, ttype, ename, ansi.Reset)
		if !AskYesNo(s, "Break this treaty?", false) {
			return
		}
		var err error
		w.With(func() {
			p := w.Player()
			e := findRealm(w, ename)
			if p == nil || e == nil {
				err = errTargetGone
				return
			}
			w.World.BreakTreaty(p, e, ttype)
		})
		if err != nil {
			fail(s, err)
			return
		}
		ok(s, "You broke the %s with %s.", ttype, ename)
	case offered:
		fmt.Fprintf(s, "\n%s"+tr(s, "%s offers you a %s.")+"%s\n", ansi.FgBrightCyan, ename, ttype, ansi.Reset)
		if !AskYesNo(s, "Accept this treaty?", false) {
			return
		}
		var err error
		w.With(func() {
			p := w.Player()
			e := findRealm(w, ename)
			if p == nil || e == nil {
				err = errTargetGone
				return
			}
			w.World.AcceptTreaty(p, e.Name, ttype)
		})
		if err != nil {
			fail(s, err)
			return
		}
		ok(s, "You accepted the %s with %s.", ttype, ename)
	default:
		// BRE offers to attach a note to the proposal; the recipient sees it with
		// the offer.
		message := ""
		if AskYesNo(s, "Attach a message?", false) {
			message = prompt(s, "Message:")
		}
		var err error
		w.With(func() {
			p := w.Player()
			e := findRealm(w, ename)
			if p == nil || e == nil {
				err = errTargetGone
				return
			}
			w.World.ProposeTreatyWithMessage(p, e, ttype, message)
		})
		if err != nil {
			fail(s, err)
			return
		}
		ok(s, "Proposed a %s to %s.", ttype, ename)
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
	var names []string
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive || e == p {
				continue
			}
			names = append(names, e.Name)
		}
	})
	if len(names) == 0 {
		ok(s, "There is no one to declare war on.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
	for i, n := range names {
		fmt.Fprintf(s, "  %d) %s\n", i+1, n)
	}
	i := promptInt(s, "Declare war on which empire (0 to cancel)?")
	if i < 1 || i > len(names) {
		return Stay
	}
	targetName := names[i-1]
	if !AskYesNo(s, "Declare war? This breaks all treaties with them.", false) {
		return Stay
	}
	var broke []string
	var err error
	w.With(func() {
		p := w.Player()
		target := findRealm(w, targetName)
		if p == nil || target == nil {
			err = errTargetGone
			return
		}
		for _, tt := range w.World.TreatiesBetween(p, target) {
			w.World.BreakTreaty(p, target, tt)
			broke = append(broke, tt)
		}
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	if len(broke) == 0 {
		ok(s, "You declared war on %s.", targetName)
	} else {
		ok(s, "You declared war on %s. Treaties broken: %s", targetName, strings.Join(broke, ", "))
	}
	return Stay
}

// findRealm re-resolves a living empire by realm name against the freshly
// reloaded world. Call it only from inside a w.With block (after the world has
// reloaded). Matching by name, not a cached pointer, is what survives the
// reload: json.Unmarshal reuses *Empire pointers by slice INDEX, so a pre-
// gathered pointer rebinds to a different realm when the empire set shifts.
// Names are unique (RealmNameTaken guards onboarding), so this returns the
// intended realm or nil (eliminated/abdicated).
func findRealm(w *ctx, name string) *game.Empire {
	for _, e := range w.Empires {
		if e.Alive && e.Name == name {
			return e
		}
	}
	return nil
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
