package help

import (
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
)

// RenderANSI turns a topic's Markdown body into colored, word-wrapped terminal
// text at the given column width (use ~78-80 for a standard screen). It
// supports the small Markdown subset the help files use on purpose:
//
//   - ATX headings: "# ", "## ", "### "
//   - bullet lists: lines starting "- " or "* "
//   - blank-line paragraph breaks
//   - inline **bold** and `code` (markers stripped)
//
// Anything outside the subset passes through as plain wrapped text. The help
// files are authored to stay inside this subset, so we need no Markdown
// dependency. If we later want colored inline spans, Wrap() must be taught to
// measure *visible* width (ignoring SGR bytes); today inline() strips markers
// so visible width equals rune count.
func (t Topic) RenderANSI(width int) string {
	if width < 20 {
		width = 20
	}
	var b strings.Builder
	var para []string // words buffered for the current paragraph

	// flush emits the buffered paragraph, wrapped, then clears the buffer.
	flush := func() {
		if len(para) == 0 {
			return
		}
		b.WriteString(Wrap(strings.Join(para, " "), width))
		b.WriteByte('\n')
		para = nil
	}

	for _, line := range strings.Split(t.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flush()
			b.WriteByte('\n')
		case strings.HasPrefix(trimmed, "### "):
			flush()
			b.WriteString(ansi.FgBrightCyan + inline(trimmed[4:]) + ansi.Reset + "\n")
		case strings.HasPrefix(trimmed, "## "):
			flush()
			b.WriteString(ansi.FgBrightCyan + inline(trimmed[3:]) + ansi.Reset + "\n")
		case strings.HasPrefix(trimmed, "# "):
			flush()
			b.WriteString(ansi.FgBrightWhite + inline(trimmed[2:]) + ansi.Reset + "\n")
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			flush()
			b.WriteString(bullet(inline(strings.TrimSpace(trimmed[2:])), width))
			b.WriteByte('\n')
		default:
			para = append(para, inline(trimmed))
		}
	}
	flush()
	return strings.TrimRight(b.String(), "\n")
}

// inline strips inline markup (**bold**, `code`), keeping the text. We drop the
// markers rather than emit SGR so the wrap width (a rune count) stays accurate.
func inline(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

// wrap word-wraps text to width columns, measuring width in runes (so multibyte
// glyphs count as one column, not their byte length).
// Wrap re-flows plain text (no ANSI markers) to the given column width, breaking
// only at spaces so words are never split mid-token. Shared with the menu package
// for wrapping advisor/report prose to the screen width.
func Wrap(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := utf8.RuneCountInString(w)
		if i > 0 {
			if lineLen+1+wl > width {
				b.WriteByte('\n')
				lineLen = 0
			} else {
				b.WriteByte(' ')
				lineLen++
			}
		}
		b.WriteString(w)
		lineLen += wl
	}
	return b.String()
}

// bullet renders "  • item" with a hanging indent, so wrapped continuation
// lines align under the item text rather than under the bullet.
func bullet(item string, width int) string {
	const lead = "  • " // two spaces, bullet, space
	const cont = "    " // continuation indent (aligns under the text)
	wrapped := Wrap(item, width-len([]rune(lead)))
	lines := strings.Split(wrapped, "\n")
	var b strings.Builder
	for i, l := range lines {
		if i == 0 {
			b.WriteString(lead + l)
		} else {
			b.WriteString("\n" + cont + l)
		}
	}
	return b.String()
}
