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
// re-prompt.
//
// Wording, layout and colors are the original's, from a live capture: only the
// realm name and the treaty type are bright cyan (the connecting words stay
// white), the three figures are bright yellow, and the whole stats line runs
// unindented into the prompt — "…; Score: N; Accept? (Y/n)". The figures are
// comma-grouped, which BRE does NOT do here (see readability divergence below).
// BRE files these after the numbered recap entries and before the mail, which is
// where gameflow calls it; unlike a recap entry an offer carries no rule or
// timestamp, since it is a prompt rather than a log line.
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
		name := ansi.FgBrightCyan + o.From + ansi.FgWhite
		pact := ansi.FgBrightCyan + tr(s, o.Type) + ansi.FgWhite
		fmt.Fprintf(s, "\n%s"+tr(s, "%s proposes a %s.")+"%s\n", ansi.FgWhite, name, pact, ansi.Reset)
		if o.Message != "" {
			fmt.Fprintf(s, "  %s\"%s\"%s\n", ansi.Dim, o.Message, ansi.Reset)
		}
		fig := func(n int) string { return ansi.FgBrightYellow + comma(n) + ansi.FgWhite }
		fmt.Fprintf(s, "%s"+tr(s, "Regions: %s │ Net Worth: %s │ Score: %s")+" │ ",
			ansi.FgWhite, fig(regions), fig(netWorth), fig(score))
		if askYesNoHere(s, "Accept?", true) {
			withPlayer(w, func(p *game.Empire) { w.World.AcceptTreaty(p, o.From, o.Type) })
		} else {
			withPlayer(w, func(p *game.Empire) { w.World.DeclineTreaty(p, o.From, o.Type) })
		}
	}
}

// Alliance-Strength geometry. BRE's is a 51-column inset rule over a 21-column
// name field and 9/10/10 number columns (its heading row is one column wider on
// Troopers than its figures, an off-by-one in the original). IB's numbers are
// comma-grouped, so the columns are 12 wide to fit eight digits with a gap, and
// the heading uses the same width as the figures; the rule follows to match.
const (
	allyNameWidth   = 21
	allyColumnWidth = 12
	allyRuleWidth   = allyNameWidth + 3*allyColumnWidth
	allyRuleDouble  = 10
)

// allianceStrength shows what each Full Defense Alliance partner will send to aid
// the player's defense — 30% of its troopers, tanks, and agents (BRE's Alliance
// Strength screen). Layout and colors are the original's: white headings over a
// red rule, ally names bright white, figures bright yellow, a zero shown as
// "NONE", and a Total Forces line under a second rule.
//
// READABILITY DIVERGENCE: BRE prints these figures ungrouped ("963016"); IB
// comma-groups them, here and on the treaty-offer stats line, because a
// seven-digit run is hard to read at a glance. The columns are widened to fit.
// BRE itself groups elsewhere (the Queen Royale's refund line), so this is not a
// house style it holds to.
func allianceStrength(s session.Session, w *ctx) Result {
	p := w.Player()
	var defenders []game.AllyContribution
	w.With(func() { defenders = w.AllyDefenders(p) })
	fmt.Fprintf(s, "\n%s%s\n", ansi.FgWhite, tr(s, "Your allies will send the following to aid you in defense:"))
	if len(defenders) == 0 {
		fmt.Fprintf(s, "%s%s\n", tr(s, "You have no defense allies."), ansi.Reset)
		pause(s)
		return Stay
	}
	fmt.Fprintf(s, "%-*s%*s%*s%*s%s\n", allyNameWidth, tr(s, "Name"),
		allyColumnWidth, tr(s, "Troopers"), allyColumnWidth, tr(s, "Tanks"),
		allyColumnWidth, tr(s, "Agents"), ansi.Reset)
	rule := ansi.FgRed + insetRule(allyRuleWidth, allyRuleDouble) + ansi.Reset
	fmt.Fprintln(s, rule)
	var total game.AllyContribution
	for _, d := range defenders {
		allyRow(s, d.Name, d.Troopers, d.Tanks, d.Agents)
		total.Troopers += d.Troopers
		total.Tanks += d.Tanks
		total.Agents += d.Agents
	}
	fmt.Fprintln(s, rule)
	allyRow(s, tr(s, "Total Forces"), total.Troopers, total.Tanks, total.Agents)
	pause(s)
	return Stay
}

