package game

import (
	"fmt"
	"strings"
)

// TreatyTypes are the agreement types two empires can form, from the BRE
// manual's Diplomacy menu. All seven now carry an effect (#11):
//   - Full Defense Alliance blocks attacks between the two empires and combines
//     offense/defense (see AreAllied, AllianceStrength).
//   - Tariff and Free Trade Agreements add per-turn trade income (tradeIncome).
//   - Intelligence Alliance and Terrorist Prevention lend agents to covert
//     offense/defense (covert.go).
//   - Technology Agreement shares a partner's tech (techAgreementCeiling,
//     advanceTech).
//   - Protective Trade guards the two realms' trade from covert bombing
//     (BombTradeRoutes, BombTradingMarket).
var TreatyTypes = []string{
	"Full Defense Alliance",
	"Tariff Trade Agreement",
	"Free Trade Agreement",
	"Protective Trade",
	"Terrorist Prevention",
	"Intelligence Alliance",
	"Technology Agreement",
}

const fullDefenseAlliance = "Full Defense Alliance"

// RelationEnemy is the hostile state a pair falls into when one side declares
// war or breaks a pact by attacking. It is stored as a Treaty row like any other
// relation so a pair always has exactly one entry (see setRelation); it is not a
// treaty and confers nothing.
//
// BRE keeps ONE relation value per pair — its enum runs -1 Enemy, 0 None, then
// 1..7 for the seven pacts and 8 Declaration Of War (BRE.OVR value->name
// dispatch, table at 0x1C0E3). IB used to keep a LIST, so the same two realms
// could stack a trade pact, an intelligence alliance and a defense alliance at
// once and collect every benefit. Forming a relation now REPLACES whatever stood
// before, which is what makes diplomacy a choice (#88).
const RelationEnemy = "Enemy"

// Treaty is the single standing relation between two empires. A and B are the
// empire names in canonical (sorted) order so a pair has one key.
type Treaty struct {
	Type string
	A, B string
}

// TreatyOffer is a pending proposal recorded on the target empire.
type TreatyOffer struct {
	From    string
	Type    string
	Message string // optional note the proposer attached; shown to the recipient
}

// treatyPair returns the two names in canonical order.
func treatyPair(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}

func hasTreatyRaw(ts []Treaty, ttype, x, y string) bool {
	for _, t := range ts {
		if t.Type == ttype && t.A == x && t.B == y {
			return true
		}
	}
	return false
}

// HasTreaty reports whether a and b share a treaty of the given type.
func (w *World) HasTreaty(a, b *Empire, ttype string) bool {
	x, y := treatyPair(a.Name, b.Name)
	return hasTreatyRaw(w.Treaties, ttype, x, y)
}

// TreatiesBetween returns the relation a and b currently hold, as a slice so
// existing callers and displays are unchanged. A pair has at most one, so this
// returns zero or one element.
func (w *World) TreatiesBetween(a, b *Empire) []string {
	x, y := treatyPair(a.Name, b.Name)
	for _, t := range w.Treaties {
		if t.A == x && t.B == y {
			return []string{t.Type}
		}
	}
	return nil
}

// Relation returns the single relation between a and b, or "" when they have
// none. This is the accessor that matches BRE's model; TreatiesBetween is the
// list-shaped view kept for display code.
func (w *World) Relation(a, b *Empire) string {
	x, y := treatyPair(a.Name, b.Name)
	for _, t := range w.Treaties {
		if t.A == x && t.B == y {
			return t.Type
		}
	}
	return ""
}

// setRelation makes ttype the ONLY relation between a and b, replacing whatever
// stood before. Passing "" clears the pair to no relation at all (BRE's 0/None).
func (w *World) setRelation(aName, bName, ttype string) {
	x, y := treatyPair(aName, bName)
	out := w.Treaties[:0]
	for _, t := range w.Treaties {
		if !(t.A == x && t.B == y) {
			out = append(out, t)
		}
	}
	w.Treaties = out
	if ttype != "" {
		w.Treaties = append(w.Treaties, Treaty{Type: ttype, A: x, B: y})
	}
}

// AreAllied reports a standing Full Defense Alliance — the pact that blocks
// attacks between the two empires (used by Targets).
func (w *World) AreAllied(a, b *Empire) bool {
	return w.HasTreaty(a, b, fullDefenseAlliance)
}

// alliesOf returns every living empire that shares a treaty of ttype with e.
func (w *World) alliesOf(e *Empire, ttype string) []*Empire {
	var out []*Empire
	for _, other := range w.Empires {
		if other != e && other.Alive && w.HasTreaty(e, other, ttype) {
			out = append(out, other)
		}
	}
	return out
}

