package session

import (
	"errors"
	"io"
)

// ErrNoInput is what a write-only session returns from ReadKey. A screen that
// asks for a keypress cannot be rendered to a file, and this says which screen
// tried rather than blocking forever or returning a key nobody pressed.
var ErrNoInput = errors.New("this session writes only and has no keyboard")

// writerSession turns any io.Writer into a Session, so a screen written for a
// player's terminal can be rendered to a file with no second copy of its layout
// (#233). It is the same seam the door and local front-ends use: the game
// writes ANSI to a stream and does not know what the stream is.
type writerSession struct{ w io.Writer }

// NewWriter returns a write-only Session over w. ReadKey always fails, which is
// the honest answer: there is nobody to press a key.
func NewWriter(w io.Writer) Session { return &writerSession{w: w} }

func (f *writerSession) Write(p []byte) (int, error) { return f.w.Write(p) }
func (f *writerSession) ReadKey() (rune, error)      { return 0, ErrNoInput }
