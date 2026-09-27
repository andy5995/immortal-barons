package game

import (
	"fmt"
	"testing"
)

// A baron's share counts every detachment they have put into the strike, as the
// original's one slot per empire does: the capture that prompted this reported
// 6.06% for a join on top of jets committed earlier (cap/eots-ibbs-03.cap).
func TestGroupAttackShareCountsEveryDetachment(t *testing.T) {
	g := GroupAttack{Contributors: []Contribution{
		{Owner: "a", AttackForce: AttackForce{Troopers: 30_000}},
		{Owner: "b", AttackForce: AttackForce{Jets: 10_000}},
		{Owner: "b", AttackForce: AttackForce{Jets: 10_000}},
	}}
	// b: 40,000 of 70,000.
	if got := fmt.Sprintf("%.2f", g.SharePct("b")); got != "57.14" {
		t.Errorf("b's share = %s, want 57.14", got)
	}
	if got := fmt.Sprintf("%.2f", GroupAttack{}.SharePct("b")); got != "0.00" {
		t.Errorf("an empty strike = %s, want 0.00", got)
	}
}
