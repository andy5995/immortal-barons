// Package textwrap re-flows console messages to a fixed width. A sysop reads
// them on an 80-column BBS console, where a long line left to the terminal
// breaks mid-word (#156).
package textwrap

import (
	"strings"
	"unicode/utf8"
)

// Console is the width to wrap a sysop-facing message to. Four columns short
// of 80, so a message is never flush against the edge.
const Console = 76

// Wrap breaks text at spaces so no word is split, putting indent in front of
// every line after the first. The caller prints its own prefix and passes an
// indent as wide as it, so the first line is measured as though the prefix
// were already there and the rest lands in the same column. Width is measured
// in runes, so a multibyte glyph counts as the one column it occupies. A
// single word longer than the width still overflows: breaking a path in the
// middle would make it unreadable and uncopyable.
func Wrap(text string, width int, indent string) string {
	var b strings.Builder
	lineLen := utf8.RuneCountInString(indent)
	for i, word := range strings.Fields(text) {
		wordLen := utf8.RuneCountInString(word)
		switch {
		case i == 0:
		case lineLen+1+wordLen > width:
			b.WriteString("\n" + indent)
			lineLen = utf8.RuneCountInString(indent)
		default:
			b.WriteByte(' ')
			lineLen++
		}
		b.WriteString(word)
		lineLen += wordLen
	}
	return b.String()
}
