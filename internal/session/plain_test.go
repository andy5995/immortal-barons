package session

import (
	"bytes"
	"testing"
)

// capture is a Session that records what was written and advertises nothing, so
// it stands in for a plain door stream.
type capture struct{ buf bytes.Buffer }

func (c *capture) ReadKey() (rune, error)      { return 0, nil }
func (c *capture) Write(p []byte) (int, error) { return c.buf.Write(p) }

func TestPlainWriterStrips(t *testing.T) {
	c := &capture{}
	s := NewPlain(c)
	in := "\x1b[H\x1b[2K\x1b[96mHelp\x1b[0m\n"
	n, err := s.Write([]byte(in))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	// The io.Writer contract is about the caller's slice, not the shorter one
	// that reached the wire.
	if n != len(in) {
		t.Errorf("Write returned %d, want %d", n, len(in))
	}
	if got := c.buf.String(); got != "Help\n" {
		t.Errorf("wrote %q, want %q", got, "Help\n")
	}
}

func TestHasANSI(t *testing.T) {
	c := &capture{}
	if !HasANSI(c) {
		t.Error("a session that advertises nothing should be treated as ANSI-capable")
	}
	if HasANSI(NewPlain(c)) {
		t.Error("the plain writer should report no ANSI")
	}
	// Either wrapping order must keep BOTH markers visible: each wrapper forwards
	// the other's, so the front-end can compose them in whichever order.
	if HasANSI(NewCP437Writer(NewPlain(c))) || HasANSI(NewPlain(NewCP437Writer(c))) {
		t.Error("CP437 wrapping should not hide the no-ANSI marker")
	}
	if IsUTF8(NewCP437Writer(NewPlain(c))) || IsUTF8(NewPlain(NewCP437Writer(c))) {
		t.Error("plain wrapping should not hide the CP437 marker")
	}
}
