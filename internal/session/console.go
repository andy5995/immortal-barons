package session

import (
	"bufio"
	"os"
	"os/exec"
)

// Console is the local-terminal Session: keypresses from stdin, ANSI to
// stdout. It puts the terminal in cbreak+no-echo mode so ReadKey returns
// one character at a time without the player pressing Enter. Output
// processing (onlcr) is left on, so a bare "\n" still maps to CR-LF.
type Console struct {
	r       *bufio.Reader
	restore func()
}

func NewConsole() *Console {
	stty("cbreak", "-echo")
	return &Console{
		r:       bufio.NewReader(os.Stdin),
		restore: func() { stty("sane") },
	}
}

func (c *Console) ReadKey() (rune, error) {
	r, _, err := c.r.ReadRune()
	return r, err
}

func (c *Console) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close restores the terminal to its normal mode. Safe to call once.
func (c *Console) Close() {
	if c.restore != nil {
		c.restore()
		c.restore = nil
	}
}

func stty(args ...string) {
	cmd := exec.Command("stty", append([]string{"-F", "/dev/tty"}, args...)...)
	cmd.Run() // best effort; no TTY (e.g. piped input) just means no raw mode
}
