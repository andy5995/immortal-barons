// Package screen decodes CP437 artwork into the engine's internal UTF-8, so a
// screen can be authored in the ANSI tools artists already use (PabloDraw,
// Moebius, TheDraw) and shipped as a .ans file rather than as Go string
// escapes.
//
// It once also rendered field-token TEMPLATES — a .ans with {Label} and [value]
// spans the program filled in at display time — which is how the two Empire
// Status pages were drawn. Those pages were replaced by the original's own
// unboxed status block, written directly, and nothing else ever used the
// templating, so it is gone. Splash art is the remaining caller and needs only
// the decode.
package screen

import (
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// FromCP437 decodes a CP437 byte stream to UTF-8: ASCII bytes, which include
// every ANSI escape, pass through unchanged, and high bytes map to their CP437
// glyph. It uses the same code-page table as the output encoder
// (golang.org/x/text/encoding/charmap), so decode and encode are exact
// inverses — art authored in CP437 survives the round trip to a CP437 door.
func FromCP437(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(charmap.CodePage437.DecodeByte(c))
	}
	return sb.String()
}
