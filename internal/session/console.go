package session

import (
	"bufio"
	"os"

	"golang.org/x/term"
)

// Console is the local-terminal Session: keypresses from stdin, ANSI to
// stdout. It puts the terminal in raw mode via x/term so ReadKey returns one
// character at a time with no echo, uniformly across Linux, macOS, the BSDs,
// and Windows. Raw mode disables the terminal's output post-processing, so
// Write translates a lone "\n" to "\r\n" itself (as Stdio does).
//
// Raw mode also means Ctrl-C no longer raises SIGINT; the player quits through
// the menu instead. This matches how door play already behaves (the BBS hands
// us a raw stream).
type Console struct {
	r       *bufio.Reader
	restore func()
}

func NewConsole() *Console {
	var restore func()
	fd := int(os.Stdin.Fd())
	// Best effort: if stdin is not a terminal (e.g. piped input in tests),
	// MakeRaw fails and we simply run without raw mode.
	if st, err := term.MakeRaw(fd); err == nil {
		restore = func() { term.Restore(fd, st) }
	}
	return &Console{
		r:       bufio.NewReader(os.Stdin),
		restore: restore,
	}
}

func (c *Console) ReadKey() (rune, error) {
	r, _, err := c.r.ReadRune()
	return r, err
}

// DrainInput drops a line terminator left buffered after a single-key answer
// (see InputDrainer).
func (c *Console) DrainInput() { drainTerminators(c.r) }

func (c *Console) Write(p []byte) (int, error) {
	if _, err := os.Stdout.Write(toCRLF(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close restores the terminal to its normal mode. Safe to call once.
func (c *Console) Close() {
	if c.restore != nil {
		c.restore()
		c.restore = nil
	}
}
