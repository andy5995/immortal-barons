package game

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

const (
	// JetsPerCarrier is how many jets one carrier can transport to a battle; jets
	// beyond that are grounded (not "usable" — BRE's Offense/attack force screen).
	JetsPerCarrier = 100

	// A Normal attack captures max(RegularAttackCaptureFloor, the Attack Rewards
	// share of the loser's regions), capped at what the loser holds. The floor is
	// BINARY-VERIFIED: BRE.OVR 0x1009f pushes 15 into the max, 0x100c3 the
	// defender's own region count into the min. The share itself is a per-level
	// table — see AttackCaptureMediumPct in balance.go.
	RegularAttackCaptureFloor = 15

	// LandDefenseBonus is how much a region adds when the AI sizes up a rival --
	// cheap, lightly-held land looks softer. IB's own, and a playtest knob.
	//
	// It is deliberately NOT in the battle math any more. It was until
	// 2026-08-24, and two independent lines say the original has no per-region
	// defence: its defence builder sums troopers, turrets and tanks and never
	// reads a region count, and a live capture discriminates -- 112 turrets
	// losing exactly 2 to a 3-jet attack matches a land-free defence, where any
	// per-region bonus makes that figure odd for every possible round count.
	LandDefenseBonus = 2
)

// returningForces opens a battle report. BRE puts the casualties first and the
// verdict last, under a line about the army coming home worn out, and it prints
// the same opening whether the attack won or lost — so a player reads what the
// fight cost before learning how it went. The wording is IB's own; only the
// order and the per-unit breakdown are the original's (docs/dev/bre-screens.md).
const returningForces = "Your forces have returned from the field, exhausted."

// CanAttack reports whether e may launch another individual (conventional)
// attack today. Config.MaxIndividualAttacks <= 0 means unlimited (matching the
// MaxRegions "<= 0 = no cap" convention).
func (w *World) CanAttack(e *Empire) bool {
	return underDailyCap(e.AttacksToday, w.Config.MaxIndividualAttacks)
}

// LocalAttacksAllowed reports whether barons on this board may attack each
// other. The Local Attacks switch is a league setting and only bites in a league
// game — BRE's help scopes it to "the ability to attack other local empires in
// Interplanetary games" — so a stand-alone board always fights.
func (w *World) LocalAttacksAllowed() bool {
	return !w.Config.IBBS || w.Config.LocalAttacks
}

// localAttacksScore reports whether winning a local attack moves either side's
// score. BRE's Local Attack Scoring is off by default so barons cannot farm
// score off their neighbours; off a league it never applies.
func (w *World) localAttacksScore() bool {
	return !w.Config.IBBS || w.Config.LocalAttackScoring
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
	o := w.AttackDetailed(a, d, f, autoCapture)
	return o.Report, o.Captured
}

// BattleOutcome is one battle in structured form, for a caller that needs the
// figures and not only the finished report. The menu stages an attack's
// casualties across a few seconds before showing the whole report, which it
// cannot do from prose it would have to parse back apart.
type BattleOutcome struct {
	Report       string
	Captured     int
	AttackerLoss UnitLoss
	DefenderLoss UnitLoss
}

