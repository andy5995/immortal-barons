package game

import "testing"

// The loan interest is BRE-verified: three live loans matched these totals and
// overall-interest figures exactly, so lock them in.
func TestLoanMathMatchesBRE(t *testing.T) {
	cases := []struct {
		amount, days, wantOwed, wantOverallTenths int
	}{
		{1000, 2, 1175, 175},  // 8.4%/day → 17.5% overall
		{616, 5, 947, 539},    // 9.0%/day → 53.9% overall
		{500, 10, 1296, 1594}, // 10.0%/day → 159.4% overall
	}
	for _, c := range cases {
		if got := LoanTotalOwed(c.amount, c.days); got != c.wantOwed {
			t.Errorf("LoanTotalOwed(%d, %d) = %d, want %d (BRE-verified)", c.amount, c.days, got, c.wantOwed)
		}
		if got := LoanOverallTenths(c.days); got != c.wantOverallTenths {
			t.Errorf("LoanOverallTenths(%d) = %d, want %d", c.days, got, c.wantOverallTenths)
		}
	}
	// Daily rate rises 0.2%/day with the term: 2d→8.4, 5d→9.0, 10d→10.0 (tenths).
	for _, c := range []struct{ days, tenths int }{{2, 84}, {5, 90}, {10, 100}} {
		if got := LoanRateTenths(c.days); got != c.tenths {
			t.Errorf("LoanRateTenths(%d) = %d, want %d", c.days, got, c.tenths)
		}
	}
}

func TestTakeLoanAndDefault(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions, e.Land = RegionMix{Coastal: 40}, 0
	e.syncLand()
	e.Gold = 0
	w.GameDay = 5

	l, err := w.TakeLoan(e, 1000, 2)
	if err != nil {
		t.Fatalf("TakeLoan: %v", err)
	}
	if e.Gold != 1000 {
		t.Errorf("gold after borrowing: want 1000, got %d", e.Gold)
	}
	if l.Owed != 1175 || l.DueDay != 7 {
		t.Errorf("loan: owed=%d dueDay=%d, want 1175/7", l.Owed, l.DueDay)
	}

	// Can't borrow past the ceiling.
	if _, err := w.TakeLoan(e, w.LoanCeiling(e)+1, 2); err != ErrCantAfford {
		t.Errorf("over-ceiling loan: want ErrCantAfford, got %v", err)
	}

	// Default: spend the borrowed gold, then let the loan come due with nothing.
	e.Gold, e.Bank = 0, 0
	w.GameDay = 7
	beforeSupport := e.Support
	w.matureLoans(e)
	if len(e.Loans) != 0 {
		t.Errorf("loan should be cleared after maturing, got %d", len(e.Loans))
	}
	wantDebt := 1175 + 1175*LoanDefaultPenaltyPct/100
	if e.Debt != wantDebt {
		t.Errorf("defaulted debt: want %d, got %d", wantDebt, e.Debt)
	}
	if e.Support >= beforeSupport {
		t.Errorf("support should drop on default: was %d, now %d", beforeSupport, e.Support)
	}
}
