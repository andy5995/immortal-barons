package session

import (
	"strings"
	"testing"
)

// A held key must not grow the line buffer without bound: a caller over a
// socket can stream keystrokes for the whole session.
func TestReadLineStopsGrowingAtTheCap(t *testing.T) {
	keys := []rune(strings.Repeat("a", 5000) + "\r")
	got, err := ReadLine(&scriptedKeys{keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(got)) != LineMaxRunes {
		t.Fatalf("line is %d runes, want %d", len([]rune(got)), LineMaxRunes)
	}
}

// Backspace still works at the cap, so a caller who hit it can correct the line.
func TestReadLineBackspaceAtTheCap(t *testing.T) {
	keys := []rune(strings.Repeat("a", LineMaxRunes+10) + "\bb\r")
	got, err := ReadLine(&scriptedKeys{keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "ab") || len([]rune(got)) != LineMaxRunes {
		t.Fatalf("got %d runes ending %q", len([]rune(got)), got[len(got)-2:])
	}
}

// TestKillLineErasesTheWholeField covers Ctrl-U at an ordinary prompt: the
// buffer empties, the echo is walked back one erase per rune, and typing
// continues on the cleared line rather than the prompt ending.
func TestKillLineErasesTheWholeField(t *testing.T) {
	s := &scriptedKeys{keys: append([]rune("1000000000"), KillLine, '2', '5', '\r')}
	got, err := ReadLine(s)
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "25" {
		t.Errorf("line = %q, want %q", got, "25")
	}
	if n := strings.Count(s.out.String(), "\b \b"); n != 10 {
		t.Errorf("erased %d runes, want 10 — one per echoed rune", n)
	}
}

// TestKillLineOnAnEmptyFieldIsANoOp proves Ctrl-U neither ends the prompt nor
// walks the cursor back over the prompt text itself when nothing is typed.
func TestKillLineOnAnEmptyFieldIsANoOp(t *testing.T) {
	s := &scriptedKeys{keys: []rune{KillLine, 'x', '\r'}}
	got, err := ReadLine(s)
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "x" {
		t.Errorf("line = %q, want %q", got, "x")
	}
	if strings.Contains(s.out.String(), "\b") {
		t.Errorf("an empty field erased something: %q", s.out.String())
	}
}

// TestKillLineErasesColumnsNotRunes is why the erase measures rather than
// counts. On a CP437 session "…" leaves the writer as three dots and "—" as two
// hyphens, so a rune-per-erase would walk the cursor back three columns for
// eight and leave the rest of the answer on screen. ValidRealmName accepts both
// characters, so this is reachable from the realm-name prompt.
func TestKillLineErasesColumnsNotRunes(t *testing.T) {
	typed := "a…b—c" // 5 runes, 8 CP437 columns
	inner := &scriptedKeys{keys: append([]rune(typed), KillLine, '\r')}
	s := NewCP437Writer(NewColumnTracker(inner, false))
	if got, err := ReadLine(s); err != nil || got != "" {
		t.Fatalf("ReadLine = %q, %v; want an empty line", got, err)
	}
	if n := strings.Count(inner.out.String(), "\b \b"); n != 8 {
		t.Errorf("erased %d columns, want 8 — the writer expands … and —", n)
	}
}

// A UTF-8 session renders each of those in one column, so the same answer
// erases in five. The measure has to follow the session, not the string.
func TestKillLineOnAUTF8SessionErasesRunes(t *testing.T) {
	typed := "a…b—c"
	inner := &scriptedKeys{keys: append([]rune(typed), KillLine, '\r')}
	if _, err := ReadLine(inner); err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if n := strings.Count(inner.out.String(), "\b \b"); n != 5 {
		t.Errorf("erased %d columns, want 5", n)
	}
}

// The single-key Backspace measures the same way: it erased one column per rune
// before Ctrl-U gave it a measure to share.
func TestBackspaceErasesColumnsNotRunes(t *testing.T) {
	inner := &scriptedKeys{keys: []rune{'a', '…', 8, '\r'}}
	got, err := ReadLine(NewCP437Writer(NewColumnTracker(inner, false)))
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "a" {
		t.Errorf("line = %q, want %q", got, "a")
	}
	if n := strings.Count(inner.out.String(), "\b \b"); n != 3 {
		t.Errorf("erased %d columns for one … , want 3", n)
	}
}

// The expander is the outermost wrapper on a live session, so a measure taken
// at a prompt goes through it. Without the forwarding it reports UTF-8 for a
// CP437 caller and every erase comes out short.
func TestMacroExpanderForwardsTheCharset(t *testing.T) {
	m := NewMacroExpander(NewCP437Writer(NewColumnTracker(&scriptedKeys{}, false)), func(string) (string, bool) { return "", false })
	if IsUTF8(m) {
		t.Error("a CP437 session reported UTF-8 through the macro expander")
	}
	if _, ok := Column(m); !ok {
		t.Error("the cursor column did not reach through the macro expander")
	}
}
