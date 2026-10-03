package game

import (
	"strings"
	"testing"
)

// A percentage of a large holding is taken in money width (pctOf). Plain int
// is 32 bits on the door builds, where count*pct passes 2^31 at about 21
// million and wrapped: mostly negative, which read as nothing lost, and past
// 2^32 a small positive slice. Run under GOARCH=386 to exercise it.
func TestPercentLossesOfHugeHoldingsDoNotWrap(t *testing.T) {
	const huge = 50_000_000
	w := testWorld()
	a := w.AddHuman("a", "Attacker")
	d := w.AddHuman("d", "Victim")
	landed := 0
	for i := 0; i < 400 && landed < 40; i++ {
		a.Agents, d.Agents = 1_000_000, huge
		d.People, d.Troopers, d.Tanks, d.Jets, d.Food = huge, huge, huge, huge, huge
		report := w.resolveBombEnemyTargets(a, d).English()
		if strings.Contains(report, "found nothing worth bombing") {
			t.Fatalf("every holding is %d, yet the bombing found nothing to destroy", huge)
		}
		if !strings.Contains(report, "destroying") {
			continue // the agent was caught
		}
		landed++
		for name, n := range map[string]int{"people": d.People, "troopers": d.Troopers,
			"agents": d.Agents, "tanks": d.Tanks, "jets": d.Jets, "food": d.Food} {
			// Every slot's band starts at 5% and none reaches 90%.
			if n < huge && (n > huge*95/100 || n < huge/10) {
				t.Fatalf("bombing left %s at %d of %d, outside every slot's band", name, n, huge)
			}
		}
	}
	if landed == 0 {
		t.Fatal("no bombing landed, so nothing was checked")
	}
	for i := 0; i < 20; i++ {
		d.Food = huge
		w.applyTerrorOp(TerrorOpBombFood, d)
		if d.Food > huge || d.Food < huge/2 {
			t.Fatalf("a terror bombing of %d food left %d", huge, d.Food)
		}
	}
}
