package menu

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// capture is the Join Group Attack screen exactly as the original draws it,
// from cap/eots-ibbs-02.cap with the ANSI stripped. It is the contract this
// table has to meet: every column width, every alignment and the abbreviated
// figures. Trailing blanks are cut, as they are off the capture's own lines.
const (
	gaCapturedHead = "Id│By│    Planet    │ Individual Target │Troopers│ Jets │ Tanks │Bombers│ Leave"
	gaCapturedRule = "──┼──┼──────────────┼───────────────────┼────────┼──────┼───────┼───────┼──────"
	gaCapturedRow  = " 1│A │ The Eclipse  │  Land of Dreams   │   118k │4500k │ 6698k │   25k │   2h"
	// A party aimed at the whole planet, and a three-digit wait, from
	// cap/20240527-134Pho_Lazarus_Public.cap.
	gaCapturedAllRow = " 3│A │The Undermine │        ALL        │      0 │    0 │ 1000k │     0 │  12h"
)

// TestGroupAttackTableMatchesTheCapture draws the captured row and compares it
// to the original's own output, character for character.
func TestGroupAttackTableMatchesTheCapture(t *testing.T) {
	fs := &fakeSession{}
	row := gaRow{
		id: 1, by: "A", planet: "The Eclipse", target: "Land of Dreams",
		troopers: 118_000, jets: 4_500_000, tanks: 6_698_000, bombers: 25_000, hours: 2,
	}
	printGroupAttackTable(fs, Term{UTF8: true}, []gaRow{row})

	got := strings.Split(strings.TrimLeft(stripANSI(fs.out.String()), "\n"), "\n")
	want := []string{gaCapturedHead, gaCapturedRule, gaCapturedRow}
	if len(got) < len(want) {
		t.Fatalf("table has %d lines, want at least %d:\n%q", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], w)
		}
	}
	// 79 columns with the separators, which is what keeps it off an 80-column
	// terminal's wrap.
	if n := len([]rune(gaCapturedRule)); n != 79 {
		t.Errorf("table width = %d, want 79", n)
	}

	// The whole-planet party from the second capture: a 13-column board name
	// filling its cell exactly, ALL where a baron would be named, and a wait in
	// three digits.
	fs = &fakeSession{}
	printGroupAttackRow(fs, Term{UTF8: true}, gaRow{
		id: 3, by: "A", planet: "The Undermine", target: "ALL", tanks: 1_000_000, hours: 12,
	})
	if got := strings.TrimRight(stripANSI(fs.out.String()), "\n"); got != gaCapturedAllRow {
		t.Errorf("whole-planet row:\n got %q\nwant %q", got, gaCapturedAllRow)
	}
}

