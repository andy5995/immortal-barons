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
	w.Read(func() { defenders = w.AllyDefenders(p) })
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
	"Protective Trade":       "Guards the trade route between the two realms: deals in transit between you survive covert bombing, whoever fires. Markets are not covered.",
	"Terrorist Prevention":   "Pools covert agents for defense, making both realms harder to spy on and sabotage.",
	"Intelligence Alliance":  "Shares intelligence — partner agents strengthen your covert operations, both attacking and defending.",
	"Technology Agreement":   "Shares technology — the partner with less advanced tech is pulled up toward the more advanced one.",
}

// diplomacyPickOpts configures the realm picker for every Diplomacy action that
// addresses a realm. The Diplomacy menu calls BRE's selection routine the same
// way each time — a multi-select list whose '?' lists Relations rather than the
// scores (docs/dev/bre-screens.md).
var diplomacyPickOpts = pickOpts{prompt: "Send to:", allowAll: true, relations: true}

// negotiateTreaty returns a Diplomacy menu action for one BRE treaty type
// (#68): mark the realms to address, then propose the pact to each. Marking a
// single realm negotiates with it instead — proposing, or accepting a matching
// offer from them. Treaty types are direct
// menu items rather than hiding behind a single "Modify Diplomacy" item.
func negotiateTreaty(ttype string) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		// Show what this pact does before choosing a partner, as BRE does — its
		// blurb sits above the "Send to:" prompt.
		if desc := treatyDescriptions[ttype]; desc != "" {
			fmt.Fprintf(s, "\n%s%s%s\n%s%s%s\n",
				ansi.FgBrightYellow, tr(s, ttype), ansi.Reset,
				ansi.Dim, WrapIndented(tr(s, desc), "  "), ansi.Reset)
		}
		picked := pickRecipients(s, w, diplomacyPickOpts)
		if len(picked) == 0 {
			return Stay
		}
		var chosen []string
		w.Read(func() {
			for _, e := range picked {
				chosen = append(chosen, e.Name)
			}
		})
		if len(chosen) == 1 {
			// One realm is the negotiation proper: propose it, or accept a
			// matching offer.
			negotiateWithType(s, w, chosen[0], ttype)
			return Stay
		}
		// Several realms take one proposal each — the ordinary single proposal, so
		// the existing rules hold: a new offer replaces a pending one, and a realm
		// already holding this pact is left alone.
		var names []string
		w.With(func() {
			p := w.Player()
			for _, name := range chosen {
				e := findRealm(w, name)
				if e != nil && !w.World.HasTreaty(p, e, ttype) {
					names = append(names, name)
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
// it if ename has already offered it, or — when the pact already stands — say
// so and leave it alone. Both empires are re-resolved by name inside every
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
		// Selecting the pact you already hold is a no-op. It used to offer to
		// break it, which put the one destructive diplomatic act behind the same
		// key as the constructive one; BRE's Diplomacy menu never breaks from a
		// treaty item either, and Declaration Of War is where a standing
		// agreement is ended.
		ok(s, "Your %s with %s stands, and is holding strong.", ttype, ename)
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

// declareWar is BRE's Declaration Of War: mark the realms to declare on — the
// same multi-select list every Diplomacy action uses — and, on confirmation,
// end the agreement held with each. It is not the cheap way out: the original
// takes a quarter of both popular support and military morale for it, once,
// so the confirmation says as much (game.DeclareWar).
func declareWar(s session.Session, w *ctx) Result {
	picked := pickRecipients(s, w, diplomacyPickOpts)
	if len(picked) == 0 {
		return Stay
	}
	var names []string
	w.Read(func() {
		for _, e := range picked {
			names = append(names, e.Name)
		}
	})
	if !AskYesNo(s, "Declare war? Breaking an agreement costs a quarter of your support and morale.", false) {
		return Stay
	}
	for _, name := range names {
		var broke []string
		var err error
		w.With(func() {
			p := w.Player()
			target := findRealm(w, name)
			if p == nil || target == nil {
				err = errTargetGone
				return
			}
			// One relation per pair (#88), so this ends whatever stood and leaves
			// the two realms hostile. Ending a real pact this way costs support and
			// morale; declaring on a realm you held no agreement with is free.
			broke = w.World.TreatiesBetween(p, target)
			w.World.DeclareWar(p, target)
		})
		switch {
		case err != nil:
			fail(s, err)
		case len(broke) == 0:
			ok(s, "You declared war on %s.", name)
		default:
			ok(s, "You declared war on %s. Agreement ended: %s", name, strings.Join(broke, ", "))
		}
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
	return pickFromListValues(s, msg, list, list)
}

// pickFromListValues is pickFromList for a list whose rows are DRAWN with more
// than the value they stand for — a target flagged `(P)` for New Realm
// Protection, say. labels and values are parallel; the value is what comes back,
// so a flag on the row can never reach the code that acts on the choice.
func pickFromListValues(s session.Session, msg string, labels, values []string) string {
	for i, x := range labels {
		fmt.Fprintf(s, "    %d) %s\n", i+1, x)
	}
	i := promptInt(s, msg+" (0 to cancel)?")
	if i < 1 || i > len(values) {
		return ""
	}
	return values[i-1]
}

// Relations-screen geometry, measured off a live BRE capture: a 75-column inset
// rule, and a row of "[X]  " then the realm name in a 40-column field.
const relationsNameWidth = 40

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
	var rows []relationsRow
	var pending []game.PendingProposal
	w.With(func() {
		pending = w.ProposalsFrom(p)
		for _, e := range w.Empires {
			if e == p || !e.Alive {
				continue
			}
			rows = append(rows, relationsRow{
				id: w.EmpireLetter(e), name: e.Name,
				relations: relationsText(s, w.TreatiesBetween(p, e)), presence: presenceOf(e, false, w.Today),
				protected: e.Protection > 0,
			})
		}
	})
	writeRelationsTable(s, w.Term, rows)
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

// relationsRow is one line of the Relations table: a realm's letter, its name,
// and what stands between it and the player.
type relationsRow struct {
	id, name, relations string
	presence            string
	protected           bool
}

// relationsText renders the Relations column for the pacts held, BRE's "None"
// when there are none.
func relationsText(s session.Session, held []string) string {
	if len(held) == 0 {
		return tr(s, "None")
	}
	named := make([]string, len(held))
	for i, t := range held {
		named[i] = tr(s, t)
	}
	return strings.Join(named, ", ")
}

// writeRelationsTable draws the "-*Relations*-" table itself. View Treaties and
// the Diplomacy picker's "?" list share it, as they share one routine in the
// original.
func writeRelationsTable(s session.Session, t Term, rows []relationsRow) {
	rule := rule75(ansi.FgBlue)
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
			nameCell(s, t, r.name, ansi.FgBrightCyan, r.presence, relationsNameWidth),
			ansi.FgBrightBlue, r.relations, ansi.Reset)
	}
	fmt.Fprintln(s, rule)
}
