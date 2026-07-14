package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
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
		{ansi.FgMagenta, "Mechanoids"},
		{ansi.FgBrightMagenta, "Rexxogans"},
		{ansi.FgBrightCyan, "Spacians"},
	}
	for _, w := range wants {
		if !strings.Contains(out, w.color+w.name) {
			t.Errorf("%s should be drawn in color %q; output:\n%s", w.name, w.color, out)
		}
	}
}
