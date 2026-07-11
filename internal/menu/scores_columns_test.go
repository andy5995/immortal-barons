package menu

import (
	"strings"
	"testing"
)

// TestPrintScoresBREHeader checks the local scores board uses BRE.OVR's column
// labels/order (Id, Empire Name, Territory, Net Worth), marks the player's row,
// and flags an eliminated empire with (dead).
func TestPrintScoresBREHeader(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	w.With(func() {
		for _, e := range w.Empires {
			if e != w.Player() {
				e.Alive = false
				break
			}
		}
	})
	printScores(f, w)
	out := f.out.String()
	for _, want := range []string{"Id", "Empire Name", "Territory", "Net Worth", "(dead)", "->"} {
		if !strings.Contains(out, want) {
			t.Errorf("scores output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Rank") || strings.Contains(out, "Land") {
		t.Errorf("scores output still uses old labels:\n%s", out)
	}
}
