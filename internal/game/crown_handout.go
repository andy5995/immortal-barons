package game

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// crown_handout.go — the Queen's purse pays out twice: the daily refund every
// realm draws for itself (QueenRefund, payments.go) and the occasional
// planet-wide handout here, which nobody asks for.
//
// BINARY-VERIFIED against write_economic_policy_news (BRE.OVR 0x04F082); the
// constants and their provenance are in balance_crown.go. Wording is IB's own.

// livingEmpires returns every realm still in the game. The handouts pay each of
// them and charge the purse per head, so the count is part of the arithmetic
// rather than just a loop bound.
func (w *World) livingEmpires() []*Empire {
	out := make([]*Empire, 0, len(w.Empires))
	for _, e := range w.Empires {
		if e.Alive {
			out = append(out, e)
		}
	}
	return out
}

// maybeCrownHandout runs the crown's end-of-turn event roll. It fires on
// CrownEventChancePct of turns and then picks one of CrownEventKinds events,
// two of which pay out of the purse — so a handout of either kind lands on
// roughly one turn in six hundred.
//
// It is called once per turn by the realm whose turn it is, but what it pays is
// planet-wide: every living realm gains, not just the caller. That is the
// original's shape, and it is why the purse can empty between one realm's turns.
func (w *World) maybeCrownHandout() {
	if w.rng.Intn(100) >= CrownEventChancePct {
		return
	}
	switch w.rng.Intn(CrownEventKinds) {
	case CrownGoldRoll:
		w.crownGoldHandout()
	case CrownTrooperRoll:
		w.crownTrooperHandout()
	}
}

// crownGoldHandout pays every living realm trunc(purse/CrownGoldDivisor) gold.
// The share is computed ONCE, before the loop, and the purse is charged it for
// each realm — so the last realm in the list is paid the same as the first even
// though the purse has shrunk, and the purse can be driven negative-ish by a
// large league. It is clamped at zero here; the original leans on the share
// having been small relative to the purse.
func (w *World) crownGoldHandout() {
	if w.RefundPool <= 0 {
		return
	}
	share := w.RefundPool / CrownGoldDivisor
	if share <= 0 {
		return
	}
	living := w.livingEmpires()
	for _, e := range living {
		w.RefundPool -= share
		if w.RefundPool < 0 {
			w.RefundPool = 0
		}
		w.creditGold(e, share, "the Queen's bounty")
		e.addEvent(fmt.Sprintf("The Queen opens her purse to the whole planet: %s gold is yours.",
			numfmt.Comma(share)))
	}
	if len(living) > 0 {
		w.postNews(fmt.Sprintf("The Queen empties part of her purse over the planet — %s gold to every realm.",
			numfmt.Comma(share)))
	}
}

// crownTrooperHandout gives every living realm the same number of troopers,
// bought out of the purse at CrownTrooperPrice each. The share is divided by the
// number of realms first, so a crowded planet gets fewer each — the opposite of
// the gold handout, and the reason the two are worth keeping apart.
func (w *World) crownTrooperHandout() {
	living := w.livingEmpires()
	if len(living) == 0 || w.RefundPool <= 0 {
		return
	}
	share := int(w.RefundPool / CrownTrooperPrice / int64(len(living)))
	if share > CrownTrooperMax {
		share = CrownTrooperMax
	}
	// "Skipped when it comes out at zero" — a purse too thin to buy one trooper
	// each buys none, rather than rounding somebody up.
	if share <= 0 {
		return
	}
	cost := int64(len(living)) * int64(share) * CrownTrooperPrice
	if cost > w.RefundPool {
		cost = w.RefundPool
	}
	w.RefundPool -= cost
	for _, e := range living {
		e.Troopers += share
		e.addEvent(fmt.Sprintf("The Queen's recruiters hand you %s troopers, paid for out of the crown purse.",
			numfmt.Comma(int64(share))))
	}
	w.postNews(fmt.Sprintf("The crown buys %s troopers for every realm on the planet.",
		numfmt.Comma(int64(share))))
}
