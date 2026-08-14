package game

import (
	"fmt"
	"strings"
)

// Bombing-run tuning: each bomber that survives the defender's anti-air
// destroys up to BomberJetKills of the defender's grounded jets; the
// defender shoots down one bomber per TurretsPerBomberDown turrets, and SDI
// blunts the raid by its percentage.
const (
	BomberJetKills       = 3
	TurretsPerBomberDown = 25

	// JetsPerCarrier is how many jets one carrier can transport to a battle; jets
	// beyond that are grounded (not "usable" — BRE's Offense/attack force screen).
	JetsPerCarrier = 100

	// A Normal attack captures max(RegularAttackCaptureFloor, the Attack Rewards
	// share of the loser's regions), capped at what the loser holds. The floor is
	// BINARY-VERIFIED: BRE.OVR 0x1009f pushes 15 into the max, 0x100c3 the
	// defender's own region count into the min. The share itself is a per-level
	// table — see AttackCaptureMediumPct in balance.go.
	RegularAttackCaptureFloor = 15

	// Capture-density modifier (IB-original): a Normal attack takes more regions
	// from a defender whose net worth is spread thinner over its land than the
	// ATTACKER's — cheap, lightly-held land falls faster — and fewer from a
	// denser, better-developed realm. The multiplier is the attacker's
	// net-worth-per-region over the defender's, as a percent, clamped to
	// [CaptureDensityMin, CaptureDensityMax]; equal density gives CaptureDensityBase
	// (no change). No BRE-verified density formula exists — public strategy guides
	// describe the *tactic* of preying on high-region, low-net-worth realms, not a
	// number — so this is IB's own and may change. Tune freely.
	CaptureDensityBase = 100 // percent: equal-density result
	CaptureDensityMin  = 50  // percent (0.5x): densest targets
	CaptureDensityMax  = 200 // percent (2.0x): softest targets

	// LandDefenseBonus is the defensive strength each region adds on top of the
	// defender's units (terrain the attacker must take). Used in the battle math
	// and by the AI when judging whether a target is winnable (#36).
	LandDefenseBonus = 2
)

// returningForces opens a battle report. BRE puts the casualties first and the
// verdict last, under a line about the army coming home worn out, and it prints
// the same opening whether the attack won or lost — so a player reads what the
// fight cost before learning how it went. The wording is IB's own; only the
// order and the per-unit breakdown are the original's (docs/dev/bre-screens.md).
const returningForces = "Your forces have returned from the field, exhausted.\n"

// bombingRun sends a's bombers against d's airfields before the ground
// clash. It destroys grounded jets (which don't defend anyway, so this
// only weakens d's future offense and net worth) and costs a some bombers
// to anti-air. It mutates both empires and returns (jetsDestroyed,
// bombersLost).
func (w *World) bombingRun(a, d *Empire, bombers int) (int, int) {
	if bombers <= 0 || d.Jets <= 0 {
		return 0, 0
	}
	lost := min(bombers, d.Turrets/TurretsPerBomberDown)
	survivors := bombers - lost
	kills := survivors * BomberJetKills * (100 - d.SDI) / 100
	if kills > d.Jets {
		kills = d.Jets
	}
	d.Jets -= kills
	a.Bombers -= lost
	return kills, lost
}

// captureDensityFactor returns the percent multiplier on a Normal attack's
// region capture (see the CaptureDensity constants): the attacker's
// net-worth-per-region over the defender's, clamped, so a defender softer than
// the attacker bleeds more land and a denser one less. A bankrupt/undefended
// defender counts as maximally soft; a landless or worthless attacker gets the
// neutral base.
func (w *World) captureDensityFactor(a, d *Empire) int {
	nwA, nwD := w.NetWorth(a), w.NetWorth(d)
	if d.Land <= 0 || nwD <= 0 {
		return CaptureDensityMax
	}
	if a.Land <= 0 || nwA <= 0 {
		return CaptureDensityBase
	}
	// (nwA/a.Land) / (nwD/d.Land) * 100, cross-multiplied to stay integer.
	factor := int64(nwA) * int64(d.Land) * 100 / (int64(a.Land) * int64(nwD))
	if factor < CaptureDensityMin {
		return CaptureDensityMin
	}
	if factor > CaptureDensityMax {
		return CaptureDensityMax
	}
	return int(factor)
}

