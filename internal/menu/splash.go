package menu

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/i18n"
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
// The art fills all 80 columns, so Splash never lets a row's CR/LF move the
// cursor: it clears the screen and puts every row at its own position. Painting
// column 80 can wrap the cursor by itself, and a CR/LF after that advances a
// second time, leaving a blank line between every row. Terminals disagree on
// when that wrap fires (xfce defers it, SyncTERM does not), so a local check
// cannot catch it. Turning autowrap off (ansi.WrapOff) fixed SyncTERM, but
// mTelnet, NetRunner and RGTerm ignore it; a position per row works on all of
// them, and makes the wrap setting irrelevant.
//
// Empty sky is painted black as 48;5;16, never as the default background (49).
// NetRunner keeps the last 256-color background in a register that neither
// ESC[0m nor ESC[49m clears, and every 256-color foreground paints it again —
// so with 49 each star came out on whatever background was set last. Splash
// sets black at the start of every row, since a row that sets only a
// foreground (the tagline) would otherwise inherit the row above's last
// background, and leaves the register black for whatever is drawn next.
//
//go:embed screens/splash.ans
var splashANS []byte

// Splash prints the Immortal Barons title screen, then waits for a keypress.
// FromCP437 decodes the .ans to the engine's internal UTF-8; the session's wire
// encoder re-encodes to CP437 for a CP437 door.
func Splash(s session.Session) {
	rows := splashRows()
	fmt.Fprint(s, ansi.Clear)
	for i, row := range rows {
		fmt.Fprint(s, ansi.MoveTo(i+1, 1), splashBlack, row)
	}
	// BRE's prompt sits on the row right under the art.
	fmt.Fprint(s, splashBlack, ansi.Reset, ansi.MoveTo(len(rows)+1, 1))
	pauseTight(s)
}

// splashBlack is black as a 256-color background; see the note on splashANS.
const splashBlack = "\x1b[48;5;16m"

// splashRows is the art one screen row per entry, decoded to UTF-8, with the
// CR/LF that ended each row removed.
func splashRows() []string {
	art := strings.TrimRight(screen.FromCP437(splashANS), "\r\n")
	return strings.Split(strings.ReplaceAll(art, "\r\n", "\n"), "\n")
}

// ShowLeagueFrozen tells a caller the league is frozen for an update, with the
// League Coordinator's own message beneath, and waits for a key. It runs before
// the menu engine knows the caller's language, so lang is passed in, as
// AskRealmName's is.
func ShowLeagueFrozen(s session.Session, lang, message string) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightYellow,
		WrapIndented(i18n.T(lang, "The league is paused while its boards are updated. Please check back later."), ""), ansi.Reset)
	if message != "" {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, WrapIndented(message, ""), ansi.Reset)
	}
	pause(s)
}
