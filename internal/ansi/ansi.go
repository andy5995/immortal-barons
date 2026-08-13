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

	// WrapOff/WrapOn toggle DECAWM, the terminal's automatic line wrap. Full-width
	// art needs this: painting column 80 of an 80-column terminal wraps the cursor
	// by itself, and the row's own CR/LF then advances a SECOND time, leaving a
	// blank line between every row. Terminals differ on when that fires — xfce
	// defers the wrap until the next character, SyncTERM takes it immediately — so
	// art that looks right locally can come out banded over a BBS. With wrap off,
	// column 80 simply leaves the cursor where it is and CR/LF does the line break,
	// which behaves the same either way and at any width.
	WrapOff = esc + "?7l"
	WrapOn  = esc + "?7h"

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

// The screen a BBS caller is assumed to have. DOS text mode is 80x25 and that
// is what the original targeted, but the terminals reaching a door today are
// mostly VT100-derived and default to 24 rows, and a BBS often reserves the
// bottom row for its own status line. Designing to 24 therefore costs one row
// on a genuine DOS terminal and saves a scrolled-away header everywhere else,
// so a screen that must be read in one piece — the splash, a menu, a page of
// settings — is built to fit ScreenRows INCLUDING its own prompt.
const (
	ScreenCols = 80
	ScreenRows = 24
)
