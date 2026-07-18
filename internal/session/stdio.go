package session

import (
	"bufio"
	"os"

	"golang.org/x/term"
)

// Stdio is the Session used when running as a BBS door: the BBS pipes the
// caller's connection to our stdin/stdout. It translates a lone "\n" to
// "\r\n", since no terminal is doing that for us.
//
// Usually the stream is a socket/pipe and no tty handling applies. But some
// BBSes (Mystic on Linux) attach the door to a PTY instead — and if that pty
// is left in canonical mode, the kernel line-buffers and echoes: single-key
// prompts see nothing until Enter, and the caller watches their own keystrokes
// (and ^H backspaces) pile up on screen. So if stdin turns out to be a
// terminal, switch it to raw mode (no echo, byte-at-a-time) and restore at
// Close; a pipe/socket is left untouched, exactly as before.
type Stdio struct {
	r       *bufio.Reader
	restore func()
}

func NewStdio() *Stdio {
	var restore func()
	fd := int(os.Stdin.Fd())
	// Best effort: on a pipe/socket MakeRaw fails and we run as before.
	if st, err := term.MakeRaw(fd); err == nil {
		restore = func() { term.Restore(fd, st) }
	}
	return &Stdio{
		r:       bufio.NewReader(os.Stdin),
		restore: restore,
	}
}

// StdinIsTerminal reports whether the door's stdin is a terminal (a pty the
// BBS attached) rather than a pipe/socket — surfaced in the launch diagnostic.
func StdinIsTerminal() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

func (s *Stdio) ReadKey() (rune, error) {
	r, _, err := s.r.ReadRune()
	return r, err
}

func (s *Stdio) Write(p []byte) (int, error) {
	if _, err := os.Stdout.Write(toCRLF(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close restores a pty stdin to its normal mode. Safe to call once; a no-op
// for the usual pipe/socket case.
func (s *Stdio) Close() {
	if s.restore != nil {
		s.restore()
		s.restore = nil
	}
}
