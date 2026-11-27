package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// A mod's faction names are what the Attack Pirates screen lists and what the
// news highlights, and the colors still go by slot.
func TestModdedFactionNamesReachTheScreens(t *testing.T) {
	w := newWorld()
	w.Rules = game.BaseRules()
	w.Rules.PirateFigures.Names = []string{"Ash", "Bone", "Crow", "Dusk", "Ember", "Flint", "Gale", "Hollow", "Iron"}
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	w.Pirates = nil
	w.EnsurePirates()
	w.Player().Protection = 0

	f := &fakeSession{keys: []rune("0\r")}
	attackPirates(f, w)
	out := f.out.String()
	if want := "1) " + ansi.FgBrightGreen + "Ash" + ansi.Reset + "\n"; !strings.Contains(out, want) {
		t.Errorf("slot 0 should list as Ash in its own color; output:\n%s", out)
	}
	if strings.Contains(out, "Humans") {
		t.Errorf("the original name is still shown:\n%s", out)
	}

	var seen []string
	for _, term := range newsHighlightTerms(w) {
		seen = append(seen, term.text)
	}
	if !strings.Contains(strings.Join(seen, "|"), "Iron") || strings.Contains(strings.Join(seen, "|"), "Humans") {
		t.Errorf("news highlights %v, want the modded names", seen)
	}
}
