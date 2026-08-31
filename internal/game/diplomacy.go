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
//   - Protective Trade spares the two realms' trade deals when trade routes are
//     bombed (bombRoutesEffect) and cuts the cost of sending a deal between them
//     (TradeDealGoldPerDayBetween). It shields no market.
var TreatyTypes = []string{
	"Full Defense Alliance",
	"Tariff Trade Agreement",
	"Free Trade Agreement",
	"Protective Trade",
	"Terrorist Prevention",
	"Intelligence Alliance",
	"Technology Agreement",
}

const (
	fullDefenseAlliance  = "Full Defense Alliance"
	intelligenceAlliance = "Intelligence Alliance"
	terroristPrevention  = "Terrorist Prevention"
	protectiveTrade      = "Protective Trade"
	tariffTradeAgreement = "Tariff Trade Agreement"
	freeTradeAgreement   = "Free Trade Agreement"
	technologyAgreement  = "Technology Agreement"
)

// HasPact reports whether a and b hold one of the seven treaties — neither None
// nor Enemy. BRE gates sending a trade deal on exactly this: create_trade_offer
// compares the pair's relation against 1 (`cmp word [es:di+0xae],1; jnl`), and
// its enum runs -1 Enemy, 0 None, 1..7 for the pacts.
func (w *World) HasPact(a, b *Empire) bool {
	rel := w.Relation(a, b)
	return rel != "" && rel != RelationEnemy
}

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

// forgetRelations erases every trace of a realm from the diplomacy tables: the
// Treaty rows naming it, and the proposals it sent that are still sitting
// unanswered on other realms. Offers made TO it are held on its own record and
// go with it. Called by dropEmpires for every removed realm — see the reasoning
// there for why a leftover row is not merely untidy.
func (w *World) forgetRelations(name string) {
	kept := w.Treaties[:0]
	for _, t := range w.Treaties {
		if t.A == name || t.B == name {
			continue
		}
		kept = append(kept, t)
	}
	w.Treaties = kept
	for _, e := range w.Empires {
		keptOffers := e.TreatyOffers[:0]
		for _, o := range e.TreatyOffers {
			if o.From == name {
				continue
			}
			keptOffers = append(keptOffers, o)
		}
		e.TreatyOffers = keptOffers
	}
}

// AreAllied reports a standing Full Defense Alliance — the pact that blocks
// attacks between the two empires (used by Targets).
func (w *World) AreAllied(a, b *Empire) bool {
	return w.HasTreaty(a, b, fullDefenseAlliance)
}

