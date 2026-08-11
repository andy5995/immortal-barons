// Package screen renders artist-editable ANSI template screens: a CP437 .ans
// file (what tools like PabloDraw and TheDraw produce) with fixed-width field
// tokens the program fills in at display time. The template owns the layout and
// colors; the program owns the live values and their translations, so a screen
// can be reskinned without touching Go code.
//
// Tokens (the token's on-screen span IS the field width, so widening a field to
// fit a translation is just adding spaces inside the brackets):
//
//	{Label      }  translatable label — the trimmed content is the msgid; the
//	               program translates it and left-justifies into the span.
//	[value      ]  value slot — the trimmed content is a field key; the program
//	               formats the value and right-justifies into the span.
//
// A leading space inside either token flips its default alignment: {  Label}
// right-aligns a header over a column of numbers, and [ sentence ] left-aligns
// a value that is prose rather than a figure.
//
// A literal delimiter is written by doubling it ({{ }} [[ ]]).
package screen

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Render converts a CP437 template to UTF-8 and fills its field tokens. Label
// tokens are translated with translate(); value tokens are looked up in values
// (a missing key yields an empty field). Each substitution is padded or
// truncated to the token's span width so the fixed layout holds.
func Render(template []byte, values map[string]string, translate func(string) string) string {
	src := fromCP437(template)
	var b strings.Builder
	runes := []rune(src)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == 0x1b { // ANSI escape (ESC[…final): copy verbatim so its '[' is
			// not misread as a value token; every color code contains one.
			b.WriteRune(r)
			if i+1 < len(runes) && runes[i+1] == '[' {
				i++
				b.WriteRune(runes[i])
				for i+1 < len(runes) {
					i++
					b.WriteRune(runes[i])
					if runes[i] >= 0x40 && runes[i] <= 0x7e { // final byte
						break
					}
				}
			}
			continue
		}
		switch r {
		case '{', '[':
			// Doubled delimiter is a literal.
			if i+1 < len(runes) && runes[i+1] == r {
				b.WriteRune(r)
				i++
				continue
			}
			close := '}'
			if r == '[' {
				close = ']'
			}
			end := indexRune(runes, close, i+1)
			if end < 0 { // no closing delimiter: emit as-is
				b.WriteRune(r)
				continue
			}
			content := string(runes[i+1 : end])
			width := end - i + 1 // span width in columns, delimiters included
			key := strings.TrimSpace(content)
			// A leading space inside the token flips the default alignment:
			// labels are left-aligned, values right-aligned (they are figures).
			flip := strings.HasPrefix(content, " ")
			if r == '{' {
				b.WriteString(fit(translate(key), width, flip))
			} else {
				b.WriteString(fit(values[key], width, !flip))
			}
			i = end
		case '}', ']':
			if i+1 < len(runes) && runes[i+1] == r { // doubled -> literal
				b.WriteRune(r)
				i++
				continue
			}
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FromCP437 decodes a CP437 byte stream to UTF-8 (ASCII bytes, which include
// every ANSI escape, pass through unchanged; high bytes map to their CP437
// glyph). Use it to render a static .ans screen that has no {label}/[value]
// field tokens — a title screen or splash — where the token parsing in Render
// is neither needed nor wanted.
func FromCP437(b []byte) string { return fromCP437(b) }

// Label is one translatable label field found in a template: its msgid and the
// column width its token span reserves. A build-time guard uses this to flag a
// translation that would not fit its cell.
type Label struct {
	Msgid string
	Width int
}

// Labels returns every {label} field in a template (after CP437 conversion), so
// a test can check that each language's translation fits the reserved width.
func Labels(template []byte) []Label {
	runes := []rune(fromCP437(template))
	var out []Label
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '{' { // literal
			i++
			continue
		}
		end := indexRune(runes, '}', i+1)
		if end < 0 {
			break
		}
		out = append(out, Label{Msgid: strings.TrimSpace(string(runes[i+1 : end])), Width: end - i + 1})
		i = end
	}
	return out
}

// fit pads s with spaces or truncates it to exactly width display columns.
// rightAlign puts the padding on the left (for numbers); otherwise on the right.
func fit(s string, width int, rightAlign bool) string {
	n := utf8.RuneCountInString(s)
	if n == width {
		return s
	}
	if n > width {
		return string([]rune(s)[:width])
	}
	pad := strings.Repeat(" ", width-n)
	if rightAlign {
		return pad + s
	}
	return s + pad
}

func indexRune(runes []rune, target rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}

// fromCP437 maps a CP437 byte stream to UTF-8 using the same code-page table
// as the output encoder (golang.org/x/text/encoding/charmap), so decode and
// encode are exact inverses. ASCII bytes (which include every ANSI escape
// sequence) map to themselves; high bytes map to their CP437 glyph. This lets
// artists author screens in the CP437 tools they already use while the game
// works internally in UTF-8.
func fromCP437(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(charmap.CodePage437.DecodeByte(c))
	}
	return sb.String()
}
