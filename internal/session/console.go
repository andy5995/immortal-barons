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
	r         *bufio.Reader
	restore   func()
	restoreVT func()
	plain     bool // console can't render ANSI: strip it rather than show it
}

func NewConsole() *Console {
	var restore func()
	fd := int(os.Stdin.Fd())
	// Best effort: if stdin is not a terminal (e.g. piped input in tests),
	// MakeRaw fails and we simply run without raw mode.
	if st, err := term.MakeRaw(fd); err == nil {
		restore = func() { term.Restore(fd, st) }
	}
	// A Windows console prints our ANSI literally until asked not to, and the
	// legacy console cannot be asked at all — there, send plain text (issue #98).
	vtOK, restoreVT := EnableVirtualTerminal()
	return &Console{
		r:         bufio.NewReader(os.Stdin),
		restore:   restore,
		restoreVT: restoreVT,
		plain:     !vtOK,
	}
}

// ANSI reports whether this console interprets escape sequences. False on a
// legacy Windows console, where Write strips them (issue #98) and the menu code
// offers numbered lists in place of lightbars (issue #99).
func (c *Console) ANSI() bool { return !c.plain }

// SetPlain forces the escapes off regardless of what the console reports, for
// -no-ansi (testing the non-ANSI path on a terminal that is perfectly capable).
func (c *Console) SetPlain() { c.plain = true }

func (c *Console) ReadKey() (rune, error) {
	r, _, err := c.r.ReadRune()
	return r, err
}

// ReadKeyByte returns the next raw byte, undecoded (see KeyByteReader).
func (c *Console) ReadKeyByte() (byte, error) { return c.r.ReadByte() }

// DrainInput drops a line terminator left buffered after a single-key answer
// (see InputDrainer).
func (c *Console) DrainInput() { drainTerminators(c.r) }

func (c *Console) Write(p []byte) (int, error) {
	out := toCRLF(p)
	if c.plain {
		out = stripEscapes(out)
	}
	if _, err := os.Stdout.Write(out); err != nil {
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
	if c.restoreVT != nil {
		c.restoreVT()
		c.restoreVT = nil
	}
}
