// Package ansi holds the small set of ANSI escape sequences the menu
// system emits. Front-ends write these bytes verbatim to whatever stream
// the player is on (local console now; BBS socket / websocket later).
package ansi

const esc = "\x1b["

const (
	Reset = esc + "0m"
	Dim   = esc + "2m"
	Clear = esc + "2J" + esc + "H" // clear screen, cursor home

	FgRed    = esc + "31m"
	FgGreen  = esc + "32m"
	FgYellow = esc + "33m"
	FgBlue   = esc + "34m"
	FgCyan   = esc + "36m"
	FgWhite  = esc + "37m"

	FgBrightYellow = esc + "93m"
	FgBrightCyan   = esc + "96m"
	FgBrightWhite  = esc + "97m"
)
