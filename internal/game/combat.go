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

	// RegularAttackLossPct is how much of each side's forces a regular (Normal)
	// attack costs — both attacker and defender — per BRE's attack.hlp ("both
	// sides fight until they suffer 15% losses, at which time they retreat").
	RegularAttackLossPct = 15
)

// bombingRun sends a's bombers against d's airfields before the ground
// clash. It destroys grounded jets (which don't defend anyway, so this
// only weakens d's future offense and net worth) and costs a some bombers
// to anti-air. It mutates both empires and returns (jetsDestroyed,
// bombersLost).
func (w *World) bombingRun(a, d *Empire) (int, int) {
	if a.Bombers <= 0 || d.Jets <= 0 {
		return 0, 0
	}
	lost := min(a.Bombers, d.Turrets/TurretsPerBomberDown)
	survivors := a.Bombers - lost
	kills := survivors * BomberJetKills * (100 - d.SDI) / 100
	if kills > d.Jets {
		kills = d.Jets
	}
	d.Jets -= kills
	a.Bombers -= lost
	return kills, lost
}

// Attack resolves a battle between attacker a and defender d, mutating
// both empires, and returns a battle report. The attacker commits its
// offense; the defender fights with its defense plus a home bonus from its
// land. Both sides apply a random factor.
func (w *World) Attack(a, d *Empire) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s attacks %s!\n\n", a.Name, d.Name)

	if kills, lost := w.bombingRun(a, d); kills > 0 || lost > 0 {
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

	ap := w.jitter(a.Offense())
	dp := w.jitter(d.Defense() + d.Land*2)

	// BRE's Normal Attack (attack.hlp): the winner captures 20% of the loser's
	// regions, and "both sides fight until they suffer 15% losses, at which
	// time they retreat" — the loss is symmetric regardless of who wins.
	loss := RegularAttackLossPct * dmg / 100
	aloss := loseForces(a, loss)
	dloss := loseForces(d, loss)

	if ap > dp {
		captured := (d.Land/5 + 1) * rew / 100
		if captured > d.Land {
			captured = d.Land
		}
		plunder := d.Gold / 4 * rew / 100
		lost := d.Regions.remove(captured)
		d.syncLand()
		a.Regions.addMix(lost)
		a.syncLand()
		d.Gold -= plunder
		a.Gold += plunder

		fmt.Fprintf(&b, "Victory! You captured %d regions and plundered %d gold.\n", captured, plunder)
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
		if d.Land <= 0 || d.People <= 0 {
			d.Alive = false
			fmt.Fprintf(&b, "\n%s has been utterly conquered!\n", d.Name)
		}
		d.Events = append(d.Events, fmt.Sprintf("%s attacked you: you lost %d regions, %d gold, and %d units.", a.Name, captured, plunder, dloss))
		w.postCombatNews(a, d, true, !d.Alive)
	} else {
		fmt.Fprintf(&b, "Defeat! Your forces returned exhausted.\n")
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
		d.Events = append(d.Events, fmt.Sprintf("%s attacked you but was repelled. You lost %d units.", a.Name, dloss))
		w.postCombatNews(a, d, false, false)
	}
	return b.String()
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

// jitter scales v by a random 0.8–1.2 factor.
func (w *World) jitter(v int) int {
	return int(float64(v) * (0.8 + w.rng.Float64()*0.4))
}
