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
// from recursing. A Ctrl-<letter> with no macro is swallowed (a harmless
// no-op) rather than passed through as an unhandled control character.
type MacroExpander struct {
	inner  Session
	lookup func(letter string) (string, bool)
	queue  []rune
}

func NewMacroExpander(inner Session, lookup func(letter string) (string, bool)) *MacroExpander {
	return &MacroExpander{inner: inner, lookup: lookup}
}

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
	// codes that double as essential line-editing keys (Backspace, LF, Enter),
	// or ReadLine and single-key prompts would never see their terminator. Tab
	// (Ctrl-I) IS allowed, since BRE's macro set includes Ctrl-I. An unmapped
	// Ctrl-key just passes through (the menu ignores it).
	if r >= 1 && r <= 26 && r != '\b' && r != '\n' && r != '\r' {
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
