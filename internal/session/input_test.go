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
