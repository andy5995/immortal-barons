package menu

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// TestPrintScoresBREHeader checks the local scores board matches BRE's layout:
// the game-name banner, BRE's column labels/order (Id, Empire Name, Territory,
// Score, Net Worth), lettered [A]/[B] ids, and DEAD in the TERRITORY column of
// an eliminated empire — where BRE puts it (cap/kd3-01.cap), rather than as a
// suffix on the name.
func TestPrintScoresBREHeader(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	w.With(func() {
		for _, e := range w.Empires {
			if e != w.Player() {
				e.Alive = false
				break
			}
		}
	})
	printScores(f, w)
	out := f.out.String()
	plain := stripANSI(out)
	for _, want := range []string{"Immortal Barons", "Id", "Empire Name", "Territory", "Score", "Net Worth", "(A)"} {
		if !strings.Contains(plain, want) {
			t.Errorf("scores output missing %q:\n%s", want, plain)
		}
	}
	// DEAD stands in the territory column, so it is followed by the two figures
	// and preceded by the name — asserting the bare word would pass on a suffix.
	if !regexp.MustCompile(`\(A\)[^\n]+ DEAD +\d`).MatchString(plain) {
		t.Errorf("DEAD is not in the territory column:\n%s", plain)
	}
	if strings.Contains(plain, "(dead)") {
		t.Errorf("the name still carries a (dead) suffix:\n%s", plain)
	}
	if strings.Contains(out, "Rank") || strings.Contains(out, "Land") {
		t.Errorf("scores output still uses old labels:\n%s", out)
	}
}

// cp437Buf is a Session that only collects bytes, so NewCP437Writer can encode
// a rendered screen into it and the test can measure what really goes on the
// wire rather than the UTF-8 the engine wrote.
type cp437Buf struct{ buf bytes.Buffer }

func (c *cp437Buf) Write(p []byte) (int, error) { return c.buf.Write(p) }
func (c *cp437Buf) ReadKey() (rune, error)      { return 0, io.EOF }

// TestScoreRowWidthOnCP437 holds every row of the scores table to the width of
// its heading on a CP437 caller — the default charset. A name may carry any
// printable rune, and a dozen of them (±, °, ½, ß, Σ, ≤ …) are one CP437 byte
// but several ASCII characters; measuring by the ASCII rendering padded the
// name cell short and walked the figures left of their headings.
func TestScoreRowWidthOnCP437(t *testing.T) {
	out := &cp437Buf{}
	s := session.NewCP437Writer(out)
	w := newWorld()
	w.Term = Term{}
	w.With(func() {
		for _, e := range w.Empires {
			e.Name = "±Tatooine"
		}
	})
	printScores(s, w)

	var head int
	for _, ln := range bytes.Split(out.buf.Bytes(), []byte("\n")) {
		ln = bytes.TrimRight(ansiEsc.ReplaceAll(ln, nil), "\r")
		switch {
		case bytes.Contains(ln, []byte("Net Worth")):
			head = len(ln)
		case bytes.Contains(ln, []byte("Tatooine")):
			if head == 0 {
				t.Fatal("row came before the heading")
			}
			if len(ln) != head {
				t.Errorf("CP437 row is %d columns, heading is %d:\n%q", len(ln), head, ln)
			}
		}
	}
	if head == 0 {
		t.Fatal("no heading in the output")
	}
}
