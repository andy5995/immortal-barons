// Package session defines the byte-stream abstraction between the game
// engine and whatever the player is connected through. The engine reads
// keypresses from, and writes ANSI bytes to, a Session; it never knows
// whether that is a local console, a BBS socket, or a websocket.
package session

import (
	"bufio"
	"io"
)

type Session interface {
	// ReadKey returns a single keypress. BRE uses a hotkey model: one
	// letter acts immediately, no Enter required.
	ReadKey() (rune, error)
	// Writer receives ANSI bytes to display to the player.
	io.Writer
}

// InputDrainer is the optional capability to drop a line terminator left in the
// input buffer after a single-key answer, so it does not leak into the next
// single-key prompt. A telnet line-mode client sends the answer key and its
// Enter as one burst ("n\r\n"), so after the key is read the CR/LF sits buffered
// and would otherwise auto-answer the following prompt. Base readers implement
// it over their bufio buffer; decorators forward it (like InputLineSetter);
// sessions with no input buffer (web, test fakes) simply do not implement it.
type InputDrainer interface{ DrainInput() }

// Drain invokes s's DrainInput if it has one, so callers and forwarding
// wrappers need no type assertion of their own.
func Drain(s Session) {
	if d, ok := s.(InputDrainer); ok {
		d.DrainInput()
	}
}

// drainTerminators consumes CR/LF/NUL bytes ALREADY buffered in r — non-blocking
// (only what is present, never a fresh read) — and stops at the first non-
// terminator, so a real typed-ahead key is left for the next prompt.
func drainTerminators(r *bufio.Reader) {
	for r.Buffered() > 0 {
		b, err := r.Peek(1)
		if err != nil || len(b) == 0 {
			return
		}
		if c := b[0]; c != '\r' && c != '\n' && c != 0 {
			return
		}
		r.ReadByte()
	}
}

// toCRLF translates a lone "\n" to "\r\n", leaving existing "\r\n" alone.
// Both Stdio and the raw-mode Console need this: no terminal is doing output
// post-processing for them, so a bare LF would otherwise stair-step.
func toCRLF(p []byte) []byte {
	out := make([]byte, 0, len(p)+16)
	var prev byte
	for _, b := range p {
		if b == '\n' && prev != '\r' {
			out = append(out, '\r')
		}
		out = append(out, b)
		prev = b
	}
	return out
}
