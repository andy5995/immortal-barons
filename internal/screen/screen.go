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
// A literal delimiter is written by doubling it ({{ }} [[ ]]).
package screen

import (
	"strings"
	"unicode/utf8"
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
			if r == '{' {
				// A leading space inside the braces right-aligns the label
				// (for table headers over right-aligned numbers); the default
				// is left-aligned.
				b.WriteString(fit(translate(key), width, strings.HasPrefix(content, " ")))
			} else {
				b.WriteString(fit(values[key], width, true))
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

// fromCP437 maps a CP437 byte stream to UTF-8. Bytes below 0x80 (ASCII, which
// includes every ANSI escape sequence) pass through unchanged; high bytes map
// to their CP437 glyph. This lets artists author in the CP437 tools they
// already use while the game emits UTF-8.
func fromCP437(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(cp437High[c-0x80])
		}
	}
	return sb.String()
}

// cp437High is the Unicode glyph for each CP437 byte 0x80..0xFF, in order.
var cp437High = [...]rune{
	'Ç', 'ü', 'é', 'â', 'ä', 'à', 'å', 'ç', 'ê', 'ë', 'è', 'ï', 'î', 'ì', 'Ä', 'Å',
	'É', 'æ', 'Æ', 'ô', 'ö', 'ò', 'û', 'ù', 'ÿ', 'Ö', 'Ü', '¢', '£', '¥', '₧', 'ƒ',
	'á', 'í', 'ó', 'ú', 'ñ', 'Ñ', 'ª', 'º', '¿', '⌐', '¬', '½', '¼', '¡', '«', '»',
	'░', '▒', '▓', '│', '┤', '╡', '╢', '╖', '╕', '╣', '║', '╗', '╝', '╜', '╛', '┐',
	'└', '┴', '┬', '├', '─', '┼', '╞', '╟', '╚', '╔', '╩', '╦', '╠', '═', '╬', '╧',
	'╨', '╤', '╥', '╙', '╘', '╒', '╓', '╫', '╪', '┘', '┌', '█', '▄', '▌', '▐', '▀',
	'α', 'ß', 'Γ', 'π', 'Σ', 'σ', 'µ', 'τ', 'Φ', 'Θ', 'Ω', 'δ', '∞', 'φ', 'ε', '∩',
	'≡', '±', '≥', '≤', '⌠', '⌡', '÷', '≈', '°', '∙', '·', '√', 'ⁿ', '²', '■', ' ',
}
