package menu

import (
	"strings"
	"testing"
)

// Every translation of DEAD must fit the Territory column and right-align in
// it, so the two figures beside it stay where the eye expects them. Go's fmt
// pads %s by RUNE count, so a multi-byte word needs no special handling — what
// this guards is a translation LONGER than the column, which would push the
// figures right on that row alone, and any future change to byte-based padding.
func TestDeadCellKeepsTheColumnInEveryLanguage(t *testing.T) {
	row := func(lang string) string {
		w := newWorld()
		var dead string
		w.With(func() {
			for _, e := range w.Empires {
				e.Language = lang
				if e != w.Player() {
					e.Alive, dead = false, e.Name
				}
			}
		})
		base := &fakeSession{}
		printScores(&langSession{Session: base, c: w}, w)
		for _, l := range strings.Split(stripANSI(base.out.String()), "\n") {
			if strings.Contains(l, dead) {
				return l
			}
		}
		t.Fatalf("%s: no row for the dead realm", lang)
		return ""
	}

	// The territory cell is right-aligned, so its LAST rune sits at a fixed
	// offset whatever the word is. English is the reference.
	end := func(l string) int { return len([]rune(strings.TrimRight(l[:strings.Index(l, "  0")], " "))) }
	want := end(row("en"))
	for _, lang := range []string{"de", "nl", "ru", "pt"} {
		got := row(lang)
		if end(got) != want {
			t.Errorf("%s: territory cell ends at rune %d, want %d (a translation too long for the column?)\n%s",
				lang, end(got), want, got)
		}
	}
}
