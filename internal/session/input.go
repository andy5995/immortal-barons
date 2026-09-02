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

// ReadLine reads a line of input terminated by Enter, echoing keystrokes
// (the console runs in no-echo mode). Backspace/DEL erase the last rune.
// It returns whatever was typed so far if the stream ends.
func ReadLine(s Session) (string, error) { return ReadLineFrom(s, nil) }

// ReadLineFrom is ReadLine with `typed` already in the buffer. It exists for a
// prompt that must PEEK at the first keystroke — one offering a single-key
// shortcut such as "?" for a list — and then hand the key back when it turns out
// to be the start of an ordinary answer. The caller has already echoed those
// runes, so they are not echoed again.
func ReadLineFrom(s Session, typed []rune) (string, error) {
	b := append([]rune(nil), typed...)
	for {
		r, err := s.ReadKey()
		if err != nil {
			return string(b), err
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(s, "\n")
			return string(b), nil
		case 127, 8: // DEL / backspace
			if len(b) > 0 {
				b = b[:len(b)-1]
				fmt.Fprint(s, "\b \b")
			}
		default:
			if r >= 32 && len(b) < LineMaxRunes {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
	}
}
