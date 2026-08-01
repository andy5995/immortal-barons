// Package ansi holds the small set of ANSI escape sequences the menu
// system emits. Front-ends write these bytes verbatim to whatever stream
// the player is on (local console now; BBS socket / websocket later).
package ansi

const esc = "\x1b["

const (
	Reset     = esc + "0m"
	Dim       = esc + "2m"
	Reverse   = esc + "7m"             // reverse video — the moving highlight in a lightbar
	EraseLine = esc + "2K"             // erase the whole line (cursor row), for in-place redraw
	EraseDown = esc + "J"              // erase from cursor to end of screen
	Home      = esc + "H"              // cursor to top-left
	Clear     = esc + "2J" + esc + "H" // clear screen, cursor home

	FgBlack   = esc + "30m"
	FgRed     = esc + "31m"
	FgGreen   = esc + "32m"
	FgYellow  = esc + "33m"
	FgBlue    = esc + "34m"
	FgMagenta = esc + "35m"
	FgCyan    = esc + "36m"
	FgWhite   = esc + "37m"

	FgBrightBlack   = esc + "90m" // gray — BRE's rules and separators (its 1;30)
	FgBrightRed     = esc + "91m"
	FgBrightGreen   = esc + "92m"
	FgBrightYellow  = esc + "93m"
	FgBrightBlue    = esc + "94m"
	FgBrightMagenta = esc + "95m"
	FgBrightCyan    = esc + "96m"
	FgBrightWhite   = esc + "97m"

	BgBlue        = esc + "44m"
	BgBlack       = esc + "40m"
	BgBrightBlack = esc + "100m"      // dark gray
	BgHeader      = esc + "48;5;233m" // darkest gray, for zebra table header rows (white text contrast)
	BgRow         = esc + "48;5;236m" // darker gray (256-color), for zebra table value rows
	BgShadow      = esc + "48;5;238m" // dark-but-not-black gray, for a raised right-edge shadow
)
