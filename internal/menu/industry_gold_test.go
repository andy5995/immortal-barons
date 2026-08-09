package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Gold is the last row and starts at nothing, so an existing empire produces
// exactly what it did before the row existed.
func TestGoldIsTheLastIndustryRowAndStartsAtZero(t *testing.T) {
	if got := prodTypeNames[len(prodTypeNames)-1]; got != "Gold" {
		t.Errorf("last row = %q, want Gold", got)
	}
	if game.DefaultProdGoldPct != 0 {
		t.Errorf("default gold share = %d%%, want 0", game.DefaultProdGoldPct)
	}
	var e game.Empire
	if p := prodField(&e, len(prodTypeNames)-1); p != &e.ProdGold {
		t.Error("the Gold row is not wired to Empire.ProdGold")
	}
}

// Gold is a production row, not a unit, so it must not appear as a
// specialization — the efficiency modifier has no units to apply to.
func TestGoldCannotBeSpecialized(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("7\r")} // the position Gold occupies on Set Industries
	specializeIndustry(f, w)

	out := stripANSI(f.out.String())
	if strings.Contains(out, "7) Gold") {
		t.Error("Gold is offered as a specialization")
	}
	if got := w.Player().Specialized; got != "" {
		t.Errorf("Specialized = %q, want the choice refused", got)
	}
}

// The specialized row carries the marker; nothing else does, and the sentence
// that used to explain it is gone.
func TestSpecializedRowIsMarkedWithAsterisks(t *testing.T) {
	w := newWorld()
	if err := w.mutatePlayer(func(p *game.Empire) error {
		p.Specialized = "Tanks"
		p.Regions.Industrial = 100
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f := &fakeSession{keys: []rune("n\r")} // decline "Change Production?"
	setIndustries(f, w)
	out := f.out.String()

	for _, line := range strings.Split(out, "\n") {
		plain := stripANSI(line)
		marked := strings.Contains(plain, "* * *")
		if strings.HasPrefix(plain, "Tanks") != marked {
			t.Errorf("marker on the wrong row: %q", plain)
		}
	}
	if strings.Contains(stripANSI(out), "Specialized in") {
		t.Error("the explanatory sentence should be gone")
	}
}
