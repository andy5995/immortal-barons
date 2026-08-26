package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// TestAttackPiratesMarksRaider checks that a faction which raided the player
// since their last play is FOLLOWED by the `«` mark, one space after the name
// (same color treatment as the online-baron mark), that a faction which did not
// raid carries nothing at all — no reserved indent, which is what #197 was about
// — and that the mark disappears once RaidersThisTurn no longer names the
// faction (the next recap's reset).
func TestAttackPiratesMarksRaider(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0             // past new-realm protection so the raid list shows
	p.RaidersThisTurn = []int{3} // Sharks (slot 3) raided this turn

	f := &fakeSession{keys: []rune("0\r")} // cancel at the faction prompt
	attackPirates(f, w)
	out := f.out.String()

	marked := ansi.FgRed + "Sharks" + ansi.Reset + " " + ansi.FgWhite + "«" + ansi.Reset
	if !strings.Contains(out, marked) {
		t.Errorf("raided faction should be followed by the mark, one space after the name; output:\n%s", out)
	}

	// Humans (slot 0) did not raid, so its row ends at the name — nothing
	// before it and nothing after it. The old layout reserved the mark's column
	// as a leading blank on every row, which is the indent #197 reported.
	unmarked := "1) " + ansi.FgBrightGreen + "Humans" + ansi.Reset + "\n"
	if !strings.Contains(out, unmarked) {
		t.Errorf("un-raided faction should carry no mark and no reserved indent; output:\n%s", out)
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

// A session held to 7-bit ASCII gets "<=" rather than the guillemet, which the
// ASCII writer would otherwise reduce to a bare "<". The shaft still takes the
// dark gray and the head the brighter one.
func TestAttackPiratesMarkOnAnASCIISession(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	p.RaidersThisTurn = []int{3} // Sharks

	f := &fakeSession{keys: []rune("0\r")}
	attackPirates(session.NewASCIIWriter(f), w)
	out := f.out.String()

	marked := "Sharks" + ansi.Reset + " " + ansi.FgWhite + "<" + ansi.FgBrightBlack + "=" + ansi.Reset
	if !strings.Contains(out, marked) {
		t.Errorf("an ASCII session should get the <= mark; output:\n%s", out)
	}
	if strings.Contains(out, "«") {
		t.Errorf("the guillemet reached an ASCII session; output:\n%s", out)
	}
}
