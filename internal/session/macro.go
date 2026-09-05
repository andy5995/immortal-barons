package session

// MacroExpander wraps a Session and expands Ctrl-<letter> keypresses into a
// saved keystroke sequence. It is game-agnostic: the sequence is looked up
// through the lookup callback (keyed by the uppercase letter "A".."Z"), so the
// session layer stays decoupled from the game.
//
// When a Ctrl-<letter> (raw runes 1..26 = Ctrl-A..Ctrl-Z) maps to a macro, its
// runes are queued and returned by subsequent ReadKey calls before the
// underlying session is read again. Runes replayed from the queue are returned
// verbatim — a Ctrl-key inside a macro is NOT re-expanded, which keeps macros
// from recursing. A Ctrl-<letter> with no macro passes through unchanged, which
// is what lets the line editors below see Ctrl-U.
type MacroExpander struct {
	inner  Session
	lookup func(letter string) (string, bool)
	queue  []rune
}

func NewMacroExpander(inner Session, lookup func(letter string) (string, bool)) *MacroExpander {
	return &MacroExpander{inner: inner, lookup: lookup}
}

// DrainInput forwards to the inner session (see InputDrainer).
func (m *MacroExpander) DrainInput() { Drain(m.inner) }

func (m *MacroExpander) ReadKey() (rune, error) {
	if len(m.queue) > 0 {
		r := m.queue[0]
		m.queue = m.queue[1:]
		return r, nil
	}
	r, err := m.inner.ReadKey()
	if err != nil {
		return r, err
	}
	// Ctrl-A..Ctrl-Z can trigger a macro — but NEVER intercept the control
	// codes that double as essential line-editing keys (Backspace, LF, Enter,
	// and Ctrl-U for erase-line), or ReadLine and single-key prompts would never
	// see their terminator. Tab (Ctrl-I) IS allowed, since BRE's macro set
	// includes Ctrl-I; Ctrl-U is not in that set (D E F R I O K L), so reserving
	// it costs a player nothing. An unmapped Ctrl-key just passes through (the
	// menu ignores it).
	if r >= 1 && r <= 26 && r != '\b' && r != '\n' && r != '\r' && r != KillLine {
		letter := string(rune('A' + r - 1))
		if seq, ok := m.lookup(letter); ok && seq != "" {
			m.queue = []rune(seq)
			first := m.queue[0]
			m.queue = m.queue[1:]
			return first, nil
		}
	}
	return r, nil
}

func (m *MacroExpander) Write(p []byte) (int, error) { return m.inner.Write(p) }

// SetInputLine forwards the prompt-restore hook to the inner session (the
// Deadline) if it supports it, so a warning can restore the caller's input line.
func (m *MacroExpander) SetInputLine(line string) {
	if s, ok := m.inner.(InputLineSetter); ok {
		s.SetInputLine(line)
	}
}

// UTF8, ASCII and ANSI forward the caller's charset and capability markers to
// the inner session. Embedding is not used here (the expander wraps rather than
// embeds), and this is the OUTERMOST wrapper on a live session — so without
// these, IsUTF8/IsASCII asked at a prompt fall back to their defaults and report
// UTF-8 for a CP437 caller. The line editors measure an echo through them.
func (m *MacroExpander) UTF8() bool  { return IsUTF8(m.inner) }
func (m *MacroExpander) ASCII() bool { return IsASCII(m.inner) }
func (m *MacroExpander) ANSI() bool  { return HasANSI(m.inner) }

// Column forwards the cursor column from a tracker below (see ColumnTracker).
func (m *MacroExpander) Column() int { n, _ := Column(m.inner); return n }
