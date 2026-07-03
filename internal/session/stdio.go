package session

import (
	"bufio"
	"os"
)

// Stdio is the Session used when running as a BBS door: the BBS pipes the
// caller's connection to our stdin/stdout. Unlike Console it does no tty
// mode changes (the stream is a socket/pipe, not a terminal) and it
// translates a lone "\n" to "\r\n", since no terminal is doing that for us.
type Stdio struct {
	r *bufio.Reader
}

func NewStdio() *Stdio {
	return &Stdio{r: bufio.NewReader(os.Stdin)}
}

func (s *Stdio) ReadKey() (rune, error) {
	r, _, err := s.r.ReadRune()
	return r, err
}

func (s *Stdio) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p)+16)
	var prev byte
	for _, b := range p {
		if b == '\n' && prev != '\r' {
			out = append(out, '\r')
		}
		out = append(out, b)
		prev = b
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}
