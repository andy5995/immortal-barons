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
// TheDraw) rather than as Go string escapes. It carries its own 256-color SGR:
// an "ANSI Shadow" wordmark with a vertical gold gradient over a starfield with
// three lit half-block planets. Modern terminals and xterm.js render the
// gradients; a 16-color client degrades gracefully.
//
//go:embed screens/splash.ans
var splashANS []byte

// Splash prints the Immortal Barons title screen, then waits for a keypress.
// FromCP437 decodes the .ans to the engine's internal UTF-8; the session's wire
// encoder re-encodes to CP437 for a CP437 door.
func Splash(s session.Session) {
	fmt.Fprint(s, ansi.Clear)
	fmt.Fprint(s, screen.FromCP437(splashANS))
	pause(s)
}
