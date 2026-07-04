// Package session defines the byte-stream abstraction between the game
// engine and whatever the player is connected through. The engine reads
// keypresses from, and writes ANSI bytes to, a Session; it never knows
// whether that is a local console, a BBS socket, or a websocket.
package session

import "io"

type Session interface {
	// ReadKey returns a single keypress. BRE uses a hotkey model: one
	// letter acts immediately, no Enter required.
	ReadKey() (rune, error)
	// Writer receives ANSI bytes to display to the player.
	io.Writer
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
