package session

import (
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// asciiWriter wraps a Session so the engine's UTF-8 output leaves as 7-bit
// ASCII. It is the third charset, beside UTF-8 and CP437: a terminal that is
// neither — a modern telnet or ssh client sitting in Latin-1, or anything
// whose encoding the sysop cannot pin down — renders CP437 line-drawing bytes
// as mojibake and multi-byte UTF-8 as pairs of stray characters. ASCII is
// plainer than both and cannot be mis-decoded.
//
// The mapping is a look-alike substitution, so a table keeps its shape: box
// rules become - | +, blocks and shades become # : . and so on. An accented
// letter loses its accent (ü -> u) rather than becoming a question mark, which
// keeps German legible; a letter with no ASCII likeness at all (Cyrillic, CJK)
// does become "?", which is why an ASCII session is held to English.
type asciiWriter struct {
	Session // inner session; promotes ReadKey (Write is overridden below)
}

// NewASCIIWriter returns a Session that emits 7-bit ASCII.
func NewASCIIWriter(inner Session) Session { return &asciiWriter{Session: inner} }

// DrainInput forwards to the inner session (see InputDrainer); the embedded
// Session promotes ReadKey but not this optional method, so forward explicitly.
func (a *asciiWriter) DrainInput() { Drain(a.Session) }

func (a *asciiWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(a.Session, ToASCII(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil // the io.Writer contract is about p, not the substituted bytes
}

// UTF8 reports that this session is NOT UTF-8 — it emits ASCII.
func (a *asciiWriter) UTF8() bool { return false }

// ASCII reports that this session is held to 7-bit ASCII, which the menu code
// reads to offer only languages that survive the substitution (English).
func (a *asciiWriter) ASCII() bool { return true }

// ANSI forwards the inner session's capability marker, so this wrapper can sit
// on either side of the plain writer without hiding it from HasANSI.
func (a *asciiWriter) ANSI() bool { return HasANSI(a.Session) }

// IsASCII reports whether s is held to 7-bit ASCII. A session that does not
// advertise the marker is not — the same convention as IsUTF8/HasANSI.
func IsASCII(s Session) bool {
	if c, ok := s.(interface{ ASCII() bool }); ok {
		return c.ASCII()
	}
	return false
}

// asciiLookAlikes maps the non-ASCII characters the game actually emits — the
// box rules and blocks from the .ans screens and the table borders, plus the
// typographic punctuation in the UI text — to their nearest ASCII shape. Runes
// not listed here fall through to the accent-stripping pass in ToASCII.
var asciiLookAlikes = strings.NewReplacer(
	// Single and double box drawing: rules keep their direction, junctions and
	// corners all become "+", which is how a plain-ASCII table has always looked.
	"─", "-", "━", "-", "═", "=",
	"│", "|", "┃", "|", "║", "|",
	"┌", "+", "┐", "+", "└", "+", "┘", "+",
	"├", "+", "┤", "+", "┬", "+", "┴", "+", "┼", "+",
	"╔", "+", "╗", "+", "╚", "+", "╝", "+",
	"╠", "+", "╣", "+", "╦", "+", "╩", "+", "╬", "+",
	// Blocks and shades, darkest to lightest.
	"█", "#", "▓", "#", "▒", ":", "░", ".",
	"▀", "-", "▄", "_", "■", "#", "▪", "*", "▬", "-",
	// Arrows and pointers.
	"→", "->", "←", "<-", "↑", "^", "↓", "v", "⇒", "=>",
	"»", ">", "«", "<", "‹", "<", "›", ">",
	// Typographic punctuation and the few symbols in figures and prose.
	"—", "--", "–", "-", "−", "-", "…", "...",
	"“", `"`, "”", `"`, "„", `"`, "‘", "'", "’", "'",
	"·", ".", "•", "*", "★", "*", "☆", "*",
	"×", "x", "÷", "/", "≈", "~", "≤", "<=", "≥", ">=", "≠", "!=",
	"²", "2", "³", "3", "¹", "1", "½", "1/2", "¼", "1/4", "¾", "3/4",
	"°", "deg", "±", "+/-", "µ", "u", "¢", "c", "£", "L", "¥", "Y", "€", "EUR",
	"©", "(c)", "®", "(r)", "™", "(tm)", "§", "S", "¶", "P", "†", "+", "‡", "++",
	"∈", "in", "Σ", "sum", "∞", "inf", "√", "sqrt",
	// Letters with no ASCII base to decompose to, but a settled transliteration.
	"ß", "ss", "Æ", "AE", "æ", "ae", "Œ", "OE", "œ", "oe",
	"Ø", "O", "ø", "o", "Þ", "Th", "þ", "th", "Ð", "D", "ð", "d",
	"Ł", "L", "ł", "l", "Đ", "D", "đ", "d", "ı", "i",
	" ", " ", // no-break space
)

// ToASCII renders s as 7-bit ASCII: known glyphs become their look-alikes, an
// accented letter is stripped to its base letter, and anything still outside
// ASCII becomes "?". ANSI escapes pass through untouched — they are ASCII
// already, and stripping them is the separate job of the plain writer.
func ToASCII(s string) string {
	s = asciiLookAlikes.Replace(s)
	if isASCIIOnly(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		default:
			// Decompose (ü -> u + combining diaeresis) and keep the base letter when
			// it is ASCII; a letter with no ASCII base has no look-alike to offer.
			d := []rune(norm.NFD.String(string(r)))
			if len(d) > 0 && d[0] < 0x80 && !unicode.IsSpace(d[0]) {
				b.WriteRune(d[0])
				continue
			}
			b.WriteByte('?')
		}
	}
	return b.String()
}

func isASCIIOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// ASCIIEncodable reports whether s survives ToASCII with every letter intact —
// no character replaced by "?". An accent lost off a letter still counts as
// intact (German reads fine without its umlauts); a Cyrillic or CJK catalog
// does not. Counting rather than searching, so a string with a question mark of
// its own is not mistaken for a substitution.
func ASCIIEncodable(s string) bool {
	return strings.Count(ToASCII(s), "?") == strings.Count(s, "?")
}

// AllASCIIEncodable reports whether every string in ss is ASCIIEncodable.
func AllASCIIEncodable(ss []string) bool {
	for _, s := range ss {
		if !ASCIIEncodable(s) {
			return false
		}
	}
	return true
}
