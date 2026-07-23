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

	// A Normal attack costs the LOSER more than the WINNER (BRE live 2026-07-21,
	// two samples: loser ~20% of forces, winner ~8% — asymmetric, not the flat
	// 15% attack.hlp implies). Medium-setup values, scaled by AttackDamage; tune
	// here. The exact ratio-dependence isn't pinned down yet — treated as fixed.
	RegularAttackWinnerLossPct = 8
	RegularAttackLoserLossPct  = 20

	// A Normal attack captures max(RegularAttackCaptureFloor, capture% of the
	// loser's regions). Live BRE (2026-07-21, Attack Rewards=Medium, five points
	// 30-574 regions): ~10% of regions with a ~15-region floor — small realms lose
	// a bigger share, large ones ~10%, and the outcome is independent of the
	// strength ratio (verified 1.3x-4x). attack.hlp's "20%" is likely the High-
	// rewards value; Medium is ~10%. Medium-setup constants; tune here.
	RegularAttackCapturePct   = 10
	RegularAttackCaptureFloor = 15

	// LandDefenseBonus is the defensive strength each region adds on top of the
	// defender's units (terrain the attacker must take). Used in the battle math
	// and by the AI when judging whether a target is winnable (#36).
	LandDefenseBonus = 2
)

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

// CanAttack reports whether e may launch another individual (conventional)
// attack today. Config.MaxIndividualAttacks <= 0 means unlimited (matching the
// MaxRegions "<= 0 = no cap" convention).
func (w *World) CanAttack(e *Empire) bool {
	limit := w.Config.MaxIndividualAttacks
	return limit <= 0 || e.AttacksToday < limit
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
// composition). captured is 0 on a loss or an auto-captured win.
func (w *World) Attack(a, d *Empire, f AttackForce, autoCapture bool) (report string, captured int) {
	a.AttacksToday++ // counts against the daily individual-attack cap (both human and AI)
	f = f.clampTo(a) // only units the attacker actually holds can be committed
	var b strings.Builder
	fmt.Fprintf(&b, "%s attacks %s!\n\n", a.Name, d.Name)

	if kills, lost := w.bombingRun(a, d, f.Bombers); kills > 0 || lost > 0 {
		fmt.Fprintf(&b, "Your bombers hit the airfields: %d enemy jets destroyed", kills)
		if lost > 0 {
			fmt.Fprintf(&b, ", %d bombers lost to anti-air", lost)
		}
		fmt.Fprint(&b, ".\n\n")
	}

	// AttackDamage scales how many units both sides lose; AttackRewards scales
	// the winner's captured land and plunder. Medium = 100% = unchanged.
	dmg := w.Config.AttackDamage.Percent()
	rew := w.Config.AttackRewards.Percent()

	// Military morale scales each side's unit effectiveness (the land defense
	// bonus is terrain, not troops, so morale doesn't touch it).
	// Only the COMMITTED force adds offense; the defender fights with everything.
	ap := w.jitter(f.groundOffense(a) * moraleFactor(a.Morale) / 100)
	dp := w.jitter(d.Defense()*moraleFactor(d.Morale)/100 + d.Land*LandDefenseBonus)

	// BRE's Normal Attack (attack.hlp): the winner captures a share of the loser's
	// regions. Losses are ASYMMETRIC (BRE live): the LOSER bleeds ~20%, the WINNER
	// ~8%, both scaled by AttackDamage. The attacker's losses fall only on the
	// committed force (held-back units are safe). The winner is decided first so
	// each side takes its own loss rate.
	attackerWins := ap > dp
	winnerLoss := RegularAttackWinnerLossPct * dmg / 100
	loserLoss := RegularAttackLoserLossPct * dmg / 100
	aLoss, dLoss := loserLoss, winnerLoss
	if attackerWins {
		aLoss, dLoss = winnerLoss, loserLoss
	}
	aloss := loseCommitted(a, f, aLoss)
	dloss := loseForces(d, dLoss)

	// Score (IB's own): the award scales with the forces used up in the battle.
	// The winner gains; the loser loses a bit less; a successful defense is worth
	// more than a successful attack.
	battle := aloss + dloss

	if attackerWins {
		// BRE's Normal Attack yields LAND ONLY — "a successful assault brings you
		// extra regions" (breins.txt); no gold is plundered (that's the pirate-raid
		// path). One attack always captures the same SHARE of regions no matter how
		// lopsided the strength: a far stronger army takes ground faster over many
		// attacks, it does not annihilate an empire in a single blow.
		// max(floor, capture% of the loser's land), then scaled by Attack Rewards.
		// The floor makes a decisive win on a small realm take (up to) all of it.
		captured = d.Land * RegularAttackCapturePct / 100
		if captured < RegularAttackCaptureFloor {
			captured = RegularAttackCaptureFloor
		}
		captured = captured * rew / 100
		if captured > d.Land {
			captured = d.Land
		}
		lost := d.Regions.remove(captured)
		d.syncLand()
		captured = lost.Total() // the actual count taken (remove caps at the defender's land)
		if autoCapture {
			a.Regions.addMix(lost)
			a.syncLand()
		}

		gain := battle / CombatScoreDivisor
		addScore(a, gain)
		addScore(d, -gain*CombatLoserPenaltyPct/100)

		fmt.Fprintf(&b, "Victory! You captured %d regions.\n", captured)
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
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
		d.Events = append(d.Events, fmt.Sprintf("%s attacked you: you lost %d regions and %d units.", a.Name, captured, dloss))
		w.postCombatNews(a, d, true, !d.Alive)
	} else {
		gain := battle / CombatScoreDivisor * DefenseWinBonusPct / 100
		addScore(d, gain)
		addScore(a, -gain*CombatLoserPenaltyPct/100)

		fmt.Fprintf(&b, "Defeat! Your forces returned exhausted.\n")
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
		d.Events = append(d.Events, fmt.Sprintf("%s attacked you but was repelled. You lost %d units; your score rose by %d.", a.Name, dloss, gain))
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

// loseForces removes pct% of an empire's combat units and returns the
// total number lost.
func loseForces(e *Empire, pct int) int {
	t := e.Troopers * pct / 100
	j := e.Jets * pct / 100
	u := e.Turrets * pct / 100
	k := e.Tanks * pct / 100
	e.Troopers -= t
	e.Jets -= j
	e.Turrets -= u
	e.Tanks -= k
	return t + j + u + k
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
// Empire.Offense on the sent units (troopers 1, jets 2, tanks 4×HQ), scaled by
// Technology. Bombers are excluded — they fly the bombing run, not the ground
// clash. (Distinct from offense(), which values a group-attack detachment flat.)
func (f AttackForce) groundOffense(e *Empire) int {
	sum := f.Troopers + f.Jets*2 + f.Tanks*4*(100+e.HQ)/100
	return sum * (100 + e.TechFactor()) / 100
}

// loseCommitted removes pct% of the committed troopers/jets/tanks from e (bombers
// are spent in the bombing run) and returns the total lost — so holding units
// back keeps them out of harm's way.
func loseCommitted(e *Empire, f AttackForce, pct int) int {
	t := f.Troopers * pct / 100
	j := f.Jets * pct / 100
	k := f.Tanks * pct / 100
	e.Troopers -= t
	e.Jets -= j
	e.Tanks -= k
	return t + j + k
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
