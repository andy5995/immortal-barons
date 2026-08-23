package menu

import (
	_ "embed"
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/screen"
	"github.com/andy5995/immortal-barons/internal/session"
)

// splashANS is the title screen, authored as a CP437 .ans (like the Empire
// Status screens) so it is editable in the usual ANSI-art tools (PabloDraw,
// Moebius) rather than as Go string escapes. THE FILE IS THE SOURCE OF TRUTH —
// edit it directly. A generator laid the piece out originally (it is kept with
// this project's other scripts), but re-running it would discard hand edits, so
// it is a starting point rather than a build step.
//
// The art is a half-block pixel canvas: an art cell is U+2580, its foreground
// painting the cell's top pixel and its background the bottom one. That makes
// pixels square, which is what lets the planets read as spheres — a circle
// drawn with whole character cells comes out twice as tall as it is wide. The
// wordmark is a 5x7 bitmap on the same canvas, so it costs four rows where a
// FIGlet block font costs six, and it is bevelled rather than filled flat: it
// is the piece's signature element and gets the only bright color in it.
// Every object is lit from the upper left.
//
// The background cells are the exception to U+2580 — stars and the nebula
// drift are punctuation glyphs ('.', U+00B7, U+00B0) in near-black colors,
// because they need far less ink per cell than the lightest shade block can
// give. Modern terminals and xterm.js render the 256-color ramps; a 16-color
// client degrades gracefully.
//
// Both the shading and the limbs are ORDERED-DITHERED at pixel resolution.
// The 256-color cube carries six levels per channel, which is not enough to
// render a sphere without visible banding, and a partly covered edge pixel has
// no dark saturated color to fade into — so each pixel is scattered between
// the two palette entries that straddle its true color, and coverage is
// applied as the same kind of alpha. The generator carries the reasoning and
// the two approaches that were tried first and did not work.
//
// The art fills all 80 columns, which is only safe because Splash turns AUTOWRAP
// OFF around it (ansi.WrapOff). Painting column 80 otherwise wraps the cursor by
// itself and the row's own CR/LF advances a second time, leaving a blank line
// between every row — which is how this art once reached SyncTERM as alternating
// picture and black bands while looking perfect locally. Terminals disagree on
// when that wrap fires (xfce defers it, SyncTERM does not), so a local check
// cannot catch it. Leave the wrap toggle in place if the art stays full-width.
//
//go:embed screens/splash.ans
var splashANS []byte

// Splash prints the Immortal Barons title screen, then waits for a keypress.
// FromCP437 decodes the .ans to the engine's internal UTF-8; the session's wire
// encoder re-encodes to CP437 for a CP437 door.
func Splash(s session.Session) {
	// Autowrap off for the art: it fills all 80 columns, and column 80 would
	// otherwise wrap the cursor on top of the row's own CR/LF. See ansi.WrapOff.
	fmt.Fprint(s, ansi.WrapOff, screen.FromCP437(splashANS), ansi.WrapOn)
	pauseTight(s) // the art ends on its own line; BRE's prompt sits right under it
}
