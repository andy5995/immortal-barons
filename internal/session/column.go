package session

// column.go — where the cursor is on the current line, so an erase can walk
// back to a remembered column instead of guessing from what was typed.
//
// Counting the ANSWER's characters is not the same thing and cannot be made the
// same thing: the writers below this layer expand "…" into three dots and "—"
// into two hyphens, a k/m/b shortcut writes digits the player never typed, and
// a prompt may echo on the caller's behalf. Reading the column off the stream
// that actually reached the terminal answers all of those at once, and answers
// them the same way whatever the charset.

// ColumnTracker wraps a Session and counts the columns written to it, so a line
// editor can record where an answer starts and later erase exactly back to it.
//
// It belongs BELOW the charset writer and above the transport, which is where
// the bytes are the ones the terminal will really receive: an em dash arrives
// here as the two hyphens CP437 renders, not as one rune. That placement is
// also why it must be told the charset — a UTF-8 session sends a rune as
// several bytes that occupy one column, while every byte of a CP437 session's
// output is its own column.
type ColumnTracker struct {
	Session      // inner session; promotes ReadKey (Write is overridden below)
	utf8    bool // the encoding of what is written THROUGH here
	col     int
}

// NewColumnTracker wraps inner. Pass utf8 for a session whose writer emits
// UTF-8; CP437 and plain-ASCII sessions pass false.
func NewColumnTracker(inner Session, utf8 bool) *ColumnTracker {
	return &ColumnTracker{Session: inner, utf8: utf8}
}

// DrainInput forwards to the inner session (see InputDrainer); the embedded
// Session promotes ReadKey but not this optional method.
func (c *ColumnTracker) DrainInput() { Drain(c.Session) }

// Column is how many columns have been written since the line began.
func (c *ColumnTracker) Column() int { return c.col }

// TabStop is how far a horizontal tab advances the cursor. Eight is the DOS and
// terminal default and nothing here changes it.
const TabStop = 8

func (c *ColumnTracker) Write(p []byte) (int, error) {
	// Escapes move the cursor without occupying a column, so they are skipped
	// rather than counted. A sequence that MOVES the cursor (a lightbar
	// repaint) therefore leaves this count wrong — which is why a line editor
	// records its start column at the prompt and never carries one across a
	// redraw.
	for _, b := range stripEscapes(p) {
		switch {
		case b == '\n' || b == '\r':
			c.col = 0
		case b == '\b':
			if c.col > 0 {
				c.col--
			}
		case b == '\t':
			c.col += TabStop - c.col%TabStop
		case b < 0x20:
			// A bell or other control byte prints nothing.
		case c.utf8 && b >= 0x80 && b < 0xc0:
			// A UTF-8 continuation byte is part of the rune before it.
		default:
			c.col++
		}
	}
	return c.Session.Write(p)
}

// UTF8, ASCII and ANSI forward the inner session's markers, so this wrapper can
// sit anywhere in the chain without hiding a capability from IsUTF8 and friends.
func (c *ColumnTracker) UTF8() bool  { return IsUTF8(c.Session) }
func (c *ColumnTracker) ASCII() bool { return IsASCII(c.Session) }
func (c *ColumnTracker) ANSI() bool  { return HasANSI(c.Session) }

// Column is the cursor's column on s, and whether s can report one at all. A
// session with no tracker beneath it — a bulletin file being rendered to a
// buffer, a test double — reports false, and a caller falls back to counting
// what it echoed.
func Column(s Session) (int, bool) {
	if c, ok := s.(interface{ Column() int }); ok {
		return c.Column(), true
	}
	return 0, false
}