// allyRow prints one line of the Alliance Strength table.
func allyRow(s session.Session, name string, troopers, tanks, agents int) {
	fmt.Fprintf(s, "%s%-*s%s%*s%*s%*s%s\n", ansi.FgBrightWhite, allyNameWidth, name,
		ansi.FgBrightYellow, allyColumnWidth, allyFigure(s, troopers),
		allyColumnWidth, allyFigure(s, tanks),
		allyColumnWidth, allyFigure(s, agents), ansi.Reset)
}

// allyFigure renders a contribution, showing a zero as BRE's "NONE".
func allyFigure(s session.Session, n int) string {
	if n == 0 {
		return tr(s, "NONE")
	}
	return comma(n)
}

// treatyDescriptions explains, in plain English, what each pact does in IB —
// shown when the player opens that treaty type's negotiation (BRE shows a pact
// blurb before the send-to list). Wording is IB's own (not copied from BRE) and
// describes IB's actual mechanics (see internal/game/diplomacy.go).
var treatyDescriptions = map[string]string{
	"Full Defense Alliance":  "Neither realm may attack the other. If either is attacked, the ally sends 30% of its troopers and tanks to reinforce the defense.",
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
		// Show what this pact does before choosing a partner, as BRE does — its
		// blurb sits above the "Send to:" prompt.
		if desc := treatyDescriptions[ttype]; desc != "" {
			fmt.Fprintf(s, "\n%s%s%s\n%s%s%s\n",
				ansi.FgBrightYellow, tr(s, ttype), ansi.Reset,
				ansi.Dim, wrapIndented(tr(s, desc), "  "), ansi.Reset)
		}
		to, target := pickRecipient(s, w, pickOpts{prompt: "Send to:", allowAll: true})
		switch target {
		case targetNone:
			return Stay
		case targetOne:
			negotiateWithType(s, w, to.Name, ttype)
			return Stay
		}
		// Z=All: BRE sends one proposal to every realm at once. Each is the
		// ordinary single proposal, so the existing rules hold — a new offer
		// replaces a pending one, and a realm already holding this pact is left
		// alone rather than asked to break it.
		var names []string
		w.With(func() {
			p := w.Player()
			for _, e := range recipients(w) {
				if !w.World.HasTreaty(p, e, ttype) {
					names = append(names, e.Name)
				}
			}
		})
		if len(names) == 0 {
			ok(s, "There is no one left to offer that to.")
			return Stay
		}
		message := askTreatyMessage(s)
		sent := 0
		for _, name := range names {
			if err := proposeTreatyTo(w, name, ttype, message); err == nil {
				sent++
			}
		}
		ok(s, "Your proposal went to %d realms.", sent)
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
		if err := proposeTreatyTo(w, ename, ttype, askTreatyMessage(s)); err != nil {
			fail(s, err)
			return
		}
		ok(s, "Proposed a %s to %s.", ttype, ename)
	}
}

// askTreatyMessage is BRE's "Would you like to attach a message? (y/N)", asked
// once per proposal — or once per Z=All batch, which sends the same note to
// every realm.
func askTreatyMessage(s session.Session) string {
	if !AskYesNo(s, "Attach a message?", false) {
		return ""
	}
	return prompt(s, "Message:")
}

// proposeTreatyTo files one proposal. Silent, so the Z=All batch can report a
// single total instead of pausing on every realm. Both empires are re-resolved
// inside the transaction: a concurrent node that eliminates either between the
// prompt and the write aborts this proposal cleanly.
func proposeTreatyTo(w *ctx, ename, ttype, message string) error {
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
	return err
}

