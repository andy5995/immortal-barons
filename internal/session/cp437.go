package session

import (
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// cp437Writer wraps a Session so the engine's UTF-8 output is transcoded to
// CP437 on the wire — the character set traditional BBS terminals (SyncTERM,
// NetRunner) expect. UI text is pre-vetted per language (CP437Encodable — a
// CP437 session only renders catalogs that map cleanly), and cp437Fallback
// rewrites common typographic characters to ASCII, so the encoder's
// last-resort substitution byte (0x1a) should be effectively unreachable.
// Input is transcoded too: ReadKey decodes one raw byte as CP437 (see
// KeyByteReader), so a character typed on the terminal round-trips.
type cp437Writer struct {
	Session           // inner session; promotes ReadKey (Write is overridden below)
	enc     io.Writer // transform.Writer that encodes UTF-8 -> CP437 into inner
}

// cp437Fallback maps typographic characters with no CP437 form to their plain
// ASCII look-alikes before encoding, so an em-dash or curly quote in content
// (help pages, player-entered text) degrades to readable ASCII instead of the
// encoder's substitution glyph.
var cp437Fallback = strings.NewReplacer(
	"—", "--", // em dash
	"–", "-", // en dash
	"…", "...", // ellipsis
	"“", `"`, "”", `"`, // curly double quotes
	"‘", "'", "’", "'", // curly single quotes
)

// NewCP437Writer returns a Session that emits CP437 instead of UTF-8. The
// door/local front-end wraps its session in this when the caller's terminal is
// not UTF-8-capable (the default); UTF-8-capable front-ends (the web server)
// skip it and emit UTF-8 directly.
func NewCP437Writer(inner Session) Session {
	return &cp437Writer{
		Session: inner,
		enc:     transform.NewWriter(inner, encoding.ReplaceUnsupported(charmap.CodePage437.NewEncoder())),
	}
}

// ReadKey returns the next keypress decoded as CP437, so a high byte the
// terminal sends for a block or line-drawing character becomes that character
// rather than U+FFFD. CP437's low half is plain ASCII, so control keys — Enter,
// backspace, ESC and the Ctrl-letter macro triggers — are unaffected. An inner
// session with no raw-byte read (a test fake, a decorator) keeps the UTF-8 path.
func (c *cp437Writer) ReadKey() (rune, error) {
	br, ok := c.Session.(KeyByteReader)
	if !ok {
		return c.Session.ReadKey()
	}
	b, err := br.ReadKeyByte()
	if err != nil {
		return 0, err
	}
	return charmap.CodePage437.DecodeByte(b), nil
}

// DrainInput forwards to the inner session (see InputDrainer); the embedded
// Session promotes ReadKey but not this optional method, so forward explicitly.
func (c *cp437Writer) DrainInput() { Drain(c.Session) }

func (c *cp437Writer) Write(p []byte) (int, error) {
	// The fallback may change the byte count, so report the caller's own count
	// on success (the io.Writer contract is about p, not the transcoded bytes).
	if _, err := io.WriteString(c.enc, cp437Fallback.Replace(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// UTF8 reports that this session is NOT UTF-8 — it emits CP437.
func (c *cp437Writer) UTF8() bool { return false }

// ANSI forwards the inner session's capability marker, so this wrapper can sit
// on either side of the plain writer without hiding it from HasANSI.
func (c *cp437Writer) ANSI() bool { return HasANSI(c.Session) }

// CP437Encodable reports whether s can be rendered on a CP437 session with no
// loss — every rune maps to a CP437 code point. Used to decide whether a
// translated catalog is safe to show on a CP437 terminal (German maps; Cyrillic
// and CJK do not). Unlike NewCP437Writer it does NOT ReplaceUnsupported, so an
// unmappable rune surfaces as an error rather than a silent substitution.
func CP437Encodable(s string) bool {
	_, err := charmap.CodePage437.NewEncoder().String(s)
	return err == nil
}

// AllCP437Encodable reports whether every string in ss is CP437Encodable — e.g.
// whether a whole translation catalog can be shown on a CP437 terminal.
func AllCP437Encodable(ss []string) bool {
	for _, s := range ss {
		if !CP437Encodable(s) {
			return false
		}
	}
	return true
}

// IsUTF8 reports whether s emits UTF-8. A session that does not advertise its
// charset (every base session and the web session) is treated as UTF-8; only
// the CP437 wrapper reports otherwise. Read it before any further wrapping,
// since Deadline / MacroExpander / langSession do not forward the marker.
func IsUTF8(s Session) bool {
	if c, ok := s.(interface{ UTF8() bool }); ok {
		return c.UTF8()
	}
	return true
}

// LocaleIsUTF8 reports whether the process locale selects UTF-8, following the
// POSIX precedence LC_ALL > LC_CTYPE > LANG (the first one set decides). The
// local front-end uses this to pick its charset when neither -utf8 nor -cp437
// is given. A door ignores it: the process locale is the BBS server's, not the
// remote caller's terminal.
func LocaleIsUTF8() bool {
	for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if s := os.Getenv(v); s != "" {
			u := strings.ToUpper(s)
			return strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8")
		}
	}
	return false
}
