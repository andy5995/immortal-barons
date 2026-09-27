package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// List Investments / Loans is laid out as run_bank lays it out (BRE.OVR
// 0x39A0C-0x39EA0). Golden text: the widths, the "   $" cells, the blank
// investment cell that keeps the loan column in place, the hold in thousands
// printed with ",000" and sized by its thousands, In Hold last, and the
// 45-column inset rule above and below the rows.
// listRuleText is the captured rule, spelled out rather than built from the
// constants so a change to them fails here.
var listRuleText = strings.Repeat("─", 5) + strings.Repeat("═", 9) + strings.Repeat("─", 31) + "\n"

func TestListInvestmentsMatchesTheOriginalsLayout(t *testing.T) {
	w := newWorld()
	w.World.Today, w.World.GameDay = "2026-09-27", 10
	p := w.Player()
	p.InvestDue, p.Debt, p.InvestHeld = 1_234_567, 0, 12_000
	p.Investments = []game.Investment{{Amount: 1, Return: 5_000_000, MaturesDay: 13}}
	p.Loans = []game.Loan{{Owed: 2_000, DueDay: 13}, {Owed: 750_000, DueDay: 15}}
	f := &fakeSession{keys: []rune(" ")}

	listInvestments(f, w)

	want := "\nDate         Investments         Loans Due\n" + listRuleText +
		"Today        $1,234,567       \n" +
		"09/30/2026   $5,000,000          $  2,000\n" +
		"10/02/2026                       $750,000\n" +
		"In Hold      $   12,000       \n" + listRuleText
	if out := stripANSI(f.out.String()); !strings.HasPrefix(out, want) {
		t.Fatalf("got:\n%q\nwant prefix:\n%q", out, want)
	}
}

// With nothing to list, the original still draws the header and the rules; it
// has no "nothing here" line.
func TestListInvestmentsEmptyDrawsTheFrame(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.InvestDue, p.Debt, p.InvestHeld, p.Investments, p.Loans = 0, 0, 0, nil, nil
	f := &fakeSession{keys: []rune(" ")}

	listInvestments(f, w)

	want := "\nDate         Investments         Loans Due\n" + listRuleText + listRuleText
	if out := stripANSI(f.out.String()); !strings.HasPrefix(out, want) {
		t.Fatalf("got:\n%q\nwant prefix:\n%q", out, want)
	}
}

// A hold whose thousands are narrower than the ",000" still reads as one
// figure: BRE's write ignores a width too small for the string, where Go's
// %*s would left-justify on the negative width and split it as "5  ,000".
func TestListInvestmentsSmallHoldStaysWhole(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.InvestDue, p.Debt, p.InvestHeld, p.Investments, p.Loans = 0, 0, 5_000, nil, nil
	f := &fakeSession{keys: []rune(" ")}

	listInvestments(f, w)

	if out := stripANSI(f.out.String()); !strings.Contains(out, "In Hold      $5,000") {
		t.Fatalf("want the hold as one figure:\n%s", out)
	}
}

// A translated label longer than BRE's ten columns widens the label column for
// every row, header included, so the figures stay in line.
func TestListInvestmentsLongLabelKeepsColumns(t *testing.T) {
	w := newWorld()
	w.World.Today, w.World.GameDay = "2026-09-27", 10
	p := w.Player()
	p.Language = "de"
	p.InvestDue, p.Debt, p.InvestHeld, p.Loans = 0, 0, 12_000, nil
	p.Investments = []game.Investment{{Amount: 1, Return: 5_000_000, MaturesDay: 13}}
	f := &fakeSession{keys: []rune(" ")}

	listInvestments(&langSession{Session: f, c: w}, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Einbehalten") {
		t.Fatalf("the German hold label (11 wide) should be on screen:\n%s", out)
	}
	var dollars []int
	for _, l := range strings.Split(out, "\n") {
		if i := strings.Index(l, "$"); i >= 0 {
			dollars = append(dollars, len([]rune(l[:i])))
		}
	}
	if len(dollars) != 2 || dollars[0] != dollars[1] {
		t.Fatalf("the $ columns should line up, got %v:\n%s", dollars, out)
	}
}
