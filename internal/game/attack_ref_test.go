package game

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// refAttack is World.Attack as it was before the conventional effect sets,
// kept verbatim as the reference the base rules must still reproduce.
func refAttack(w *World, a, d *Empire, f AttackForce) (report string, captured int) {
	// No daily counter here: the local attack is limited by the turn, not by a
	// per-day allowance (see CanAttack). AttacksToday counts interplanetary
	// individual strikes alone.
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
		allyTroopers += allySent(ally.Troopers)
		allyTanks += allySent(ally.Tanks)
	}
	// GroupLong, not bare and not Format: the original groups THIS line past four
	// digits while leaving the casualty lines on the same screen bare at any size
	// — a live capture has it sending "8636 Tanks" and, in another battle,
	// "11,508 Troopers" and "248,073,416 Tanks", beside "You lost 81294 Jets".
	// The spelling is chosen per line here, not per screen.
	if allyTroopers+allyTanks > 0 {
		fmt.Fprintf(&b, tr("%s's allies send %s troopers and %s tanks to aid the defense.")+"\n\n",
			d.Name, numfmt.GroupLong(allyTroopers, a.Language), numfmt.GroupLong(allyTanks, a.Language))
	}

	// Military morale scales each side's unit effectiveness (the land defense
	// bonus is terrain, not troops, so morale doesn't touch it).
	// Only the COMMITTED force adds offense; the defender fights with everything.
	// There is no jitter on the inputs: the variance lives inside the battle,
	// where BRE puts it, and rolling the strengths beforehand as well would
	// double-count it.
	ap := float64(w.groundOffense(f, a)) * float64(moraleFactor(a.Morale)) / 100
	dp := float64(w.Defense(d))*float64(moraleFactor(d.Morale))/100 + float64(w.allyDefenseBoost(d))

	// Fight it out. Both sides grind each other down until one has lost the share
	// of its force it is willing to lose, so the loser always pays the full
	// retreat share and the winner pays whatever the strength ratio cost it.
	attackerWins, aLoss, dLoss := w.battleAttrition(ap, dp, w.Config.AttackDamage.AttackRetreatPct())
	aloss := loseCommitted(a, f, aLoss)
	dloss := loseForces(d, dLoss)
	w.bleedAllies(a, d, dLoss, aloss) // the allies' committed 30% bleeds at the defender's rate, and each is told

	// A beaten defender's HeadQuarters is knocked back, and can be flattened by
	// repeated defeats. BINARY-VERIFIED (BRE.OVR 0xFFA2: Random(3)+5 subtracted
	// from the defender's HQ field, then clamped at zero). IB had no HQ damage
	// at all, which made a finished HeadQuarters permanent once built.
	if attackerWins && d.HQ > 0 {
		hq := w.rules().HQ
		d.HQ -= hq.BattleLossMin + w.rng.Intn(hq.BattleLossJitter)
		if d.HQ < 0 {
			d.HQ = 0
		}
	}

	// Casualty lines list each side's losses by unit type (BRE shows the same
	// breakdown). The field order mirrors what each side fields: the attacker
	// commits troopers/jets/tanks/bombers, the defender holds troopers/turrets/
	// tanks/jets.
	attackerCas := func(u UnitLoss) string { return attackerCasualties(u).In(a.Language) }
	defenderCasIn := func(u UnitLoss) string { return defenderCas(u).In(a.Language) }

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
		captured = d.Regions.remove(captured).Total() // remove caps at the defender's land
		d.syncLand()
		// Placed in this same save as Coastal, where the AI keeps it; a player's
		// session parks it untyped for the picker (ParkUntyped).
		a.Regions.Coastal += captured
		a.syncLand()

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
			gain = captured * per
		}
		addScore(a, gain)

		fmt.Fprint(&b, tr(returningForces)+"\n")
		fmt.Fprintf(&b, tr("Your casualties: %s.")+"\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, tr("The enemy lost: %s.")+"\n\n", defenderCasIn(dloss))
		fmt.Fprintf(&b, tr("Victory! You captured %d regions.")+"\n", captured)
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
		d.addEvent(sentences(
			say("{who} attacked you and took {n} regions. You lost {lost}.",
				"who", a.Name, "n", captured, "lost", defenderCas(dloss)),
			say("{who} lost {lost}.", "who", a.Name, "lost", attackerCasualties(aloss))))
		w.postCombatNews(a, d, true, !d.Alive, captured)
	} else {
		// A repelled attack scores nothing for either side. The original's only two
		// Score writes are on the winning-ATTACK paths, and neither touches a
		// second realm's record — a successful defense is its own reward.

		fmt.Fprint(&b, tr(returningForces)+"\n")
		fmt.Fprintf(&b, tr("Your casualties: %s.")+"\n\n", attackerCas(aloss))
		fmt.Fprintf(&b, tr("The enemy lost: %s.")+"\n\n", defenderCasIn(dloss))
		fmt.Fprint(&b, tr("Defeat! Your forces were beaten off the field.")+"\n")
		// A beaten attacker's army loses heart (LostAttackMoraleMin). Silent, as
		// in the original: the loss report says nothing about morale.
		a.Morale -= LostAttackMoraleMin + w.rng.Intn(LostAttackMoraleJitter)
		if a.Morale < 0 {
			a.Morale = 0
		}
		d.addEvent(sentences(
			say("{who} attacked you and was repelled. You lost {lost}.",
				"who", a.Name, "lost", defenderCas(dloss)),
			say("{who} lost {lost}.", "who", a.Name, "lost", attackerCasualties(aloss))))
		w.postCombatNews(a, d, false, false, 0)
	}
	return b.String(), captured
}
