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
// NetRunner) and Synchronet doors expect. A rune with no CP437 equivalent is
// replaced with the substitution byte 0x1a; in practice a CP437 session forces
// English (see ctx.UTF8), so every rune it emits is already CP437-mappable and
// the substitution is effectively unreachable. ReadKey passes through to the
// inner session; only output is transcoded.
type cp437Writer struct {
	Session           // inner session; promotes ReadKey (Write is overridden below)
	enc     io.Writer // transform.Writer that encodes UTF-8 -> CP437 into inner
}

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

func (c *cp437Writer) Write(p []byte) (int, error) { return c.enc.Write(p) }

// UTF8 reports that this session is NOT UTF-8 — it emits CP437.
func (c *cp437Writer) UTF8() bool { return false }

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
