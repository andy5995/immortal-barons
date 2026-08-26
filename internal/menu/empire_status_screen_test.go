package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// statusLines renders the Empire Status block for a test world and returns its
// lines without colour, blank leading and trailing rows trimmed.
func statusLines(w *ctx) []string {
	out := sgr.ReplaceAllString(empireStatusBlock(&fakeSession{}, w), "")
	return strings.Split(strings.Trim(out, "\n"), "\n")
}

// The block follows the original's field order and its two-row layouts (see the
// capture in docs/dev/bre-screens.md). Golden text, so a reordered or renamed
// field has to be re-argued rather than silently accepted.
func TestEmpireStatusMatchesTheCapturedFieldOrder(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Name = "Testland"
	p.TurnsLeft, p.Score = 10, 4473
	p.Gold, p.Bank = 6_156_219, 7_255_312
	p.People, p.Tax, p.Support, p.Food = 3386, 12, 100, 6441
	p.Agents, p.HQ, p.SDI, p.Morale = 25, 40, 30, 100
	p.Troopers, p.Jets, p.Turrets = 42259, 300, 12
	p.Tanks, p.Bombers, p.Carriers = 4, 5, 6
	p.Regions = game.RegionMix{
		Coastal: 24, River: 14, Agricultural: 5, Desert: 4,
		Industrial: 3, Urban: 2, Mountain: 1, Technology: 7, Waste: 9,
	}
	p.Protection = 100

	want := []string{
		"─────══════════════───────────────────────────────────────────────────",
		"-*Testland*-",
		"Turns: 10",
		"Score: 4,473",
		"Gold: 6,156,219",
		"Bank: 7,255,312",
		"Population: 3,386 (Tax Rate: 12%)",
		"Popular Support: 100%",
		"Food: 6,441",
		"Agents: 25",
		"Headquarters: 40% Complete",
		"Military: [42,259 Troopers] [300 Jets] [12 Turrets]",
		"          [4 Tanks] [5 Bombers] [6 Carriers]",
		"          [100% Morale]",
		"SDI Strength: 30%",
		"Regions:  [24 Coastal] [14 River] [5 Agricultural] [4 Desert]",
		"          [3 Industrial] [2 Urban] [1 Mountain] [7 Technology]",
		"          [9 Waste]",
		"You have 100 turns of protection left.",
		"─────══════════════───────────────────────────────────────────────────",
	}
	got := statusLines(w)
	if len(got) != len(want) {
		t.Fatalf("block has %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}

// The original's colours, from a live capture: the title's `-*`/`*-` brackets
// bright cyan around a bright-white realm name, every figure bright cyan, the
// label text around it white.
func TestEmpireStatusUsesTheCapturedColours(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Name, p.TurnsLeft, p.Gold = "Testland", 10, 500
	out := empireStatusBlock(&fakeSession{}, w)

	for _, want := range []string{
		"\x1b[96m-*\x1b[97mTestland\x1b[96m*-\x1b[0m", // title
		"\x1b[37mTurns: \x1b[96m10\x1b[37m\x1b[0m",    // white label, bright-cyan figure
		"\x1b[96m500\x1b[37m",                         // gold reverts to white after the figure
	} {
		if !strings.Contains(out, want) {
			t.Errorf("block is missing %q:\n%q", want, out)
		}
	}
}

// The rule that brackets the block, as a golden literal off the capture: `34`
// blue, 70 columns, 5 single + 14 double + 51 single, drawn immediately above
// the title and immediately below the reign line. Written out in full rather
// than rebuilt from statusRuleWidth, so a retune has to bring new evidence.
func TestEmpireStatusIsBracketedByTheCapturedRule(t *testing.T) {
	w := newWorld()
	w.Player().Name = "Testland"
	out := empireStatusBlock(&fakeSession{}, w)

	const rule = "\x1b[34m─────══════════════───────────────────────────────────────────────────\x1b[0m"
	if n := strings.Count(out, rule); n != 2 {
		t.Fatalf("want the 70-column blue inset rule twice, got %d:\n%q", n, out)
	}
	if !strings.Contains(out, rule+"\n\x1b[96m-*") {
		t.Error("the rule should sit immediately above the -*realm*- title")
	}
	if !strings.HasSuffix(out, rule+"\n") {
		t.Error("the rule should close the block, immediately after the reign line")
	}
	// A rule above and below is not a box: nothing runs down the sides.
	if strings.ContainsAny(out, "│┌┐└┘") {
		t.Errorf("the status block draws no box:\n%q", out)
	}
}

// A field the realm does not have is left out, so the block sizes itself to the
// realm — the original omits every zero figure and prints "None" for no army.
func TestEmpireStatusOmitsZeroFields(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Bank, p.Food, p.Agents, p.HQ, p.SDI = 0, 0, 0, 0, 0, 0
	p.Troopers, p.Jets, p.Turrets, p.Tanks, p.Bombers, p.Carriers = 0, 0, 0, 0, 0, 0

	out := strings.Join(statusLines(w), "\n")
	for _, gone := range []string{"Gold:", "Bank:", "Food:", "Agents:", "Headquarters:", "SDI Strength:", "Morale"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q should be omitted at zero:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "Military: None") {
		t.Errorf("an empty army should read \"Military: None\":\n%s", out)
	}
}

// The System menu's Empire Status item still lands on the block. The script
// asserts a marker unique to that screen AND the presence stamp the action
// leaves behind: when the keys run dry the session ends cleanly, so a re-mapped
// hotkey upstream would otherwise leave this green having drawn nothing.
func TestSystemMenuReachesEmpireStatus(t *testing.T) {
	menus := BuildMenus()
	// 'E' is Empire Status on the System menu, RETURN dismisses its pause, '0' quits.
	f, w, err := run(t, "E\r0", menus.System)
	if err != nil {
		t.Fatalf("session ended with %v", err)
	}
	if !strings.Contains(f.out.String(), "Popular Support:") {
		t.Fatalf("script never reached Empire Status; output was:\n%s", f.out.String())
	}
	var p *game.Empire
	w.With(func() { p = w.Player() })
	if p.LastActive == 0 {
		t.Error("viewing the status left no presence stamp, so the action never ran")
	}
}

// A treasury past a billion reaches the block in full, grouped, the way BRE
// prints its own `Bank: 2,000,000,000`. The formatter is unit-tested on its
// own; this pins that the block actually routes gold through it.
func TestEmpireStatusPrintsBillionsInFull(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Bank = 1_847_392_104, 12_500_000_000

	out := strings.Join(statusLines(w), "\n")
	for _, want := range []string{"Gold: 1,847,392,104", "Bank: 12,500,000,000"} {
		if !strings.Contains(out, want) {
			t.Errorf("block is missing %q:\n%s", want, out)
		}
	}
}
