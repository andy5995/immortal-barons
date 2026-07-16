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

// Treaty is a standing agreement of a given type between two empires. A and B
// are the empire names in canonical (sorted) order so a pair has one key.
type Treaty struct {
	Type string
	A, B string
}

// TreatyOffer is a pending proposal recorded on the target empire.
type TreatyOffer struct {
	From string
	Type string
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

// TreatiesBetween returns the types of treaty a and b currently share.
func (w *World) TreatiesBetween(a, b *Empire) []string {
	x, y := treatyPair(a.Name, b.Name)
	var out []string
	for _, t := range w.Treaties {
		if t.A == x && t.B == y {
			out = append(out, t.Type)
		}
	}
	return out
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

// techAgreementCeiling is the TechLevel an empire may reach from its Technology
// Agreement partners alone (#11): a capped share of the highest-tech partner's
// level, so even a realm with little Technology of its own gains some of a strong
// partner's advances. Zero if it holds no such treaty. See advanceTech.
func (w *World) techAgreementCeiling(e *Empire) int {
	best := 0
	for _, ally := range w.alliesOf(e, "Technology Agreement") {
		if ally.TechLevel > best {
			best = ally.TechLevel
		}
	}
	return best * TechAgreementCapPct / 100
}

// AllianceStrength returns e's combined offense and defense with its Full
// Defense Alliance partners, plus the ally names.
func (w *World) AllianceStrength(e *Empire) (offense, defense int, allies []string) {
	offense, defense = e.Offense(), e.Defense()
	for _, ally := range w.alliesOf(e, fullDefenseAlliance) {
		offense += ally.Offense()
		defense += ally.Defense()
		allies = append(allies, ally.Name)
	}
	return offense, defense, allies
}

// ProposeTreaty records a pending offer of ttype from `from` to `to`, and
// mails the target. No-op if they already hold that treaty or an identical
// offer is pending.
func (w *World) ProposeTreaty(from, to *Empire, ttype string) {
	if w.HasTreaty(from, to, ttype) {
		return
	}
	for _, o := range to.TreatyOffers {
		if o.From == from.Name && o.Type == ttype {
			return
		}
	}
	to.TreatyOffers = append(to.TreatyOffers, TreatyOffer{From: from.Name, Type: ttype})
	to.Mail = append(to.Mail, fmt.Sprintf("%s proposes a %s (respond in the Diplomacy menu).", from.Name, ttype))
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
	x, y := treatyPair(me.Name, fromName)
	if !hasTreatyRaw(w.Treaties, ttype, x, y) {
		w.Treaties = append(w.Treaties, Treaty{Type: ttype, A: x, B: y})
	}
	return true
}

// BreakTreaty ends a treaty of ttype between a and b.
func (w *World) BreakTreaty(a, b *Empire, ttype string) {
	x, y := treatyPair(a.Name, b.Name)
	out := w.Treaties[:0]
	for _, t := range w.Treaties {
		if !(t.Type == ttype && t.A == x && t.B == y) {
			out = append(out, t)
		}
	}
	w.Treaties = out
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
}
