package game

import "testing"

// The sysop panel lists an in-flight force with exact counts, in the returning
// report's order, and names only the types it holds.
func TestAttackForceSummaryIsExact(t *testing.T) {
	f := AttackForce{Troopers: 31204, Bombers: 66110}
	if got, want := f.Summary(), "31,204 Troopers and 66,110 Bombers"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
	if got := (AttackForce{}).Summary(); got != "" {
		t.Errorf("an empty force summarized as %q", got)
	}
}