// TestGroupAttackTableClipsAnOverlongName holds the columns in place when a
// board or realm name is wider than its cell. Nothing caps either at the width
// of a cell, so an unclipped one would walk every figure to its right.
func TestGroupAttackTableClipsAnOverlongName(t *testing.T) {
	fs := &fakeSession{}
	printGroupAttackTable(fs, Term{UTF8: true}, []gaRow{{
		id: 9, by: "B", planet: strings.Repeat("W", 40), target: strings.Repeat("X", 40), hours: 12,
	}})
	lines := strings.Split(strings.TrimLeft(stripANSI(fs.out.String()), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected a head, a rule and a row, got %q", lines)
	}
	// Against the CAPTURED row's width, not the rule's: the original's own row
	// runs one column shorter, because the Leave figure is right-justified in
	// five of that column's six and the trailing blank falls off the line end.
	if got, want := len([]rune(lines[2])), len([]rune(gaCapturedRow)); got != want {
		t.Errorf("row is %d columns against the capture's %d: %q", got, want, lines[2])
	}
}

// TestGroupAttackLeaveIsWholeHours covers the Leave column's three cases: the
// wait rounds UP so a freshly filed attack reads its full delay, a force
// already due reads 0h, and a legacy attack that knows only its departure DAY
// reads "?" rather than a figure invented for it.
func TestGroupAttackLeaveIsWholeHours(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		ga   game.GroupAttack
		want string
	}{
		{"just filed", game.GroupAttack{DepartAt: now.Add(8*time.Hour - time.Second)}, "8h"},
		{"an hour off", game.GroupAttack{DepartAt: now.Add(time.Hour)}, "1h"},
		{"already due", game.GroupAttack{DepartAt: now.Add(-time.Hour)}, "0h"},
		{"legacy, day only", game.GroupAttack{DepartDay: 4}, "?"},
	}
	for _, c := range cases {
		if got := gaLeave(hoursUntil(now, c.ga)); got != c.want {
			t.Errorf("%s: Leave = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestGroupAttackChoiceIsTheDisplayedId proves the prompt matches on the Id the
// table SHOWS — the party's slot — and hands back the GroupAttack.ID behind it,
// which is the number the join transaction re-resolves against. The two are
// different, so matching or returning the wrong one joins the wrong strike.
func TestGroupAttackChoiceIsTheDisplayedId(t *testing.T) {
	rows := []gaRow{{id: 2, attack: 4127}, {id: 3, attack: 4130}}
	for _, c := range []struct {
		typed string
		want  int
	}{
		{"2", 4127}, {"3", 4130},
	} {
		fs := &fakeSession{keys: []rune(c.typed + "\r")}
		got, id := promptGroupChoice(fs, rows)
		if got != c.want {
			t.Errorf("typing %q selected attack %d, want %d", c.typed, got, c.want)
		}
		if strconv.Itoa(id) != c.typed {
			t.Errorf("typing %q reported back Id %d", c.typed, id)
		}
	}
	// The underlying id is NOT an answer: it is never on the screen, and a
	// player typing one is naming a slot no party here holds.
	fs := &fakeSession{keys: []rune("4127\r")}
	if got, _ := promptGroupChoice(fs, rows); got != 0 {
		t.Errorf("the world-wide id selected attack %d; it should cancel", got)
	}
	if !strings.Contains(stripANSI(fs.out.String()), "Join which group?") {
		t.Errorf("the original's prompt is missing: %q", fs.out.String())
	}
}

// TestJoinGroupAttackDrawsTheTableAndJoins runs the live screen end to end: it
// must REACH the table (the head is asserted, not merely "some output"), show
// the leader's pooled force in the unit columns, and record the join against
// the Id the table printed.
func TestJoinGroupAttackDrawsTheTableAndJoins(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Troopers = 10_000
		p.Protection = 0 // past new-realm protection so the join isn't gated
	})
	var slot int
	commitOnFile(t, cfg, func(w *game.World) {
		leader := w.AddHuman("leader", "Leaderland")
		leader.Troopers, leader.Jets = 500_000, 20_000
		w.GameDay = 0
		// A season's worth of ids already spent, so the party's own ID is four
		// digits while its slot is 1 — the case the two-column Id field exists
		// for, and the one where selecting by the wrong number joins nothing.
		w.NextAttackID = 4126
		ga, err := w.CreateGroupAttack(leader, "Mars", "", game.GroupAttackHoursMax,
			game.AttackForce{Troopers: 118_000, Jets: 4_500})
		if err != nil {
			t.Fatalf("seed group attack: %v", err)
		}
		if ga.ID == ga.Slot {
			t.Fatalf("this test needs the id and the slot to differ, got %d for both", ga.ID)
		}
		slot = ga.Slot
	})

	fs := &fakeSession{keys: []rune(strconv.Itoa(slot) + "\r500\r")}
	joinGroupAttack(fs, b)

	out := stripANSI(fs.out.String())
	if !strings.Contains(out, gaCapturedHead) {
		t.Fatalf("the table's head was never drawn:\n%s", out)
	}
	// The pooled force, abbreviated as the original abbreviates it -- not an
	// offense total, which is what this screen replaced.
	if !strings.Contains(out, "118k") || !strings.Contains(out, "4500") {
		t.Errorf("the committed force is missing from the columns:\n%s", out)
	}
	if !strings.Contains(out, "Mars") {
		t.Errorf("the target planet is missing from the columns:\n%s", out)
	}

	w := committedWorld(t, cfg)
	if n := len(w.GroupAttacks[0].Contributors); n != 2 {
		t.Fatalf("contributors = %d, want 2 — the join never landed", n)
	}
}
