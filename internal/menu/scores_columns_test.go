package menu

import (
	"strings"
	"testing"
)

// TestPrintScoresBREHeader checks the local scores board matches BRE's layout:
// the game-name banner, BRE's column labels/order (Id, Empire Name, Territory,
// Score, Net Worth), lettered [A]/[B] ids, and (dead) on an eliminated empire.
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
	plain := stripANSI(out)
	for _, want := range []string{"Immortal Barons", "Id", "Empire Name", "Territory", "Score", "Net Worth", "(dead)", "[A]"} {
		if !strings.Contains(plain, want) {
			t.Errorf("scores output missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(out, "Rank") || strings.Contains(out, "Land") {
		t.Errorf("scores output still uses old labels:\n%s", out)
	}
}
