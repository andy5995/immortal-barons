package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// A column fills a cell per 20% of the per-date limit, and a remainder caps it
// with a partly filled cell, so a full column means a full date and anything
// due shows at least the cap.
func TestShadeCells(t *testing.T) {
	type cells struct {
		full int
		part bool
	}
	for pct, want := range map[int]cells{
		-5: {0, false}, 0: {0, false}, 1: {0, true}, 19: {0, true}, 20: {1, false}, 37: {1, true},
		40: {2, false}, 54: {2, true}, 97: {4, true}, 99: {4, true}, 100: {5, false}, 140: {5, false},
	} {
		if full, part := shadeCells(pct, investChartRows); full != want.full || part != want.part {
			t.Errorf("shadeCells(%d, %d) = %d, %v, want %d, %v", pct, investChartRows, full, part, want.full, want.part)
		}
	}
}

// The Bank menu draws the chart between its closing rule and the gold line,
// centered under the box with a % scale, a baseline and day labels, and
// leaves it out when nothing is invested.
func TestBankMenuDrawsTheInvestmentChart(t *testing.T) {
	w := newWorld()
	p := w.Player()
	day := w.World.GameDay
	p.Investments = []game.Investment{
		{Amount: 1, Return: 2_000_000_000, MaturesDay: day + 1},  // full
		{Amount: 1, Return: 740_000_000, MaturesDay: day + 3},    // 37%: 1 cell and a cap
		{Amount: 1, Return: 1_000, MaturesDay: day + 10},         // a sliver: a cap
		{Amount: 1, Return: 2_000_000_000, MaturesDay: day + 11}, // past the chart
	}
	f := &fakeSession{}
	draw(f, w, BuildMenus().Bank)
	got := strings.Split(sgr.ReplaceAllString(f.out.String(), ""), "\n")

	want := []string{
		"                   Investment returns by day",
		"          100% ▒▒▒",
		"               ▒▒▒",
		"           50% ▒▒▒",
		"               ▒▒▒     ░░░",
		"               ▒▒▒     ▒▒▒                         ░░░",
		"            0% ───────────────────────────────────────",
		"                1   2   3   4   5   6   7   8   9   10",
	}
	start := -1
	for i, l := range got {
		if l == want[0] {
			start = i
		}
	}
	if start < 1 || !strings.HasPrefix(got[start-1], "─") {
		t.Fatalf("chart heading not found under the closing rule:\n%s", strings.Join(got, "\n"))
	}
	for i := range want {
		if strings.TrimRight(got[start+i], " ") != want[i] {
			t.Errorf("chart line %d:\n got %q\nwant %q", i, got[start+i], want[i])
		}
	}
	if !strings.HasPrefix(got[start+len(want)], "You have ") {
		t.Errorf("gold line does not follow the chart: %q", got[start+len(want)])
	}

	// The hold counts as well: what tomorrow's maintenance pours onto a date
	// shows on it, and in gold like every other column.
	p.Investments = []game.Investment{{Amount: 1, Return: 1_000_000_000, MaturesDay: day + 1}}
	p.InvestHeld = 2_000_000_000
	f = &fakeSession{}
	draw(f, w, BuildMenus().Bank)
	out := f.out.String()
	got = strings.Split(sgr.ReplaceAllString(out, ""), "\n")
	for i, l := range got {
		if l == want[0] {
			start = i
		}
	}
	if l := strings.TrimRight(got[start+5], " "); l != "               ▒▒▒ ▒▒▒" {
		t.Errorf("hold not counted, bottom row %q", l)
	}
	if strings.Contains(out, ansi.FgBrightRed+"▒") || !strings.Contains(out, ansi.FgBrightYellow+"▒▒▒") {
		t.Error("columns not drawn in gold")
	}
	p.InvestHeld = 0

	p.Investments = []game.Investment{{Amount: 1, Return: 5, MaturesDay: day + 11}}
	f = &fakeSession{}
	draw(f, w, BuildMenus().Bank)
	if strings.Contains(f.out.String(), "Investment returns by day") {
		t.Error("chart drawn with nothing maturing in the next ten days")
	}
}
