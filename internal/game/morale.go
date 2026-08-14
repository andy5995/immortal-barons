package game

// Military morale and popular support: the two 0-100 stats the crown spends
// gold to keep up. This file holds the parts of the model that BRE keeps in its
// food-allocation, civil-unrest and end-of-turn routines; the pay-to-boost
// prompts are in payments.go and the constants in balance.go.
//
// Both stats matter more in the original than a status-screen figure suggests:
// BRE's silent Auto-Pay Maintenance branch is gated on BOTH sitting at exactly
// 100 (BRE.EXE 0x3b12-0x3b6d), so anything that nudges either off 100 puts the
// baron back through the manual payment sequence.

// BRE bills the two food obligations separately and scores them separately: the
// people's shortfall costs popular support, the army's costs military morale.
// PeopleFoodUpkeep and ForcesFoodUpkeep in payments.go are the two needs.

// shortfallPenalty is the stat points lost for meeting only `given` of a `need`.
// Every one of BRE's shortfalls — armed forces, regions, the crown tax, and both
// halves of the food bill — is scored on the same +1-on-both-sides ratio, so a
// realm that meets its need exactly pays nothing and one that meets none of it
// pays very nearly, but never quite, the whole scale:
//
//	trunc((1 - (given+1)/(need+1)) x scale)
//
// Only the scale and which stat it lands on differ (see the constants).
func shortfallPenalty(need, given, scale int64) int {
	if need <= 0 || given >= need {
		return 0
	}
	if given < 0 {
		given = 0
	}
	return int((need - given) * scale / (need + 1))
}

// civilWarSeverity is the percentage a civil war will destroy, or zero while the
// obligation was met to at least thresholdPct. BRE rounds this one where it
// truncates the penalties above.
func civilWarSeverity(need, given int64, thresholdPct int) int {
	if need <= 0 || given >= need {
		return 0
	}
	if given < 0 {
		given = 0
	}
	if (given+1)*100 >= int64(thresholdPct)*(need+1) {
		return 0
	}
	short := (need - given) * CivilWarSeverityScale
	return int((2*short + (need + 1)) / (2 * (need + 1))) // round to nearest
}

// feed consumes this turn's food and files what the shortfall costs. The people
// are fed first and the army from what is left, which is the order BRE prompts
// in. Nobody emigrates: BRE has no starvation attrition at all, only the two
// stat penalties and the civil war.
func (w *World) feed(e *Empire) {
	peopleNeed, armyNeed := e.PeopleFoodUpkeep(), e.ForcesFoodUpkeep()
	e.LastFoodConsumed = peopleNeed + armyNeed

	toPeople := min(e.Food, peopleNeed)
	toArmy := min(e.Food-toPeople, armyNeed)
	e.Food -= toPeople + toArmy

	e.PendingSupportPenalty += shortfallPenalty(int64(peopleNeed), int64(toPeople), StarvationPenaltyScale)
	e.PendingMoralePenalty += shortfallPenalty(int64(armyNeed), int64(toArmy), StarvationPenaltyScale)
	e.CivilWarSeverity += civilWarSeverity(int64(peopleNeed), int64(toPeople), FoodCivilWarThresholdPct)
	if toPeople < peopleNeed || toArmy < armyNeed {
		w.postStarvationNews(e)
	}
}

// resolveCivilWar spends a pending civil war: popular support is halved, and the
// filed percentage of the realm's regions and of every military unit type — held
// or escrowed on the Trading Market — is destroyed. Records the severity in
// LastCivilWar for the end-of-turn report.
//
// DIVERGENCE: BRE returns the destroyed regions to the planet-wide land pool it
// sells new land from (config record +0x20). IB has no such pool — its Daily
// Land Creation allowance is per-empire — so the land is simply gone.
func (w *World) resolveCivilWar(e *Empire) {
	e.LastCivilWar = 0
	sev := e.CivilWarSeverity
	e.CivilWarSeverity = 0
	if sev <= 0 {
		return
	}
	if sev > CivilWarPerCent {
		sev = CivilWarPerCent
	}
	e.LastCivilWar = sev
	e.Support /= CivilWarSupportDivisor

	if lost := e.Land * sev / CivilWarPerCent; lost > 0 {
		e.Regions.remove(lost)
		e.syncLand()
	}
	keep := CivilWarPerCent - sev
	for _, good := range []string{"Trooper", "Jet", "Turret", "Bomber", "Tank", "Carrier"} {
		f := marketField(e, good)
		*f = *f / CivilWarPerCent * keep
		if l := w.marketListing(e.Owner, good); l != nil {
			l.Qty = l.Qty / CivilWarPerCent * keep
		}
	}
	e.addEvent("Famine tipped your realm into civil war.")
	w.postCivilWarNews(e)
}

// moraleDesertRate draws this turn's desertion percentage. BRE picks a band from
// the morale figure and jitters it with two independent draws, so the milder
// bands often come out zero or negative and nobody leaves.
func (w *World) moraleDesertRate(e *Empire) int {
	if e.Morale >= MoraleDesertBandTop || e.Morale < 0 {
		return 0
	}
	b := MoraleDesertBands[e.Morale/MoraleDesertBandWidth]
	down := w.rng.Intn(b.Down) // drawn first, as the original draws it
	return b.Base + w.rng.Intn(b.Up) - down
}

// moraleDesertion runs one turn of low-morale desertion. Troopers, jets and
// tanks each desert at the drawn rate, independently, one face of
// MoraleDesertTypeOdds sparing each. Turrets, bombers and carriers never desert:
// the original's routine touches three unit types and stops.
func (w *World) moraleDesertion(e *Empire) {
	e.LastMoraleDesertion = 0
	pct := w.moraleDesertRate(e)
	if pct <= 0 {
		return
	}
	for _, good := range []string{"Trooper", "Jet", "Tank"} {
		if w.rng.Intn(MoraleDesertTypeOdds) == 0 {
			continue
		}
		f := marketField(e, good)
		d := *f / MoraleDesertPerCent * pct
		*f -= d
		e.LastMoraleDesertion += d
		if l := w.marketListing(e.Owner, good); l != nil {
			d = l.Qty / MoraleDesertPerCent * pct
			l.Qty -= d
			e.LastMoraleDesertion += d
		}
	}
}
