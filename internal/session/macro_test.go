package session

import (
	"io"
	"testing"
)

// scriptedKeys is a minimal Session that returns a fixed sequence of runes,
// then io.EOF. Writes are discarded.
type scriptedKeys struct {
	keys []rune
	i    int
}

func (s *scriptedKeys) ReadKey() (rune, error) {
	if s.i >= len(s.keys) {
		return 0, io.EOF
	}
	r := s.keys[s.i]
	s.i++
	return r, nil
}

func (s *scriptedKeys) Write(p []byte) (int, error) { return len(p), nil }

// drain reads runes until EOF and returns them.
func drain(t *testing.T, s Session) []rune {
	t.Helper()
	var got []rune
	for {
		r, err := s.ReadKey()
		if err == io.EOF {
			return got
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, r)
	}
}

func TestMacroExpanderExpandsCtrlKey(t *testing.T) {
	// Ctrl-B (rune 2) followed by 'Z'. Ctrl-B maps to "XY".
	inner := &scriptedKeys{keys: []rune{2, 'Z'}}
	m := NewMacroExpander(inner, func(letter string) (string, bool) {
		if letter == "B" {
			return "XY", true
		}
		return "", false
	})
	got := string(drain(t, m))
	if got != "XYZ" {
		t.Errorf("want XYZ, got %q", got)
	}
}

func TestMacroExpanderPassesUnmappedCtrlKey(t *testing.T) {
	// Ctrl-A (rune 1) has no macro; it passes through unchanged (the menu
	// ignores it) rather than being swallowed.
	inner := &scriptedKeys{keys: []rune{1, 'A', 'B'}}
	m := NewMacroExpander(inner, func(string) (string, bool) { return "", false })
	got := drain(t, m)
	if len(got) != 3 || got[0] != 1 || got[1] != 'A' || got[2] != 'B' {
		t.Errorf("unmapped Ctrl-key should pass through, got %v", got)
	}
}

func TestMacroExpanderNeverInterceptsEnter(t *testing.T) {
	// Enter is Ctrl-M (rune 13). Even with a macro bound to "M", Enter must
	// pass through so ReadLine and single-key prompts still terminate.
	inner := &scriptedKeys{keys: []rune{'\r'}}
	m := NewMacroExpander(inner, func(letter string) (string, bool) {
		if letter == "M" {
			return "XX", true
		}
		return "", false
	})
	got := drain(t, m)
	if len(got) != 1 || got[0] != '\r' {
		t.Errorf("Enter must not be hijacked by a Ctrl-M macro, got %v", got)
	}
}

func TestMacroExpanderDoesNotRecurse(t *testing.T) {
	// A macro whose body contains a Ctrl-key rune must NOT re-expand: the
	// control rune is returned verbatim from the queue.
	inner := &scriptedKeys{keys: []rune{2}} // Ctrl-B
	m := NewMacroExpander(inner, func(letter string) (string, bool) {
		if letter == "B" {
			return string([]rune{1, 'Q'}), true // body: Ctrl-A, 'Q'
		}
		return "", false
	})
	got := drain(t, m)
	if len(got) != 2 || got[0] != 1 || got[1] != 'Q' {
		t.Errorf("macro body should replay verbatim, got %v", got)
	}
}

func TestMacroExpanderPassesPlainKeys(t *testing.T) {
	inner := &scriptedKeys{keys: []rune{'h', 'i'}}
	m := NewMacroExpander(inner, func(string) (string, bool) { return "", false })
	if got := string(drain(t, m)); got != "hi" {
		t.Errorf("want hi, got %q", got)
	}
}