// allyAgents sums the agents of every empire sharing a treaty of ttype with e.
// Used to blend allied intelligence into covert odds.
func (w *World) allyAgents(e *Empire, ttype string) int {
	sum := 0
	for _, ally := range w.alliesOf(e, ttype) {
		sum += ally.Agents
	}
	return sum
}

// tradeIncome is the extra per-turn gold from trade treaties, scaled by
// population: a Free Trade Agreement earns more than a Tariff Trade Agreement.
func (w *World) tradeIncome(e *Empire) int {
	tariff := len(w.alliesOf(e, "Tariff Trade Agreement"))
	free := len(w.alliesOf(e, "Free Trade Agreement"))
	return tariff*e.People/40 + free*e.People/20
}

// A Technology Agreement lets a partner's research help your own (#11). BRE adds
// an unmultiplied research contribution per partner, bounded by whichever side
// holds less Technology — so a pact with a strong partner helps, but does not
// substitute for holding tech yourself. Applied in advanceTech.

// AllyContribution is the detachment a Full Defense Alliance partner sends to aid
// an ally under attack — BRE-verified as 30% of the ally's troopers, tanks, and
// agents (turrets/jets/bombers/carriers stay home). Mirrors BRE's Alliance
// Strength screen.
type AllyContribution struct {
	Name                    string
	Troopers, Tanks, Agents int
}

// AllyDefenders returns what each of e's Full Defense Alliance partners will send
// to reinforce e in defense: AllyDefenseContribPct% of their troopers, tanks, and
// agents. Empty if e holds no such alliance.
func (w *World) AllyDefenders(e *Empire) []AllyContribution {
	var out []AllyContribution
	for _, ally := range w.alliesOf(e, fullDefenseAlliance) {
		out = append(out, AllyContribution{
			Name:     ally.Name,
			Troopers: ally.Troopers * AllyDefenseContribPct / 100,
			Tanks:    ally.Tanks * AllyDefenseContribPct / 100,
			Agents:   ally.Agents * AllyDefenseContribPct / 100,
		})
	}
	return out
}

// allyDefenseBoost is the extra battle power d's Full Defense Alliance partners
// add when d is attacked: each sends 30% of its troopers + tanks (agents are
// covert, turrets stay home), valued exactly as the ally's own Defense() weighs
// those units (tanks 3–5 troopers by HQ, then morale- and tech-scaled).
func (w *World) allyDefenseBoost(d *Empire) int {
	sum := 0
	for _, ally := range w.alliesOf(d, fullDefenseAlliance) {
		troopers := ally.Troopers * AllyDefenseContribPct / 100
		tanks := ally.Tanks * AllyDefenseContribPct / 100
		base := troopers + tankStrength(tanks, ally.HQ)
		v := base * moraleFactor(ally.Morale) / 100
		sum += techRaise(v, ally.TechMilitaryFactor())
	}
	return sum
}

// bleedAllies applies pct% casualties to each Full Defense Alliance partner's
// committed detachment (its sent 30% of troopers + tanks) after a battle in which
// d was defended — the reinforcements bleed at the same rate as the defender.
func (w *World) bleedAllies(d *Empire, pct int) {
	for _, ally := range w.alliesOf(d, fullDefenseAlliance) {
		ally.Troopers -= ally.Troopers * AllyDefenseContribPct / 100 * pct / 100
		ally.Tanks -= ally.Tanks * AllyDefenseContribPct / 100 * pct / 100
	}
}

// ProposeTreaty records a pending offer of ttype from `from` to `to` with no
// attached message. Thin wrapper over ProposeTreatyWithMessage.
func (w *World) ProposeTreaty(from, to *Empire, ttype string) {
	w.ProposeTreatyWithMessage(from, to, ttype, "")
}

// ProposeTreatyWithMessage records a pending offer of ttype from `from` to `to`
// with an optional attached message, and mails the target. No-op if they already
// hold that treaty or an identical offer is pending.
func (w *World) ProposeTreatyWithMessage(from, to *Empire, ttype, message string) {
	if w.HasTreaty(from, to, ttype) {
		return
	}
	for _, o := range to.TreatyOffers {
		if o.From == from.Name && o.Type == ttype {
			return
		}
	}
	to.TreatyOffers = append(to.TreatyOffers, TreatyOffer{From: from.Name, Type: ttype, Message: message})
	w.SendMail(from, to, Message{
		To:   w.EmpireLetter(to),
		When: w.DateForDay(w.GameDay),
		Body: fmt.Sprintf("Proposes a %s (respond in the Diplomacy menu).", ttype),
	})
}