// CanAttack reports whether e may launch another individual (conventional)
// attack today. Config.MaxIndividualAttacks <= 0 means unlimited (matching the
// MaxRegions "<= 0 = no cap" convention).
func (w *World) CanAttack(e *Empire) bool {
	return underDailyCap(e.AttacksToday, w.Config.MaxIndividualAttacks)
}

// underDailyCap reports whether one more of something is allowed today. A limit
// of 0 or less means no cap, matching the MaxRegions convention.
func underDailyCap(used, limit int) bool { return limit <= 0 || used < limit }

// CanGroupAttack, CanTerrorOp and CanBombingOp are the interplanetary
// equivalents of CanAttack, against their own per-day allowances.
func (w *World) CanGroupAttack(e *Empire) bool {
	return underDailyCap(e.GroupAttacksToday, w.Config.MaxGroupAttacks)
}

func (w *World) CanTerrorOp(e *Empire) bool {
	return underDailyCap(e.TerrorOpsToday, w.Config.MaxTerrorOps)
}

func (w *World) CanBombingOp(e *Empire) bool {
	return underDailyCap(e.BombingOpsToday, w.Config.MaxBombingOps)
}

// Attack resolves a battle between attacker a and defender d, mutating
// both empires, and returns a battle report. The attacker commits its
// offense; the defender fights with its defense plus a home bonus from its
// land. Both sides apply a random factor.
// Attack resolves a regular attack. On a win the loser bleeds its regions as a
// proportional mix; the winner captures the same count. autoCapture governs how
// the winner GAINS that land: true (AI, group attacks) adds it as the same mix;
// false (the human menu path) leaves the attacker's regions untouched and
// returns the captured count so the caller can let the player pick the types
// (#58 — BRE decouples the loser's mix from the winner's freely-chosen
// composition). captured is 0 on a loss, and excludes any waste taken, which
// transfers as waste on both paths because nobody chooses to hold ruin.
func (w *World) Attack(a, d *Empire, f AttackForce, autoCapture bool) (report string, captured int) {
	a.AttacksToday++ // counts against the daily individual-attack cap (both human and AI)
	f = f.clampTo(a) // only units the attacker actually holds can be committed
	// Attacking a realm you hold an agreement with ends it the dishonourable way
	// and costs popular support at home — the "internal troubles" BRE's manual
	// says a Declaration Of War avoids (#88). Runs before the battle so the
	// alliance reinforcement below sees the broken relation.
	w.breachTreaty(a, d)
	var b strings.Builder
	fmt.Fprintf(&b, "%s attacks %s!\n\n", a.Name, d.Name)

	// Full Defense Alliance: each of the defender's partners sends 30% of its
	// troopers and tanks to reinforce the defense (BRE-verified). Reported to the
	// attacker, matching BRE's "the empire's allies send N Troopers and M Tanks".
	allyTroopers, allyTanks := 0, 0
	for _, ally := range w.alliesOf(d, fullDefenseAlliance) {
		allyTroopers += ally.Troopers * AllyDefenseContribPct / 100
		allyTanks += ally.Tanks * AllyDefenseContribPct / 100
	}
	if allyTroopers+allyTanks > 0 {
		fmt.Fprintf(&b, "%s's allies send %d troopers and %d tanks to aid the defense.\n\n", d.Name, allyTroopers, allyTanks)
	}

	bomberLoss := 0 // folded into the attacker's casualty breakdown below
	if kills, lost := w.bombingRun(a, d, f.Bombers); kills > 0 || lost > 0 {
		bomberLoss = lost
		fmt.Fprintf(&b, "Your bombers hit the airfields: %d enemy jets destroyed", kills)
		if lost > 0 {
			fmt.Fprintf(&b, ", %d bombers lost to anti-air", lost)
		}
		fmt.Fprint(&b, ".\n\n")
	}

	// Military morale scales each side's unit effectiveness (the land defense
	// bonus is terrain, not troops, so morale doesn't touch it).
	// Only the COMMITTED force adds offense; the defender fights with everything.
	// There is no jitter on the inputs: the variance lives inside the battle,
	// where BRE puts it, and rolling the strengths beforehand as well would
	// double-count it.
	ap := f.groundOffense(a) * moraleFactor(a.Morale) / 100
	dp := d.Defense()*moraleFactor(d.Morale)/100 + d.Land*LandDefenseBonus + w.allyDefenseBoost(d)

	// Fight it out. Both sides grind each other down until one has lost the share
	// of its force it is willing to lose, so the loser always pays the full
	// retreat share and the winner pays whatever the strength ratio cost it.
	attackerWins, aLoss, dLoss := w.battleAttrition(ap, dp, w.Config.AttackDamage.AttackRetreatPct())
	aloss := loseCommitted(a, f, aLoss)
	aloss.Bombers = bomberLoss // bombers fall to anti-air in the bombing run, not the ground clash
	dloss := loseForces(d, dLoss)
	w.bleedAllies(d, dLoss) // the allies' committed 30% bleeds at the defender's rate

	// A beaten defender's HeadQuarters is knocked back, and can be flattened by
	// repeated defeats. BINARY-VERIFIED (BRE.OVR 0xFFA2: Random(3)+5 subtracted
	// from the defender's HQ field, then clamped at zero). IB had no HQ damage
	// at all, which made a finished HeadQuarters permanent once built.
	if attackerWins && d.HQ > 0 {
		d.HQ -= HQBattleLossMin + w.rng.Intn(HQBattleLossJitter)
		if d.HQ < 0 {
			d.HQ = 0
		}
	}

	// Score (IB's own): the award scales with the forces used up in the battle.
	// The winner gains; the loser loses a bit less; a successful defense is worth
	// more than a successful attack.
	battle := aloss.Total() + dloss.Total()

	// Casualty lines list each side's losses by unit type (BRE shows the same
	// breakdown). The field order mirrors what each side fields: the attacker
	// commits troopers/jets/tanks/bombers, the defender holds troopers/turrets/
	// tanks/jets.
	attackerCas := func(u UnitLoss) string {
		return fmt.Sprintf("%d troopers, %d jets, %d tanks, %d bombers", u.Troopers, u.Jets, u.Tanks, u.Bombers)
	}
	defenderCas := func(u UnitLoss) string {
		return fmt.Sprintf("%d troopers, %d turrets, %d tanks, %d jets", u.Troopers, u.Turrets, u.Tanks, u.Jets)
	}

	if attackerWins {
		// BRE's Normal Attack yields LAND ONLY — "a successful assault brings you
		// extra regions" (breins.txt); no gold is plundered (that's the pirate-raid
		// path). One attack always captures the same SHARE of regions no matter how
		// lopsided the strength: a far stronger army takes ground faster over many
		// attacks, it does not annihilate an empire in a single blow.
		// min(the loser's land, max(floor, the Attack Rewards share of it)) — BRE's
		// own order of operations, with IB's net-worth density modifier folded into
		// the share before the floor is applied. The floor makes a decisive win on
		// a small realm take (up to) all of it.
		share := int64(w.Config.AttackRewards.AttackCapturePct()) * int64(w.captureDensityFactor(a, d))
		captured = int(int64(d.Land) * share / 10000)
		if captured < RegularAttackCaptureFloor {
			captured = RegularAttackCaptureFloor
		}
		if captured > d.Land {
			captured = d.Land
		}
		lost := d.Regions.remove(captured)
		d.syncLand()
		// taken is every region that changed hands — what both reports quote.
		// captured is the subset the winner still has to find a type for, which
		// is what the caller feeds to the picker.
		taken := lost.Total() // remove caps at the defender's land
		captured = taken
		if autoCapture {
			a.Regions.addMix(lost)
			a.syncLand()
		} else if lost.Waste > 0 {
			// Waste is not something a winner chooses to hold — it transfers as the
			// ruin it is, and only the clean land reaches the picker.
			a.Regions.Waste += lost.Waste
			a.syncLand()
			captured -= lost.Waste
		}

		gain := battle / CombatScoreDivisor
		addScore(a, gain)
		addScore(d, -gain*CombatLoserPenaltyPct/100)

		fmt.Fprint(&b, returningForces)
		fmt.Fprintf(&b, "Your casualties: %s.\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, "The enemy lost: %s.\n\n", defenderCas(dloss))
		fmt.Fprintf(&b, "Victory! You captured %d regions.\n", taken)
		if gain > 0 {
			fmt.Fprintf(&b, "Your score rose by %d.\n", gain)
		}
		// Total conquest only when the capture actually reduces the defender to
		// nothing — the final blow after grinding them down. You take their last
		// regions and seize the remains of their military (BRE's BRCRUSH.DSP).
		if d.Land <= 0 || d.People <= 0 {
			absorbMilitary(a, d)
			d.Alive = false
			d.DiedDay = w.GameDay
			fmt.Fprintf(&b, "\nYou crushed %s completely and seized the remains of its military!\n", d.Name)
		}
		d.addEvent(fmt.Sprintf("%s attacked you: you lost %d regions and %d units.", a.Name, taken, dloss.Total()))
		w.postCombatNews(a, d, true, !d.Alive)
	} else {
		gain := battle / CombatScoreDivisor * DefenseWinBonusPct / 100
		addScore(d, gain)
		addScore(a, -gain*CombatLoserPenaltyPct/100)

		fmt.Fprint(&b, returningForces)
		fmt.Fprintf(&b, "Your casualties: %s.\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, "The enemy lost: %s.\n\n", defenderCas(dloss))
		fmt.Fprint(&b, "Defeat! Your forces took the field and could not hold it.\n")
		d.addEvent(fmt.Sprintf("%s attacked you but was repelled. You lost %d units; your score rose by %d.", a.Name, dloss.Total(), gain))
		w.postCombatNews(a, d, false, false)
	}
	return b.String(), captured
}

// absorbMilitary transfers a conquered empire's surviving military to the
// conqueror — BRE's total-conquest reward ("you also get all the remains of
// your opponent's military", BRCRUSH.DSP). The loser is left with nothing.
func absorbMilitary(a, d *Empire) {
	a.Troopers += d.Troopers
	a.Jets += d.Jets
	a.Turrets += d.Turrets
	a.Tanks += d.Tanks
	a.Carriers += d.Carriers
	a.Bombers += d.Bombers
	d.Troopers, d.Jets, d.Turrets, d.Tanks, d.Carriers, d.Bombers = 0, 0, 0, 0, 0, 0
}

// battleAttrition fights a regular attack out and reports who won and what it
// cost each side, as a fraction of what that side brought.
//
// BINARY-VERIFIED against BRE's resolver (BRE.OVR 0xE81F). Both armies grind
// each other down a round at a time and the fight ends the moment EITHER side
// has lost the share it is willing to lose (retreatPct, the sysop's Attack
// Damage). That structure is the whole point: the side that breaks off has lost
// exactly the retreat share, while the other has lost only what the strength
// ratio cost it — so a lopsided attacker walks away almost intact and an evenly
// matched one pays nearly as much as the loser. IB used to hand the winner a
// flat 8% and the loser a flat 20%, which is roughly the even-match case applied
// to every battle.
//
// Each round a side is hit with a probability equal to its OPPONENT's share of
// the two strengths, and failing that on a flat BattleUpsetPct chance — so the
// weaker army still lands blows. Termination is not in doubt: the two hit
// probabilities sum to 1, so at least one side is hit with probability 0.77 or
// better every round, and about 30 hits carry a side to the deepest threshold.
func (w *World) battleAttrition(ap, dp, retreatPct int) (attackerWins bool, aLost, dLost float64) {
	survive := float64(100-retreatPct) / 100
	a0, d0 := float64(ap), float64(dp)
	a, d := a0, d0
	aFloor, dFloor := a0*survive, d0*survive
	for a > aFloor && d > dFloor {
		total := a + d
		if w.rng.Float64()*total < a || w.rng.Intn(100) < BattleUpsetPct {
			d = d*BattleRoundSurvival - BattleRoundFlatLoss
		}
		total = a + d // the second exchange sees the first one's damage
		if w.rng.Float64()*total < d || w.rng.Intn(100) < BattleUpsetPct {
			a = a*BattleRoundSurvival - BattleRoundFlatLoss
		}
	}
	return a > aFloor, lossFraction(a0, a), lossFraction(d0, d)
}

// lossFraction is how much of a starting strength was ground away, counting a
// side that was driven to nothing as a total loss.
func lossFraction(start, left float64) float64 {
	if left <= 0 || start <= 0 {
		return 1
	}
	return 1 - left/start
}

// UnitLoss is a per-type casualty breakdown, so a battle report can show each
// side's losses by unit type (as BRE does) instead of one lump total.
type UnitLoss struct {
	Troopers, Jets, Turrets, Tanks, Bombers int
}

// Total is the combined casualty count, for the score math and event lines that
// only care about the aggregate.
func (u UnitLoss) Total() int {
	return u.Troopers + u.Jets + u.Turrets + u.Tanks + u.Bombers
}

// loseForces removes the given fraction of an empire's combat units and returns
// the per-type breakdown lost (the defender fights with everything, so all four
// types bleed). The share is a fraction rather than whole percent because the
// winner of a lopsided battle walks away having lost well under one percent —
// rounding that to zero or one would erase the difference between a cheap win
// and an expensive one.
func loseForces(e *Empire, frac float64) UnitLoss {
	l := UnitLoss{
		Troopers: shareOf(e.Troopers, frac),
		Jets:     shareOf(e.Jets, frac),
		Turrets:  shareOf(e.Turrets, frac),
		Tanks:    shareOf(e.Tanks, frac),
	}
	e.Troopers -= l.Troopers
	e.Jets -= l.Jets
	e.Turrets -= l.Turrets
	e.Tanks -= l.Tanks
	return l
}

// AttackForce (defined in ibbs.go for group attacks — Troopers/Jets/Tanks/Bombers)
// doubles as a regular attack's committed detachment: BRE prompts "Send how many
// Troopers/Jets/Tanks/Bombers?" so a player can hold units back. Only the committed
// units add offense and only they take losses.

// FullForce commits every usable unit (jets capped at what the carriers can
// transport) — the AI's choice and the max the force-selection prompt offers.
func FullForce(e *Empire) AttackForce {
	return AttackForce{
		Troopers: e.Troopers,
		Jets:     min(e.Jets, e.Carriers*JetsPerCarrier),
		Tanks:    e.Tanks,
		Bombers:  e.Bombers,
	}
}

// clampTo trims the committed force to what e actually holds (jets to usable),
// so a stale or over-stated force can't attack with units the empire lacks.
func (f AttackForce) clampTo(e *Empire) AttackForce {
	usableJets := min(e.Jets, e.Carriers*JetsPerCarrier)
	return AttackForce{
		Troopers: clampInt(f.Troopers, 0, e.Troopers),
		Jets:     clampInt(f.Jets, 0, usableJets),
		Tanks:    clampInt(f.Tanks, 0, e.Tanks),
		Bombers:  clampInt(f.Bombers, 0, e.Bombers),
	}
}

// groundOffense is the committed force's regular-attack strength, mirroring
// Empire.Offense on the sent units (troopers 1, jets 2, tanks 3.5–4.5 by HQ), scaled by
// Technology. Bombers are excluded — they fly the bombing run, not the ground
// clash. (Distinct from offense(), which values a group-attack detachment flat.)
func (f AttackForce) groundOffense(e *Empire) int {
	sum := f.Troopers + f.Jets*2 + tankStrength(f.Tanks, e.HQ)
	return techRaise(sum, e.TechMilitaryFactor())
}

// loseCommitted removes pct% of the committed troopers/jets/tanks from e and
// returns the per-type breakdown — so holding units back keeps them out of harm's
// way. Bomber losses come from the bombing run, not here, and are folded in by
// the caller.
func loseCommitted(e *Empire, f AttackForce, frac float64) UnitLoss {
	l := UnitLoss{
		Troopers: shareOf(f.Troopers, frac),
		Jets:     shareOf(f.Jets, frac),
		Tanks:    shareOf(f.Tanks, frac),
	}
	e.Troopers -= l.Troopers
	e.Jets -= l.Jets
	e.Tanks -= l.Tanks
	return l
}

// shareOf is frac of n, rounded down and never more than n.
func shareOf(n int, frac float64) int {
	return clampInt(int(float64(n)*frac), 0, n)
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// addScore adjusts an empire's Score, never letting it fall below zero.
func addScore(e *Empire, n int) {
	e.Score += n
	if e.Score < 0 {
		e.Score = 0
	}
}

// jitter scales v by a random 0.8–1.2 factor.
func (w *World) jitter(v int) int {
	return int(float64(v) * (0.8 + w.rng.Float64()*0.4))
}
