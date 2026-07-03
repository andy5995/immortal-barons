package game

import (
	"fmt"
	"strings"
)

// Attack resolves a battle between attacker a and defender d, mutating
// both empires, and returns a battle report. The attacker commits its
// offense; the defender fights with its defense plus a home bonus from its
// land. Both sides apply a random factor.
func (w *World) Attack(a, d *Empire) string {
	ap := w.jitter(a.Offense())
	dp := w.jitter(d.Defense() + d.Land*2)

	var b strings.Builder
	fmt.Fprintf(&b, "%s attacks %s!\n\n", a.Name, d.Name)

	if ap > dp {
		captured := d.Land/5 + 1
		if captured > d.Land {
			captured = d.Land
		}
		plunder := d.Gold / 4
		d.Land -= captured
		a.Land += captured
		d.Gold -= plunder
		a.Gold += plunder

		aloss := loseForces(a, 15)
		dloss := loseForces(d, 30)

		fmt.Fprintf(&b, "Victory! You captured %d regions and plundered %d gold.\n", captured, plunder)
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
		if d.Land <= 0 || d.People <= 0 {
			d.Alive = false
			fmt.Fprintf(&b, "\n%s has been utterly conquered!\n", d.Name)
		}
	} else {
		aloss := loseForces(a, 25)
		dloss := loseForces(d, 10)
		fmt.Fprintf(&b, "Defeat! Your forces returned exhausted.\n")
		fmt.Fprintf(&b, "You lost %d units; the enemy lost %d.\n", aloss, dloss)
	}
	d.Events = append(d.Events, "While you were away: "+b.String())
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