// AcceptTreaty forms the treaty if `me` has a matching pending offer, consuming
// the offer. Returns false if there was no such offer.
func (w *World) AcceptTreaty(me *Empire, fromName, ttype string) bool {
	found := false
	kept := me.TreatyOffers[:0]
	for _, o := range me.TreatyOffers {
		if o.From == fromName && o.Type == ttype {
			found = true
		} else {
			kept = append(kept, o)
		}
	}
	me.TreatyOffers = kept
	if !found {
		return false
	}
	// A pair holds one relation, so accepting REPLACES whatever stood before —
	// taking a trade pact with an ally gives up the defense alliance.
	w.setRelation(me.Name, fromName, ttype)
	return true
}

// DeclineTreaty removes a matching pending offer from `me` without forming a
// treaty (the counterpart to AcceptTreaty). Returns false if there was no such
// offer.
func (w *World) DeclineTreaty(me *Empire, fromName, ttype string) bool {
	found := false
	kept := me.TreatyOffers[:0]
	for _, o := range me.TreatyOffers {
		if o.From == fromName && o.Type == ttype {
			found = true
		} else {
			kept = append(kept, o)
		}
	}
	me.TreatyOffers = kept
	return found
}

// BreakTreaty ends a treaty of ttype between a and b, leaving them with no
// relation. A no-op when that is not the relation they hold.
func (w *World) BreakTreaty(a, b *Empire, ttype string) {
	if w.Relation(a, b) == ttype {
		w.setRelation(a.Name, b.Name, "")
	}
}

// DeclareWar is BRE's Declaration Of War: the formal way to end an agreement.
// Its manual is explicit that this breaks a pact "without causing internal
// troubles in your realm" — so unlike breaching a treaty by attacking
// (breachTreaty), it costs no popular support. The pair is left at Enemy and the
// other realm is notified by mail.
//
// SIMPLIFICATION, flagged rather than hidden: the original says "the treaty is
// not officially broken until the other realm is notified", implying the old
// pact still stands until the message lands. IB breaks it at once and mails the
// notice in the same step; the delayed-break window is not modelled.
func (w *World) DeclareWar(a, b *Empire) {
	w.setRelation(a.Name, b.Name, RelationEnemy)
	w.SendMail(a, b, Message{
		To:   w.EmpireLetter(b),
		When: w.DateForDay(w.GameDay),
		Body: "Declares war on your realm. Any agreement between us is ended.",
	})
}

// breachTreaty ends a pact the dishonourable way — by attacking a realm you had
// an agreement with. BRE's manual describes Declaration Of War as the route that
// avoids "internal troubles in your realm", so the route that skips it must
// cause them: the breaker loses popular support. A pair already at Enemy, or
// with no relation, is not a breach and costs nothing.
func (w *World) breachTreaty(a, b *Empire) {
	rel := w.Relation(a, b)
	if rel == "" || rel == RelationEnemy {
		return
	}
	w.setRelation(a.Name, b.Name, RelationEnemy)
	a.adjustSupport(-TreatyBreachSupportPenalty)
	a.Events = append(a.Events, fmt.Sprintf("Breaking the %s with %s without declaring war cost you popular support.", rel, b.Name))
	b.Events = append(b.Events, fmt.Sprintf("%s attacked you, breaking the %s between your realms.", a.Name, rel))
}

// EnsureTreaties migrates a save that predates typed treaties: old untyped
// alliances become Full Defense Alliance treaties, and old alliance offers
// become Full Defense Alliance offers. Idempotent (clears the legacy fields).
func (w *World) EnsureTreaties() {
	for _, k := range w.Alliances {
		names := strings.SplitN(k, "\x00", 2)
		if len(names) != 2 {
			continue
		}
		x, y := treatyPair(names[0], names[1])
		if !hasTreatyRaw(w.Treaties, fullDefenseAlliance, x, y) {
			w.Treaties = append(w.Treaties, Treaty{Type: fullDefenseAlliance, A: x, B: y})
		}
	}
	w.Alliances = nil
	for _, e := range w.Empires {
		for _, from := range e.AllianceOffers {
			e.TreatyOffers = append(e.TreatyOffers, TreatyOffer{From: from, Type: fullDefenseAlliance})
		}
		e.AllianceOffers = nil
	}

	// Collapse any pair that carries more than one relation. Saves written before
	// #88 could stack several treaties on one pair; the model now allows exactly
	// one, so keep the LAST recorded and discard the rest — the most recently
	// agreed relation is the one that would have replaced the others.
	seen := map[[2]string]bool{}
	kept := make([]Treaty, 0, len(w.Treaties))
	for i := len(w.Treaties) - 1; i >= 0; i-- {
		t := w.Treaties[i]
		key := [2]string{t.A, t.B}
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, t)
	}
	// kept is in reverse order; restore the original ordering for stable output.
	for l, r := 0, len(kept)-1; l < r; l, r = l+1, r-1 {
		kept[l], kept[r] = kept[r], kept[l]
	}
	w.Treaties = kept
}
