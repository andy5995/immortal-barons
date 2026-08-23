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
		if got := LoanTotalOwed(int64(c.amount), c.days); got != int64(c.wantOwed) {
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
	if _, err := w.TakeLoan(e, w.LoanCeiling(e, 2)+1, 2); err != ErrCantAfford {
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
	wantDebt := int64(1175 + 1175*LoanDefaultPenaltyPct/100)
	if e.Debt != wantDebt {
		t.Errorf("defaulted debt: want %d, got %d", wantDebt, e.Debt)
	}
	if e.Support >= beforeSupport {
		t.Errorf("support should drop on default: was %d, now %d", beforeSupport, e.Support)
	}
}

// The ceiling is a DISCOUNT, not a multiple: BRE sizes what the realm will owe
// at maturity, so a longer term offers less (run_bank, BRE.OVR 0x38648). And net
// worth stops counting at 10,000,000, so the richest realm in the game borrows
// against the same headroom as a merely rich one. Both were invisible to the
// live sampling this replaced, which never varied the term.
func TestLoanCeilingDiscountsByTermAndCapsNetWorth(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")

	// Rich enough to sit above the net-worth cap several times over.
	e.Troopers = 100_000_000
	if nw := int64(w.NetWorth(e)); nw <= LoanCeilingNetWorthCap {
		t.Fatalf("net worth %d does not exceed the cap; the test proves nothing", nw)
	}

	// Capped: 10 x 10,000,000, then discounted by the term's compound factor.
	// Golden figures, not the constants — a retune must fail this and produce new
	// evidence, which is the point of a fidelity contract.
	for _, tc := range []struct{ days int }{{1}, {2}, {10}} {
		want := int64(100_000_000 / loanFactor(tc.days))
		if got := w.LoanCeiling(e, tc.days); got != want {
			t.Errorf("%d-day ceiling %d, want %d", tc.days, got, want)
		}
	}
	// A one-day term offers strictly more than a ten-day one.
	if short, long := w.LoanCeiling(e, 1), w.LoanCeiling(e, 10); short <= long {
		t.Errorf("1-day ceiling %d should beat the 10-day %d", short, long)
	}
	// 10 days at 10.0%/day compounds to about 2.59x, so the ceiling is well under
	// half the uncapped headroom.
	if got := w.LoanCeiling(e, 10); got > 40_000_000 {
		t.Errorf("10-day ceiling %d, want well under 40,000,000", got)
	}

	// What is already owed comes off before the discount.
	e.Debt = 60_000_000
	if got, want := w.LoanCeiling(e, 1), int64(40_000_000/loanFactor(1)); got != want {
		t.Errorf("ceiling with debt %d, want %d", got, want)
	}
	e.Debt = 200_000_000
	if got := w.LoanCeiling(e, 1); got != 0 {
		t.Errorf("a realm owing more than its headroom may borrow %d, want 0", got)
	}
}
