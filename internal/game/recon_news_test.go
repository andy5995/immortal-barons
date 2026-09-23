package game

import (
	"strings"
	"testing"
)

// A terror op's own spy report is filed without a news line, but only one per
// terror result: a genuine spy report on the same realm in the same packet still
// gets its line.
func TestReconNewsSkipsOnlyTheTerrorOpsOwnReports(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	p := Packet{
		Results: []AttackResult{{Kind: "terror", TargetBoard: "Far", TargetEmpire: "Ruritania"}},
		ReconReports: []SpyReport{
			{Board: "Far", Empire: "Ruritania"},
			{Board: "Far", Empire: "Ruritania"},
		},
	}

	w.fileReconReports(p)

	if len(w.SpyDatabase) != 2 {
		t.Errorf("filed %d reports, want both", len(w.SpyDatabase))
	}
	lines := 0
	for _, n := range w.NewsToday {
		if strings.Contains(n.Text, "reported back on Ruritania") {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("%d news lines for Ruritania, want 1 (the sweep's, not the terror op's)", lines)
	}
}
