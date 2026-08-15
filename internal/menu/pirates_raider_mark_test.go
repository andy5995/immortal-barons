package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// TestAttackPiratesMarksRaider checks that a faction which raided the player
// since their last play carries the "->" mark (same color treatment as the
// online-baron mark), that a faction which did not raid keeps the mark's
// column reserved as blank rather than shifting its name left, and that the
// mark disappears once RaidersThisTurn no longer names the faction (the next
// recap's reset).
func TestAttackPiratesMarksRaider(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0             // past new-realm protection so the raid list shows
	p.RaidersThisTurn = []int{3} // Sharks (slot 3) raided this turn

	f := &fakeSession{keys: []rune("0\r")} // cancel at the faction prompt
	attackPirates(f, w)
	out := f.out.String()

	marked := ansi.FgBrightBlack + "-" + ansi.FgWhite + ">" + ansi.Reset + ansi.FgRed + "Sharks"
	if !strings.Contains(out, marked) {
		t.Errorf("raided faction should carry the ->mark hugging its name; output:\n%s", out)
	}

	// Humans (slot 0) did not raid: its name keeps the mark's column reserved
	// as two blanks, so it starts in the same place the marked row's name
	// does, rather than shifting left for lack of a mark.
	unmarked := "  " + ansi.FgBrightGreen + "Humans"
	if !strings.Contains(out, unmarked) {
		t.Errorf("un-raided faction should hold the mark's column as blank; output:\n%s", out)
	}
	if strings.Contains(out, ansi.FgBrightBlack+"-"+ansi.FgWhite+">"+ansi.Reset+ansi.FgBrightGreen+"Humans") {
		t.Errorf("un-raided faction should not carry the raid mark; output:\n%s", out)
	}

	// A faction not named in RaidersThisTurn — the next turn's recap — loses
	// the mark again.
	p.RaidersThisTurn = nil
	f2 := &fakeSession{keys: []rune("0\r")}
	attackPirates(f2, w)
	out2 := f2.out.String()
	if strings.Contains(out2, marked) {
		t.Errorf("mark should clear once RaidersThisTurn no longer names the faction; output:\n%s", out2)
	}
}

// A turn can carry more than one raid, and the raids can come from different
// factions — so the mark has to appear beside EVERY faction that hit, not just
// the first. raiderSlots collects the distinct set of slots; this pins that the
// screen honours all of them.
func TestAttackPiratesMarksEveryRaider(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	// Two different factions hit, and one of them twice — the repeat must not
	// produce a second entry or a doubled mark.
	p.RaidersThisTurn = raiderSlots([]game.PirateHit{{Slot: 2}, {Slot: 6}, {Slot: 2}})
	if len(p.RaidersThisTurn) != 2 {
		t.Fatalf("raiderSlots gave %v, want the two distinct slots", p.RaidersThisTurn)
	}

	f := &fakeSession{keys: []rune("0\r")}
	attackPirates(f, w)
	out := stripANSI(f.out.String())

	marked := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, pirateRaiderMark) {
			for _, name := range game.PirateFactions {
				if strings.Contains(line, name) {
					marked[name] = true
				}
			}
		}
	}
	for _, slot := range []int{2, 6} {
		if name := game.PirateFactions[slot]; !marked[name] {
			t.Errorf("%s raided but carries no mark; output:\n%s", name, out)
		}
	}
	if n := len(marked); n != 2 {
		t.Errorf("marked %d factions (%v), want exactly the two that raided", n, marked)
	}
}
