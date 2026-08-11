package menu

import (
	"strings"
	"testing"
)

// TestSplashRendersWordmark checks the embedded CP437 splash decodes and prints:
// the gold-gradient SGR and a block glyph both survive FromCP437.
func TestSplashRendersWordmark(t *testing.T) {
	f := &fakeSession{}
	Splash(f)
	out := f.out.String()

	if !strings.Contains(out, "\x1b[38;5;136m") {
		t.Error("splash is missing the gold-gradient color code")
	}
	// The art is a half-block pixel canvas, so every art cell is U+2580.
	if !strings.Contains(out, "▀") {
		t.Error("splash is missing its block glyphs (CP437 decode failed?)")
	}
}