// AttackDetailed is Attack with the figures kept.
func (w *World) AttackDetailed(a, d *Empire, f AttackForce, autoCapture bool) BattleOutcome {
	var captured int
	a.AttacksToday++ // counts against the daily individual-attack cap (both human and AI)
	f = f.clampTo(a) // only units the attacker actually holds can be committed
	// Attacking a realm you hold an agreement with tears the agreement up and
	// costs a quarter of both support and morale (BreachTreaty). The menu asks
	// first and charges it there, so this is normally a no-op by the time the
	// battle starts; it stands for the AI and for any caller that did not ask.
	// Either way it runs before the strengths are read, so the battle is fought
	// at the reduced morale and the alliance reinforcement below sees the break.
	w.BreachTreaty(a, d)
	var b strings.Builder
	// The report is written in the ATTACKER's language: it is what that player
	// reads at the end of their own turn. The defender's copy is filed as an
	// event further down and translated into THEIR language instead, because the
	// two players need not share one.
	tr := func(msgid string) string { return i18n.T(a.Language, msgid) }

	// Full Defense Alliance: each of the defender's partners sends 30% of its
	// troopers and tanks to reinforce the defense (BRE-verified). Reported to the
	// attacker, matching BRE's "the empire's allies send N Troopers and M Tanks".
	allyTroopers, allyTanks := 0, 0
	for _, ally := range w.alliesOf(d, fullDefenseAlliance) {
		allyTroopers += ally.Troopers * AllyDefenseContribPct / 100
		allyTanks += ally.Tanks * AllyDefenseContribPct / 100
	}
	if allyTroopers+allyTanks > 0 {
		fmt.Fprintf(&b, tr("%s's allies send %d troopers and %d tanks to aid the defense.")+"\n\n", d.Name, allyTroopers, allyTanks)
	}

	// Military morale scales each side's unit effectiveness (the land defense
	// bonus is terrain, not troops, so morale doesn't touch it).
	// Only the COMMITTED force adds offense; the defender fights with everything.
	// There is no jitter on the inputs: the variance lives inside the battle,
	// where BRE puts it, and rolling the strengths beforehand as well would
	// double-count it.
	ap := float64(f.groundOffense(a)) * float64(moraleFactor(a.Morale)) / 100
	dp := float64(d.Defense())*float64(moraleFactor(d.Morale))/100 + float64(w.allyDefenseBoost(d))

	// Fight it out. Both sides grind each other down until one has lost the share
	// of its force it is willing to lose, so the loser always pays the full
	// retreat share and the winner pays whatever the strength ratio cost it.
	attackerWins, aLoss, dLoss := w.battleAttrition(ap, dp, w.Config.AttackDamage.AttackRetreatPct())
	aloss := loseCommitted(a, f, aLoss)
	dloss := loseForces(d, dLoss)
	w.bleedAllies(a, d, dLoss) // the allies' committed 30% bleeds at the defender's rate, and each is told

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

	// Casualty lines list each side's losses by unit type (BRE shows the same
	// breakdown). The field order mirrors what each side fields: the attacker
	// commits troopers/jets/tanks/bombers, the defender holds troopers/turrets/
	// tanks/jets.
	attackerCas := func(u UnitLoss) string {
		return fmt.Sprintf(tr("%d troopers, %d jets, %d tanks, %d bombers"), u.Troopers, u.Jets, u.Tanks, u.Bombers)
	}
	defenderCas := func(u UnitLoss) string { return defenderCasIn(a.Language, u) }

	if attackerWins {
		// BRE's Normal Attack yields LAND ONLY — "a successful assault brings you
		// extra regions" (breins.txt); no gold is plundered (that's the pirate-raid
		// path). One attack always captures the same SHARE of regions no matter how
		// lopsided the strength: a far stronger army takes ground faster over many
		// attacks, it does not annihilate an empire in a single blow.
		// min(the loser's land, max(floor, the Attack Rewards share of it)) — BRE's
		// own chain, BINARY-VERIFIED: total_regions x pct / 100, truncated, then
		// max against the floor and min against the land. The floor makes a
		// decisive win on a small realm take (up to) all of it. IB multiplied a
		// net-worth density factor into the share until #200; the original reads
		// the defender's region count and the level constant and nothing else.
		captured = int(int64(d.Land) * int64(w.Config.AttackRewards.AttackCapturePct()) / 100)
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

		// Score is per REGION taken, and a total conquest pays better than an
		// ordinary win — BRE's two award sites in resolve_regular_attack, told
		// apart by the report each one follows. Nothing is awarded for losing and
		// nothing is taken from the loser: the original writes only the acting
		// player's Score field, ever.
		// A total conquest is the capture taking every region, and nothing else:
		// the original's BRCRUSH path is reached from the land test alone. IB also
		// crowned one when the defender was left with no people, until #200.
		crushed := d.Land <= 0
		gain := 0
		if w.localAttacksScore() {
			per := CombatScoreWinPerRegion
			if crushed {
				per = CombatScoreCrushPerRegion
			}
			gain = taken * per
		}
		addScore(a, gain)

		fmt.Fprint(&b, tr(returningForces)+"\n")
		fmt.Fprintf(&b, tr("Your casualties: %s.")+"\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, tr("The enemy lost: %s.")+"\n\n", defenderCas(dloss))
		fmt.Fprintf(&b, tr("Victory! You captured %d regions.")+"\n", taken)
		if gain > 0 {
			fmt.Fprintf(&b, tr("Your score rose by %d.")+"\n", gain)
		}
		// Total conquest only when the capture actually reduces the defender to
		// nothing — the final blow after grinding them down. You take their last
		// regions and seize the remains of their military (BRE's BRCRUSH.DSP).
		if crushed {
			absorbMilitary(a, d)
			w.Kill(d)
			fmt.Fprintf(&b, "\n"+tr("You crushed %s completely and seized the remains of its military!")+"\n", d.Name)
		}
		d.addEvent(fmt.Sprintf(i18n.T(d.Language, "%s attacked you and took %d regions. You lost %s."),
			a.Name, taken, defenderCasIn(d.Language, dloss)))
		w.postCombatNews(a, d, true, !d.Alive)
	} else {
		// A repelled attack scores nothing for either side. The original's only two
		// Score writes are on the winning-ATTACK paths, and neither touches a
		// second realm's record — a successful defence is its own reward.

		fmt.Fprint(&b, tr(returningForces)+"\n")
		fmt.Fprintf(&b, tr("Your casualties: %s.")+"\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, tr("The enemy lost: %s.")+"\n\n", defenderCas(dloss))
		fmt.Fprint(&b, tr("Defeat! Your forces were beaten off the field.")+"\n")
		d.addEvent(fmt.Sprintf(i18n.T(d.Language, "%s attacked you and was repelled. You lost %s."),
			a.Name, defenderCasIn(d.Language, dloss)))
		w.postCombatNews(a, d, false, false)
	}
	return BattleOutcome{
		Report:       b.String(),
		Captured:     captured,
		AttackerLoss: aloss,
		DefenderLoss: dloss,
	}
}

// defenderCasIn lists a defender's losses by unit type in lang — the attacker's
// report and the defender's own event both read it, in their own languages. A
// total on its own ("lost N units") is what the defender's event used to get,
// and it is not what a player wants to know after a battle.
func defenderCasIn(lang string, u UnitLoss) string {
	return fmt.Sprintf(i18n.T(lang, "%d troopers, %d turrets, %d tanks, %d jets"), u.Troopers, u.Turrets, u.Tanks, u.Jets)
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
// The strengths arrive as float64 because the original keeps them in Real48 the
// whole way. Truncating them to integers first is not a rounding detail: a live
// capture had 3 jets at full morale attack 112 turrets, and BRE reported ZERO
// jets lost where a truncated 6.6 -> 6 loses one (see TestCapturedJetsVersusTurrets).
//
// Verified in the running binary, not only by decode: six staged battles (763
// troopers + 525 tanks + 140 bombers against a 1.35M-unit defender) repelled
// the attacker at ~20.5% every time while the defender lost nothing in three,
// ~1% in one and ~2% in two — the upset roll's 1%-per-hit quanta
// (cap/small-vs-large-20260830.cap, 2026-08-30). A token force stripping
// thousands of units off a giant is the original's behaviour, not a defect.
func (w *World) battleAttrition(ap, dp float64, retreatPct int) (attackerWins bool, aLost, dLost float64) {
	survive := float64(100-retreatPct) / 100
	a0, d0 := ap, dp
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

// remoteBattleAttrition fights an ARRIVING interplanetary strike. It is
// battleAttrition's sibling, not a caller of it, because BRE gives the two
// resolvers different loops (see RemoteJetBomberWeight): there is no upset roll
// here, and the defender's jets fight their own battle against the attacker's
// bombers alongside the ground one.
//
// BINARY-VERIFIED against BRE.OVR 0x03f4a0 +0x0647: the exit tests at +0x068d
// and +0x06b7, the three rolls at +0x06e1, +0x0744 and +0x07cb with their hits
// at +0x0718, +0x079f and +0x0802, the clamps at +0x082e/+0x0853/+0x0878, the
// fractions written out at +0x08e7 and +0x094a, and the outcome at +0x08c0.
//
// The fractions are OUTCOMES of the fight. retreatPct is only where the loop
// stops, which is what keeps a token force from hurting anybody: a strike far
// weaker than the defence reaches its own threshold almost at once, so the
// defender's fraction comes out near zero. Handing the defender retreatPct
// outright instead is what let a 1,100-unit strike destroy 7,696 units (#199).
func (w *World) remoteBattleAttrition(ap, dp, jets, bombers, retreatPct int) (attackerWins bool, aLost, dLost, jetLost float64) {
	survive := float64(100-retreatPct) / 100
	a0, d0, j0 := float64(ap), float64(dp), float64(jets)
	a, d, j := a0, d0, j0
	air := float64(bombers) * RemoteJetBomberWeight
	aFloor, dFloor := a0*survive, d0*survive
	for a > aFloor && d > dFloor {
		total := a + d
		if w.rng.Float64()*total < a {
			d = max(0, d*BattleRoundSurvival-BattleRoundFlatLoss)
		}
		// The air battle is decided by the bombers alone and never feeds back
		// into the ground one; with no bombers sent, air is 0 and no jet dies.
		if air > 0 && w.rng.Float64()*(air+j) < air {
			j = max(0, j*BattleRoundSurvival-BattleRoundFlatLoss)
		}
		total = a + d // the second exchange sees the first one's damage
		if w.rng.Float64()*total < d {
			a = max(0, a*BattleRoundSurvival-BattleRoundFlatLoss)
		}
	}
	return a > aFloor, lossFraction(a0, a), lossFraction(d0, d), lossFraction(j0, j)
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

// loseForcesSplit removes a fraction of an empire's combat units and returns the
// per-type breakdown lost (the defender fights with everything, so all four
// types bleed). Jets take the air fraction; troopers, turrets and tanks take the
// ground one. An interplanetary battle sets the two apart — see
// remoteBattleAttrition.
//
// The shares are fractions rather than whole percent because the winner of a
// lopsided battle walks away having lost well under one percent, and rounding
// that to zero or one would erase the difference between a cheap win and an
// expensive one.
func loseForcesSplit(e *Empire, ground, air float64) UnitLoss {
	l := UnitLoss{
		Troopers: shareOf(e.Troopers, ground),
		Jets:     shareOf(e.Jets, air),
		Turrets:  shareOf(e.Turrets, ground),
		Tanks:    shareOf(e.Tanks, ground),
	}
	e.Troopers -= l.Troopers
	e.Jets -= l.Jets
	e.Turrets -= l.Turrets
	e.Tanks -= l.Tanks
	return l
}

// loseForces is loseForcesSplit for a battle fought on one planet, where the
// ground and the air bleed at the same rate.
func loseForces(e *Empire, frac float64) UnitLoss {
	return loseForcesSplit(e, frac, frac)
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
		Troopers: min(max(f.Troopers, 0), e.Troopers),
		Jets:     min(max(f.Jets, 0), usableJets),
		Tanks:    min(max(f.Tanks, 0), e.Tanks),
		Bombers:  min(max(f.Bombers, 0), e.Bombers),
	}
}

// groundOffense is the committed force's regular-attack strength, mirroring
// Empire.Offense on the sent units (troopers 1, jets 2, tanks 3.5–4.5 by HQ), scaled by
// Technology. Bombers are excluded: the original's local resolver keeps them out
// of the offence sum (docs/mechanics-reference.md) though it bleeds them with
// the rest. (Distinct from offense(), which values a group-attack detachment flat.)
func (f AttackForce) groundOffense(e *Empire) int {
	sum := f.Troopers + f.Jets*2 + tankStrength(f.Tanks, e.HQ)
	return techRaise(sum, e.TechMilitaryFactor())
}

// loseCommitted removes frac of the committed force from e, every type at the
// SAME fraction, and returns the per-type breakdown — so holding units back
// keeps them out of harm's way. BINARY-VERIFIED (resolve_regular_attack, proc
// +0x1581..+0x1631): one Real48 loss fraction is multiplied into each of the
// four committed counts in turn and subtracted from troopers, jets, tanks and
// bombers alike. Bombers add no offence locally but bleed with the rest; IB
// used to fly them on a bombing run of its own instead, with anti-air losses
// and grounded-jet kills the original has no trace of (#200).
func loseCommitted(e *Empire, f AttackForce, frac float64) UnitLoss {
	l := UnitLoss{
		Troopers: shareOf(f.Troopers, frac),
		Jets:     shareOf(f.Jets, frac),
		Tanks:    shareOf(f.Tanks, frac),
		Bombers:  shareOf(f.Bombers, frac),
	}
	e.Troopers -= l.Troopers
	e.Jets -= l.Jets
	e.Tanks -= l.Tanks
	e.Bombers -= l.Bombers
	return l
}

// shareOf is frac of n, rounded down and never more than n.
func shareOf(n int, frac float64) int {
	return min(max(int(float64(n)*frac), 0), n)
}

// addScore adjusts an empire's Score, never letting it fall below zero.
func addScore(e *Empire, n int) {
	e.Score += n
	if e.Score < 0 {
		e.Score = 0
	}
}
