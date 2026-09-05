package session

// plainWriter wraps a Session and strips ANSI escapes from everything written
// through it, for a caller whose terminal cannot interpret them — a door caller
// whose dropfile clears the ANSI flag (issue #101), or anyone run with -no-ansi.
// The local Console strips on its own (it can ask the console directly), so this
// is the remote equivalent: the dropfile flag is the only signal a caller at the
// far end of a socket gives us.
//
// It also advertises ANSI() == false, which is what the menu code reads to offer
// a numbered list in place of a lightbar (issue #99) — stripping alone leaves
// the lightbar unusable, since it also relies on cursor-home and erase-line to
// repaint in place.
type plainWriter struct {
	Session // inner session; promotes ReadKey (Write is overridden below)
}

// NewPlain returns a Session that emits no ANSI escapes.
func NewPlain(inner Session) Session { return &plainWriter{Session: inner} }

// DrainInput forwards to the inner session (see InputDrainer); the embedded
// Session promotes ReadKey but not this optional method, so forward explicitly.
func (p *plainWriter) DrainInput() { Drain(p.Session) }

func (p *plainWriter) Write(b []byte) (int, error) {
	if _, err := p.Session.Write(stripEscapes(b)); err != nil {
		return 0, err
	}
	return len(b), nil // the io.Writer contract is about b, not the stripped bytes
}

// ANSI reports that this session cannot render escape sequences.
func (p *plainWriter) ANSI() bool { return false }

// UTF8 forwards the inner session's charset marker, so this wrapper can sit on
// either side of the CP437 writer without hiding it from IsUTF8.
func (p *plainWriter) UTF8() bool { return IsUTF8(p.Session) }

// Column forwards the cursor column from a tracker below. The stripping above
// is why this is safe: what the tracker counts is what the terminal receives.
func (p *plainWriter) Column() int { n, _ := Column(p.Session); return n }

// HasANSI reports whether s can render ANSI escape sequences. A session that
// does not advertise the capability is treated as ANSI-capable — that is the
// case for every base session except a legacy Windows console, and for the door
// unless the dropfile says otherwise. Every wrapper forwards the marker, so it
// reads the same from anywhere in the chain.
func HasANSI(s Session) bool {
	if c, ok := s.(interface{ ANSI() bool }); ok {
		return c.ANSI()
	}
	return true
}
