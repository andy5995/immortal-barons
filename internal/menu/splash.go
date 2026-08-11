package menu

import (
	_ "embed"
	"fmt"

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
// The art is a half-block pixel canvas: every cell is U+2580, its foreground
// painting the cell's top pixel and its background the bottom one. That makes
// pixels square, which is what lets the planets read as spheres — a circle
// drawn with whole character cells comes out twice as tall as it is wide. The
// wordmark is a 5x7 bitmap on the same canvas, so it costs four rows where a
// FIGlet block font costs six. Every object is lit from the upper left.
// Modern terminals and xterm.js render the 256-color ramps; a 16-color client
// degrades gracefully.
//
//go:embed screens/splash.ans
var splashANS []byte

// Splash prints the Immortal Barons title screen, then waits for a keypress.
// FromCP437 decodes the .ans to the engine's internal UTF-8; the session's wire
// encoder re-encodes to CP437 for a CP437 door.
func Splash(s session.Session) {
	fmt.Fprint(s, screen.FromCP437(splashANS))
	pauseTight(s) // the art ends on its own line; BRE's prompt sits right under it
}
