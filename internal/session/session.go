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
