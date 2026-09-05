package session

import (
	"fmt"
	"io"
)

// A read failing mid-session — an idle/time boot returning io.EOF, or a dropped
// connection — means the session is over. Rather than each prompt returning a
// fake value and letting the turn run on to completion on defaults, an input
// helper calls End to unwind the whole session at once. GuardEnd (deferred at
// the top of the menu loop) turns that back into an io.EOF return, which the
// caller already treats as a clean, save-and-exit end.
type endPanic struct{ err error }

// End aborts the session by panicking with the read error; recover it with
// GuardEnd or AsEnd. Input helpers call it when a read returns an error.
func End(err error) { panic(endPanic{err}) }

// AsEnd reports whether a recovered panic value came from End, returning the
// wrapped read error (io.EOF if none). For callers that want a custom recover.
func AsEnd(r any) (error, bool) {
	ep, ok := r.(endPanic)
	if !ok {
		return nil, false
	}
	if ep.err != nil {
		return ep.err, true
	}
	return io.EOF, true
}

// GuardEnd recovers an End panic into *err (as the wrapped read error, or
// io.EOF); any other panic is re-raised. Use as: defer session.GuardEnd(&err).
func GuardEnd(err *error) {
	if e, ok := AsEnd(recover()); ok {
		*err = e
	}
}

// LineMaxRunes bounds a typed line. Past it further keys are ignored (not
// echoed) until Backspace or Enter, so a caller holding a key down over a
// socket cannot grow the buffer for the whole session. Wider than any prompt's
// own limit (a realm name, a message line) so those still report their own
// error.
const LineMaxRunes = 255

// Editor is the line-editing state a prompt keeps: the runes typed and the
// column each one started at, so an erase walks the cursor back to a column the
// terminal actually reached rather than to a guess made from the answer.
//
// The columns come from a ColumnTracker under the charset writer (see
// column.go), which is what makes this charset-blind: "…" costs one column to a
// UTF-8 caller and three to a CP437 one, and neither this type nor its callers
// have to know which. Where no tracker is present — a test double, a bulletin
// rendered to a buffer — it falls back to one column per rune, which is exact
// for the digits and ASCII those cases carry.
type Editor struct {
	s     Session
	runes []rune
	cols  []int // cols[i] is the column runes[i] began at
	start int   // the column the answer began at, after the prompt
}

// NewEditor begins editing at the cursor's current column.
func NewEditor(s Session, typed []rune) *Editor {
	e := &Editor{s: s, start: col(s, 0)}
	// Runes the caller already echoed: their columns are unknown individually,
	// so they are measured as one each from the start. Only a peeked first key
	// arrives this way, which is a single printable rune.
	for i, r := range typed {
		e.runes = append(e.runes, r)
		e.cols = append(e.cols, e.start+i)
	}
	return e
}

// col reads the session's cursor column, or def where nothing tracks it.
func col(s Session, def int) int {
	if n, ok := Column(s); ok {
		return n
	}
	return def
}

// Put echoes r and remembers where it landed.
func (e *Editor) Put(r rune) {
	e.cols = append(e.cols, col(e.s, e.start+len(e.runes)))
	e.runes = append(e.runes, r)
	fmt.Fprintf(e.s, "%c", r)
}

// PutString echoes a whole string as one unit — a k/m/b expansion, or the max
// prefilled by ">" — which Backspace then removes a character at a time, as it
// does for anything else on the line.
func (e *Editor) PutString(str string) {
	for _, r := range str {
		e.Put(r)
	}
}

// Backspace erases the last rune, walking back over every column it occupied.
func (e *Editor) Backspace() {
	if len(e.runes) == 0 {
		return
	}
	to := e.cols[len(e.cols)-1]
	e.runes = e.runes[:len(e.runes)-1]
	e.cols = e.cols[:len(e.cols)-1]
	EraseBack(e.s, to, 1)
}

// Kill erases the whole answer, back to the column it began at.
func (e *Editor) Kill() {
	if len(e.runes) == 0 {
		return
	}
	n := len(e.runes)
	e.runes = e.runes[:0]
	e.cols = e.cols[:0]
	EraseBack(e.s, e.start, n)
}

// EraseBack blanks every column between the cursor and column to, for a caller
// that keeps its own buffer and so cannot use an Editor. Where nothing tracks
// the cursor — a test double, a bulletin rendered to a buffer — it erases
// fallback columns instead, which callers set to the rune count they echoed.
//
// Backspace-space-backspace rather than an ANSI erase, because a caller with no
// ANSI has to be able to correct a typo too.
func EraseBack(s Session, to, fallback int) {
	n := fallback
	if c, ok := Column(s); ok {
		n = c - to
	}
	for range n {
		fmt.Fprint(s, "\b \b")
	}
}

// Len is how many runes are in the buffer, and String what they spell.
func (e *Editor) Len() int       { return len(e.runes) }
func (e *Editor) String() string { return string(e.runes) }

// Reset empties the buffer WITHOUT erasing anything, for a caller that has
// repainted the line itself.
func (e *Editor) Reset() { e.runes, e.cols = e.runes[:0], e.cols[:0] }

// KillLine is Ctrl-U, which erases everything typed so far and leaves the
// cursor back at the prompt. The original has no such key: correcting a
// mistyped 1000000000 there means holding Backspace down ten times, which is
// the kind of wear this clone has no reason to reproduce. It is deliberately
// the one line-editing key IB adds.
//
// Ctrl-U cannot be a macro slot, so nothing a player binds can shadow it: BRE's
// fixed eight are D E F R I O K L (menu.macroKeys), and MacroExpander refuses
// this rune outright for the same reason it refuses Backspace and Enter.
const KillLine = 21

// ReadLine reads a line of input terminated by Enter, echoing keystrokes
// (the console runs in no-echo mode). Backspace/DEL erase the last rune and
// Ctrl-U the whole line.
// It returns whatever was typed so far if the stream ends.
func ReadLine(s Session) (string, error) { return ReadLineFrom(s, nil) }

// ReadLineFrom is ReadLine with `typed` already in the buffer. It exists for a
// prompt that must PEEK at the first keystroke — one offering a single-key
// shortcut such as "?" for a list — and then hand the key back when it turns out
// to be the start of an ordinary answer. The caller has already echoed those
// runes, so they are not echoed again.
func ReadLineFrom(s Session, typed []rune) (string, error) {
	e := NewEditor(s, typed)
	for {
		r, err := s.ReadKey()
		if err != nil {
			return e.String(), err
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(s, "\n")
			return e.String(), nil
		case 127, 8: // DEL / backspace
			e.Backspace()
		case KillLine:
			e.Kill()
		default:
			if r >= 32 && e.Len() < LineMaxRunes {
				e.Put(r)
			}
		}
	}
}
