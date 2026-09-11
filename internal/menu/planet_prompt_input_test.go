package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// A click in SyncTERM sends an X10 mouse report — ESC [ M and three printable
// coordinate bytes. The planet prompt read raw keys, so the ESC vanished and
// `[M` plus the coordinates went into the field, over and over, with Ctrl-U
// unable to clear them because the loop had no kill case.
func TestPlanetPromptIgnoresMouseReports(t *testing.T) {
	planets := []game.LeagueNode{{Number: 1, Name: "Nova Hub"}, {Number: 2, Name: "The Eclipse"}}
	keys := []rune{}
	for range 3 { // three clicks: ESC [ M button col row
		keys = append(keys, 0x1b, '[', 'M', ' ', '!', '!')
	}
	keys = append(keys, []rune("ova\r")...) // 'N' is the peeked first key
	f := &fakeSession{keys: keys}

	got, err := readPlanetAnswer(f, planets, 'N')
	if err != nil {
		t.Fatalf("readPlanetAnswer: %v", err)
	}
	if got != "Nova Hub" {
		t.Errorf("answer = %q, want Nova Hub — the mouse bytes reached the field", got)
	}
	if strings.ContainsAny(f.out.String(), "[M") && strings.Contains(f.out.String(), "[M!") {
		t.Errorf("the mouse report was echoed:\n%s", f.out.String())
	}
}

// Ctrl-U starts the answer again, which is how a player recovers a field that
// has been filled with something they did not type.
func TestPlanetPromptCtrlUClearsTheField(t *testing.T) {
	planets := []game.LeagueNode{{Number: 1, Name: "Nova Hub"}, {Number: 2, Name: "The Eclipse"}}
	keys := append([]rune("XYZZY"), session.KillLine)
	keys = append(keys, []rune("Eclipse\r")...)
	f := &fakeSession{keys: keys}

	got, err := readPlanetAnswer(f, planets, 'Q') // a first key that matches nothing
	if err != nil {
		t.Fatalf("readPlanetAnswer: %v", err)
	}
	if got != "The Eclipse" {
		t.Errorf("answer = %q, want The Eclipse — Ctrl-U did not clear the field", got)
	}
}