// declareWar is BRE's Declaration Of War: pick a target and, on confirmation,
// end the agreement held with them. It is not the cheap way out — the original
// takes a quarter of both popular support and military morale for it, so the
// confirmation says as much (game.DeclareWar).
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
	if !AskYesNo(s, "Declare war? Breaking an agreement costs a quarter of your support and morale.", false) {
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
		// One relation per pair (#88), so this ends whatever stood and leaves the
		// two realms hostile. Ending a real pact this way costs support and
		// morale; declaring on a realm you had no agreement with is free.
		broke = w.World.TreatiesBetween(p, target)
		w.World.DeclareWar(p, target)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	if len(broke) == 0 {
		ok(s, "You declared war on %s.", targetName)
	} else {
		ok(s, "You declared war on %s. Agreement ended: %s", targetName, strings.Join(broke, ", "))
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

// Relations-screen geometry, measured off a live BRE capture: a 75-column inset
// rule, and a row of "[X]  " then the realm name in a 40-column field.
const (
	relationsRuleWidth  = 75
	relationsRuleDouble = 15
	relationsNameWidth  = 40
)

// viewDiplomacy is BRE's View Treaties — its "-*Relations*-" screen. It lists
// EVERY living realm with the pact held, "None" included, not just the ones under
// treaty, so the roster doubles as the empire-letter key. Colors are the
// original's: the brackets and rules blue, the letter bright white, the name
// bright cyan, the relation bright blue.
//
// IB ADDS a short "Awaiting a reply" list under the table (#92). In BRE a
// proposal you sent is invisible — the target still reads "None", exactly like a
// realm you never contacted — so there is no way to tell a request you made from
// one you only meant to make. It is a safe divergence: your own outgoing
// proposals are information you already have, so it reveals nothing about anyone
// else. It sits below the table rather than in the Relations column because a
// pact plus a pending type would overflow 80 columns.
func viewDiplomacy(s session.Session, w *ctx) Result {
	p := w.Player()
	type row struct {
		id, name, treaties string
		online             bool
	}
	var rows []row
	var pending []game.PendingProposal
	w.With(func() {
		pending = w.ProposalsFrom(p)
		for _, e := range w.Empires {
			if e == p || !e.Alive {
				continue
			}
			held := w.TreatiesBetween(p, e)
			named := make([]string, len(held))
			for i, t := range held {
				named[i] = tr(s, t)
			}
			relations := tr(s, "None")
			if len(named) > 0 {
				relations = strings.Join(named, ", ")
			}
			rows = append(rows, row{w.EmpireLetter(e), e.Name, relations, e.Online()})
		}
	})
	rule := ansi.FgBlue + insetRule(relationsRuleWidth, relationsRuleDouble) + ansi.Reset
	fmt.Fprintf(s, "\n%s-*%s%s%s*-%s\n\n", ansi.FgBlue, ansi.FgBrightWhite, tr(s, "Relations"), ansi.FgBlue, ansi.Reset)
	fmt.Fprintf(s, "%s%-5s%-*s%s%s\n", ansi.FgBrightWhite,
		tr(s, "Id"), relationsNameWidth, tr(s, "Empire Name"), tr(s, "Relations"), ansi.Reset)
	fmt.Fprintln(s, rule)
	for _, r := range rows {
		// The online mark takes the FIRST of the two spaces BRE leaves after
		// "[X]", so the second still separates it from the name and every
		// captured column holds: name field 40, Relations at 45. A headed column
		// of its own — as the two score tables get — would move Relations, and
		// this screen is one of the closest matches to the original. The cost is
		// that the mark has no heading here; the Scores screen is where a player
		// meets it labelled.
		fmt.Fprintf(s, "%s[%s%s%s]%s  %s%s%s%s\n", ansi.FgBlue, ansi.FgBrightWhite, r.id, ansi.FgBlue,
			ansi.Reset,
			nameCell(s, r.name, ansi.FgBrightCyan, r.online, relationsNameWidth),
			ansi.FgBrightBlue, r.treaties, ansi.Reset)
	}
	fmt.Fprintln(s, rule)
	if len(pending) > 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "Awaiting a reply:"), ansi.Reset)
		for _, o := range pending {
			fmt.Fprintf(s, "  %s%s%s — %s%s%s\n", ansi.FgBrightCyan, o.To, ansi.FgWhite,
				ansi.FgBrightYellow, tr(s, o.Type), ansi.Reset)
		}
	}
	pause(s)
	return Stay
}