// TreatyPartners returns every living empire holding a standing relation with e
// of any treaty type. Enemy is stored as a relation but is not a treaty (see
// RelationEnemy), so an enemy is not a partner.
func (w *World) TreatyPartners(e *Empire) []*Empire {
	var out []*Empire
	for _, other := range w.Empires {
		if other == e || !other.Alive {
			continue
		}
		if rel := w.Relation(e, other); rel != "" && rel != RelationEnemy {
			out = append(out, other)
		}
	}
	return out
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

// tradeIncome is the extra per-turn gold from trade treaties. Each partner pays
// on the SMALLER of the two populations, so a pact with a tiny realm earns
// little and neither side can farm a giant; a Free Trade Agreement is worth
// nearly twice a Tariff. See TariffTradeGoldPerHead for the binary evidence and
// — importantly — which population unit the rates are in.
func (w *World) tradeIncome(e *Empire) int {
	sum := int64(0)
	for _, other := range w.alliesOf(e, tariffTradeAgreement) {
		sum += tradePactIncome(e, other, TariffTradeGoldPerHead, TariffTradeProtectedSelfCut, TariffTradeProtectedPartnerCut)
	}
	for _, other := range w.alliesOf(e, freeTradeAgreement) {
		sum += tradePactIncome(e, other, FreeTradeGoldPerHead, FreeTradeProtectedSelfCut, FreeTradeProtectedPartnerCut)
	}
	return int(sum)
}

// tradePactIncome is one trade partner's contribution: the smaller of the two
// populations at the pact's per-head rate, with New Realm Protection on either
// side cutting that rate.
func tradePactIncome(e, other *Empire, perHead, selfCut, partnerCut int) int64 {
	rate := perHead
	if e.Protection > 0 {
		rate -= selfCut
	}
	if other.Protection > 0 {
		rate -= partnerCut
	}
	if rate < 0 {
		rate = 0
	}
	// The rates are per BRE population unit, so the count goes back into that
	// unit before they are applied. See TariffTradeGoldPerHead.
	return int64(min(e.People, other.People)) / PopBREUnitScale * int64(rate)
}

// A Technology Agreement lets a partner's research help your own (#11). BRE adds
// an unmultiplied research contribution per partner, bounded by whichever side
// holds fewer Technology REGIONS — so a pact with a strong partner helps, but
// does not substitute for holding tech yourself. Binary-verified; applied in
// advanceTech, where the provenance is.

// A Full Defense Alliance is LOCAL ONLY. BRE's manual says so of the treaty
// itself — "effective only in Local Games" — and the binary matches: the
// relation row belongs to one planet's empire records and never rides an
// inter-BBS packet. So the three helpers below feed the local battle
// (combat.go's Attack) and the Alliance Strength screen, and nothing in the
// interplanetary path may call them: resolveRemoteAttack meets an arriving
// strike with the target's own Defense() alone. Pinned by
// TestFullDefenseAllianceDoesNotDefendAgainstInterplanetaryStrikes.

// AllyContribution is one row of BRE's Alliance Strength screen: what a partner
// sends to aid a realm under attack. The screen has three figure columns and
// TWO treaties feed them, one each:
//
//   - a Full Defense Alliance sends AllyDefenseContribPct% of its troopers and
//     tanks (turrets/jets/bombers/carriers stay home) — it sends NO agents;
//   - a Terrorist Prevention pact sends CovertAllyDefensePct% of its agents,
//     and no troops.
//
// BINARY-VERIFIED (BRE.OVR 0x01177a, send_defensive_aid): the routine sets two
// independent shares from the pair's relation — 0x32 (50) against relation 4,
// Terrorist Prevention, applied to agents; 0x1e (30) against relation 7, Full
// Defense Alliance, applied to troopers and tanks. The attack resolver's aid
// loop likewise reads troopers and tanks only, and the attacker's in-battle line
// names only those two.
//
// IB used to credit an alliance partner 30% of its AGENTS as well, which lent
// help the original never lends. A pair holds one relation at a time
// (setRelation), so a partner appears on exactly one of the two lists and the
// rows cannot double-count.
type AllyContribution struct {
	Name                    string
	Troopers, Tanks, Agents int
}

// AllyDefenders returns what each of e's defensive partners will send to
// reinforce it, one row per partner. Empty if e holds neither pact.
func (w *World) AllyDefenders(e *Empire) []AllyContribution {
	var out []AllyContribution
	for _, ally := range w.alliesOf(e, fullDefenseAlliance) {
		out = append(out, AllyContribution{
			Name:     ally.Name,
			Troopers: ally.Troopers * AllyDefenseContribPct / 100,
			Tanks:    ally.Tanks * AllyDefenseContribPct / 100,
		})
	}
	for _, ally := range w.alliesOf(e, terroristPrevention) {
		out = append(out, AllyContribution{
			Name:   ally.Name,
			Agents: ally.Agents * CovertAllyDefensePct / 100,
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

// bleedAllies applies the given casualty fraction to each Full Defense Alliance
// partner's committed detachment (its sent 30% of troopers + tanks) after a
// battle in which d was defended — the reinforcements bleed at the same rate as
// the defender — and tells each partner what it lost and where.
//
// The notice is not optional bookkeeping. Without it a player's troopers and
// tanks vanish in a battle they were never told about: the loss has no visible
// cause, and nothing else on any screen would ever explain it. BRE files it in
// the SAME loop iteration as the deduction (BRE.OVR 0x00ef90, the aid loop in
// resolve_regular_attack, which passes the ally's own record to the asynchronous
// recap filer), so the two belong together here as well.
//
// The wording is IB's own. Only the FACT that a notice is filed, who receives
// one, and that it is filed unconditionally are taken from the original; its
// sentence is not reproduced.
//
// Colour needs nothing here: showTurnEvents paints realm names bright-cyan and
// highlights figures, both from a live capture, so naming both realms and both
// counts styles this line like every other recap entry.
//
// WHO IS TOLD is wider than who helps, and that is BRE's own shape rather than
// a choice. Its loop guard is `cmp word [es:di-0xebf],0x5 / jg` (BRE.OVR
// 0x10545) — every relation ABOVE 5 — while the detachment share is written
// only at `cmp ax,0x7 / jnz` (0xf541), equality with Full Defense Alliance. A
// Technology Agreement partner therefore receives a battle notice reading zero
// and zero, having contributed nothing and lost nothing. Relation 8 looks like
// it should qualify too, but is never stored: break_diplomatic_treaty writes
// `xor ax,ax` to both relation rows (0x1a8f0, 0x1a912), so a Declaration Of War
// leaves 0 behind and the value exists only as a display string.
//
// NOTHING SUPPRESSES A ZERO LINE. The 236 instructions between the guard and
// the deduction contain no branch of any kind, and the recap filer itself
// (04ef:002f) is 25 instructions with no conditional jump — no zero-total test,
// no dedup. A partner that sent nothing is told so.
//
// The zero lines are faithful, not a defect: if play shows them to be noise,
// suppressing them is a deliberate divergence to be recorded as one, not a bug
// fix to be quietly applied.
func (w *World) bleedAllies(a, d *Empire, frac float64) {
	for _, ally := range w.battleNotified(d) {
		var troopers, tanks int
		if w.HasTreaty(d, ally, fullDefenseAlliance) {
			troopers = shareOf(ally.Troopers*AllyDefenseContribPct/100, frac)
			tanks = shareOf(ally.Tanks*AllyDefenseContribPct/100, frac)
			ally.Troopers -= troopers
			ally.Tanks -= tanks
		}
		ally.addEvent(fmt.Sprintf("%s attacked %s: you lost %d troopers and %d tanks in the defence.",
			a.Name, d.Name, troopers, tanks))
	}
}

// battleNotified returns every living realm BRE tells about a battle fought
// against d — the relations its report loop admits, which is its enum above 5:
// Technology Agreement and Full Defense Alliance. Only the second sends anyone.
func (w *World) battleNotified(d *Empire) []*Empire {
	var out []*Empire
	for _, other := range w.Empires {
		if other == d || !other.Alive {
			continue
		}
		switch w.Relation(d, other) {
		case technologyAgreement, fullDefenseAlliance:
			out = append(out, other)
		}
	}
	return out
}

// ProposeTreaty records a pending offer of ttype from `from` to `to` with no
// attached message. Thin wrapper over ProposeTreatyWithMessage.
func (w *World) ProposeTreaty(from, to *Empire, ttype string) {
	w.ProposeTreatyWithMessage(from, to, ttype, "")
}

// ProposeTreatyWithMessage records a pending offer of ttype from `from` to `to`
// with an optional attached message, and mails the target. No-op if they already
// hold that treaty.
//
// A new proposal REPLACES any other still-unanswered proposal to the same realm
// (#92), for the same reason a pair holds one relation at a time (#88): only one
// of them can ever be agreed, so leaving both pending would let a realm answer a
// pact you had already thought better of.
func (w *World) ProposeTreatyWithMessage(from, to *Empire, ttype, message string) {
	if w.HasTreaty(from, to, ttype) {
		return
	}
	for _, o := range to.TreatyOffers {
		if o.From == from.Name && o.Type == ttype {
			return // already asked for exactly this; don't mail them twice
		}
	}
	kept := to.TreatyOffers[:0]
	for _, o := range to.TreatyOffers {
		if o.From != from.Name {
			kept = append(kept, o)
		}
	}
	to.TreatyOffers = append(kept, TreatyOffer{From: from.Name, Type: ttype, Message: message})
	// A proposal mails nothing. The covering message rides on the offer and is
	// shown with it; the mailbox is for messages the player sent through Send
	// Message. Captured from a live game (cap/kd3-01.cap): one proposal carries
	// an inline "Message attached:" block, the next has none, and the mail read
	// moments later is an unrelated conversation. IB used to mail a generated
	// "proposes a pact" line on every proposal, which told the target what the
	// offer prompt had just asked them.
}

// PendingProposal is one treaty offer YOU sent that has not been answered yet.
type PendingProposal struct {
	To   string
	Type string
}

// ProposalsFrom returns the still-unanswered offers `from` has sent. A proposal
// is stored on the RECIPIENT, so the sender's own list is derived by scanning
// rather than by keeping a second copy that could drift out of step (#92).
// Proposals do not expire: they stand until the other realm accepts, rejects, or
// is eliminated.
func (w *World) ProposalsFrom(from *Empire) []PendingProposal {
	var out []PendingProposal
	for _, e := range w.Empires {
		if !e.Alive || e == from {
			continue
		}
		for _, o := range e.TreatyOffers {
			if o.From == from.Name {
				out = append(out, PendingProposal{To: e.Name, Type: o.Type})
			}
		}
	}
	return out
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
	w.notifyProposer(me, fromName, "accepted", ttype)
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
	if found {
		w.notifyProposer(me, fromName, "rejected", ttype)
	}
	return found
}

// notifyProposer tells the empire that sent an offer how it was answered. BRE
// files this on the proposer's "since your last play" log — one line per reply,
// arriving whenever the other realm got around to playing — so a proposal is
// never met with silence. Wording is BRE's, driven live on a league game.
func (w *World) notifyProposer(me *Empire, fromName, verb, ttype string) {
	from := w.FindByName(fromName)
	if from == nil {
		return
	}
	from.addEvent(fmt.Sprintf("%s %s your %s proposal.", me.Name, verb, ttype))
}

// BreakTreaty ends a treaty of ttype between a and b, leaving them with no
// relation. A no-op when that is not the relation they hold.
func (w *World) BreakTreaty(a, b *Empire, ttype string) {
	if w.Relation(a, b) == ttype {
		w.setRelation(a.Name, b.Name, "")
	}
}

// DeclareWar is IB's formal way to end an agreement, and the one place the two
// games are known to part company. "Declaration Of War" exists in the original
// only as a RELATION label in format_diplomatic_status, beside Enemy, None and
// the seven pacts; no routine a player invokes bears that name, and the support
// and morale charge belongs to the breach path (see BreachTreaty). IB keeps the
// menu item and the charge pending a decision on both (#242). Only ending a real
// pact costs, the pair is left at Enemy, and the other realm is notified by
// mail.
//
// The break is IMMEDIATE, and so is the original's, despite the manual's "the
// treaty is not officially broken until the other realm is notified": BRE clears
// both empires' relation rows in the same routine that prompts, with nothing
// waiting on the message being read. There is no delayed-break window to model.
func (w *World) DeclareWar(a, b *Empire) {
	if rel := w.Relation(a, b); rel != "" && rel != RelationEnemy {
		a.Support = a.Support / TreatyBreakKeepDenominator * TreatyBreakKeepNumerator
		a.Morale = a.Morale / TreatyBreakKeepDenominator * TreatyBreakKeepNumerator
		a.addEvent(fmt.Sprintf("Tearing up the %s with %s set off revolts at home; support and morale fell sharply.", rel, b.Name))
	}
	w.setRelation(a.Name, b.Name, RelationEnemy)
	w.SendMail(a, b, Message{
		To:   w.EmpireLetter(b),
		Body: "Declares war on your realm. Any agreement between us is ended.",
	})
}

// BreachTreaty tears up a pact by aiming an attack at the realm that holds it,
// and charges the breaker a quarter of both popular support and military morale
// (TreatyBreakKeepNumerator). It returns the relation it ended, or "" when there
// was nothing to break.
//
// The original puts all of this in break_diplomatic_treaty (BRE.OVR 0x01a838),
// reached only from the target picker its regular, nuclear, chemical and
// biological attacks share — so the confirmation, the charge and the break all
// land BEFORE the force is committed, and the battle is then fought at the
// reduced morale. The caller asks first; refusing aborts the attack.
//
// The pair is left with NO relation rather than at Enemy: the original zeroes
// both realms' relation words (0x624 and 0x646), and 0 is None in its enum.
//
// IB charged nothing here until 2026-08-31, on the reading that no attack path
// consults the relation. It does, one level up.
func (w *World) BreachTreaty(a, b *Empire) string {
	rel := w.Relation(a, b)
	if rel == "" || rel == RelationEnemy {
		return ""
	}
	a.Support = a.Support / TreatyBreakKeepDenominator * TreatyBreakKeepNumerator
	a.Morale = a.Morale / TreatyBreakKeepDenominator * TreatyBreakKeepNumerator
	w.setRelation(a.Name, b.Name, "")
	a.addEvent(fmt.Sprintf("You tore up the %s with %s to attack it. Revolts broke out at home; support and morale fell sharply.", rel, b.Name))
	b.addEvent(fmt.Sprintf("%s tore up the %s between your realms to attack you.", a.Name, rel))
	return rel
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
