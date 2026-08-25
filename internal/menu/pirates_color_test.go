package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// TestAttackPiratesColorsFactions checks each faction name is drawn in its
// BRE color (verified from BRE.EXE's color table). Colors are keyed to
// game.PirateFactions order, which is how the menu lists them.
func TestAttackPiratesColorsFactions(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0              // past new-realm protection so the raid list shows
	f := &fakeSession{keys: []rune("0\r")} // cancel at the faction prompt

	attackPirates(f, w)
	out := f.out.String()

	// Spot-check the ends and a middle pair the eye tends to get wrong.
	wants := []struct {
		color, name string
	}{
		{ansi.FgBrightGreen, "Humans"},
		{ansi.FgRed, "Sharks"},
		{ansi.FgBrightMagenta, "Trilobarians"},
		{ansi.FgBrightCyan, "Ammonians"},
	}
	for _, w := range wants {
		if !strings.Contains(out, w.color+w.name) {
			t.Errorf("%s should be drawn in color %q; output:\n%s", w.name, w.color, out)
		}
	}
}

// TestPirateColorsMatchTheFactions checks the one thing a slot-keyed palette
// cannot check for itself: that slot i still paints the faction it says it
// does. The colours are looked up by index into a list that lives in another
// package, so reordering or renaming game.PirateFactions would repaint every
// faction below the change without any compile error.
func TestPirateColorsMatchTheFactions(t *testing.T) {
	if len(pirateColors) != len(game.PirateFactions) {
		t.Fatalf("pirateColors has %d rows, game.PirateFactions has %d",
			len(pirateColors), len(game.PirateFactions))
	}
	for i, want := range game.PirateFactions {
		if got := pirateColors[i].Faction; got != want {
			t.Errorf("slot %d paints %q, but game.PirateFactions has %q there", i, got, want)
		}
		if pirateColors[i].Color == "" {
			t.Errorf("slot %d (%s) has no colour", i, want)
		}
	}
}
