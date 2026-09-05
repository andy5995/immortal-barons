package session

import (
	"strings"
	"testing"
)

// TestColumnTrackerCounts holds the counter to what a terminal actually does
// with each kind of byte. Each case is written from column 0.
func TestColumnTrackerCounts(t *testing.T) {
	cases := []struct {
		name  string
		utf8  bool
		write string
		want  int
	}{
		{"plain text", false, "abc", 3},
		{"a colour escape occupies nothing", false, "\x1b[1;33mabc\x1b[0m", 3},
		{"a newline starts the line over", false, "abcdef\nxy", 2},
		{"a carriage return does too", false, "abcdef\rxy", 2},
		{"backspace walks back", false, "abc\b\b", 1},
		{"backspace stops at the left margin", false, "a\b\b\b\b", 0},
		{"a tab reaches the next stop", false, "ab\tc", 9},
		{"a bell prints nothing", false, "a\x07b", 2},
		// The case the charset made awkward: CP437 sends one byte per column,
		// so its high bytes each count, while UTF-8 sends several bytes for one.
		{"CP437 high bytes are columns", false, "a\xc4\xc4", 3},
		{"a UTF-8 rune is one column", true, "a—b", 3},
		{"a UTF-8 box glyph is one column", true, "││", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ct := NewColumnTracker(&scriptedKeys{}, c.utf8)
			if _, err := ct.Write([]byte(c.write)); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := ct.Column(); got != c.want {
				t.Errorf("column = %d, want %d", got, c.want)
			}
		})
	}
}

// TestEraseIsTheSameOnEitherCharset is the point of tracking the cursor rather
// than measuring the answer. The same five runes reach the terminal as five
// columns for a UTF-8 caller and eight for a CP437 one, because the writer
// expands two of them — and neither the prompt nor the editor above has to know
// which. Both erase back to exactly where the answer began.
func TestEraseIsTheSameOnEitherCharset(t *testing.T) {
	const typed = "a…b—c" // a…b—c

	for _, c := range []struct {
		name    string
		wrap    func(Session) Session
		columns int
	}{
		{"UTF-8", func(s Session) Session { return NewColumnTracker(s, true) }, 5},
		{"CP437", func(s Session) Session { return NewCP437Writer(NewColumnTracker(s, false)) }, 8},
	} {
		t.Run(c.name, func(t *testing.T) {
			// No Enter: the stream ends after the kill, so the cursor is left
			// where the erase put it rather than at the start of the next line.
			inner := &scriptedKeys{keys: append([]rune(typed), KillLine)}
			s := c.wrap(inner)
			// A prompt of some width first: the erase must stop at the answer,
			// not run back over the question.
			prompt := "Send how many Troopers? "
			if _, err := s.Write([]byte(prompt)); err != nil {
				t.Fatalf("prompt: %v", err)
			}
			if got, _ := ReadLine(s); got != "" {
				t.Fatalf("ReadLine = %q, want an empty line", got)
			}
			if n := strings.Count(inner.out.String(), "\b \b"); n != c.columns {
				t.Errorf("erased %d columns, want %d", n, c.columns)
			}
			// The cursor is back where the answer started, which is what a
			// following keystroke will be echoed at.
			if col, _ := Column(s); col != len(prompt) {
				t.Errorf("cursor left at column %d, want %d", col, len(prompt))
			}
		})
	}
}

// TestEraseBackFallsBackWithoutATracker keeps every prompt working on a session
// nothing tracks — the test doubles, and a screen rendered into a buffer for a
// bulletin file. There the fallback count is exact, since those carry ASCII.
func TestEraseBackFallsBackWithoutATracker(t *testing.T) {
	inner := &scriptedKeys{keys: append([]rune("1000000000"), KillLine, '\r')}
	if got, err := ReadLine(inner); err != nil || got != "" {
		t.Fatalf("ReadLine = %q, %v", got, err)
	}
	if n := strings.Count(inner.out.String(), "\b \b"); n != 10 {
		t.Errorf("erased %d columns, want 10", n)
	}
}
