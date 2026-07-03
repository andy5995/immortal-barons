package game

import (
	"fmt"
	"strings"
)

// Attack resolves a battle between attacker a and defender d, mutating
// both empires, and returns a battle report. Defender fights with a home
// advantage proportional to its land. Both sides apply a random factor.
func (w *World) Attack(a, d *Empire) string {
	ap := w.jitter(a.Power())
	dp := w.jitter(d.Power() + d.Land*2)

	var b strings.Builder
	fmt.Fprintf(&b, "%s attacks %s!\n\n", a.Name, d.Name)

	if ap > dp {
		captured := d.Land/5 + 1
		if captured > d.Land {
			captured = d.Land
		}
		aloss := a.Troopers * 15 / 100
		dloss := d.Troopers * 30 / 100
		plunder := d.Gold / 4

		d.Land -= captured
		a.Land += captured
		a.Troopers -= aloss
		d.Troopers -= dloss
		d.Gold -= plunder
		a.Gold += plunder

		fmt.Fprintf(&b, "Victory! You captured %d regions and plundered %d gold.\n", captured, plunder)
		fmt.Fprintf(&b, "You lost %d troopers; the enemy lost %d.\n", aloss, dloss)
		if d.Land <= 0 || d.People <= 0 {
			d.Alive = false
			fmt.Fprintf(&b, "\n%s has been utterly conquered!\n", d.Name)
		}
	} else {
		aloss := a.Troopers * 25 / 100
		dloss := d.Troopers * 10 / 100
		a.Troopers -= aloss
		d.Troopers -= dloss
		fmt.Fprintf(&b, "Defeat! Your forces returned exhausted.\n")
		fmt.Fprintf(&b, "You lost %d troopers; the enemy lost %d.\n", aloss, dloss)
	}
	return b.String()
}

// jitter scales v by a random 0.8–1.2 factor.
func (w *World) jitter(v int) int {
	return int(float64(v) * (0.8 + w.rng.Float64()*0.4))
}
