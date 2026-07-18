package session

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

// TestDrainInput checks that DrainInput drops a line terminator left buffered
// after a single-key answer (a telnet line-mode client sends "n\r\n" as one
// burst) while leaving a real typed-ahead key for the next read.
func TestDrainInput(t *testing.T) {
	cases := []struct {
		name, in  string
		wantAfter rune
	}{
		{"CRLF drained", "n\r\ny", 'y'},
		{"CR drained", "n\ry", 'y'},
		{"CR NUL drained", "n\r\x00y", 'y'},
		{"no terminator keeps type-ahead", "ny", 'y'},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Stdio{r: bufio.NewReader(strings.NewReader(c.in))}
			if k, _ := s.ReadKey(); k != 'n' {
				t.Fatalf("first ReadKey = %q, want 'n'", k)
			}
			s.DrainInput()
			k, err := s.ReadKey()
			if err != nil {
				t.Fatalf("second ReadKey: %v", err)
			}
			if k != c.wantAfter {
				t.Fatalf("after DrainInput, ReadKey = %q, want %q (terminator not dropped, or a real key eaten)", k, c.wantAfter)
			}
		})
	}
}

// TestDrainInputRoutesThroughDecorators guards against a forwarding gap: the
// drain must reach the base reader through the real door stack
// (macro -> deadline -> cp437 -> stdio), or the fix would be a silent no-op
// while every other test still passes.
func TestDrainInputRoutesThroughDecorators(t *testing.T) {
	base := &Stdio{r: bufio.NewReader(strings.NewReader("n\r\ny"))}
	var s Session = NewCP437Writer(base)
	s = NewDeadline(s, 0, 0, time.Time{}) // idle<=0 && zero hard -> ReadKey fast path
	s = NewMacroExpander(s, func(string) (string, bool) { return "", false })

	if k, _ := s.ReadKey(); k != 'n' {
		t.Fatalf("first ReadKey = %q, want 'n'", k)
	}
	d, ok := s.(interface{ DrainInput() })
	if !ok {
		t.Fatal("decorator stack does not expose DrainInput — a forwarding gap")
	}
	d.DrainInput()
	k, err := s.ReadKey()
	if err != nil {
		t.Fatalf("ReadKey after drain: %v", err)
	}
	if k != 'y' {
		t.Fatalf("after DrainInput through the stack, ReadKey = %q, want 'y' (a wrapper failed to forward)", k)
	}
}
