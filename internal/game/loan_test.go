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

func TestTakeLoanAndCollection(t *testing.T) {
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

	// Due: the loan moves into Debt and the bank sets a per-turn installment of
	// the balance over the turns per day. Nothing is taken from gold or savings
	// at maintenance, and support is untouched.
	e.Gold, e.Bank = 0, 5_000
	w.GameDay = 7
	beforeSupport := e.Support
	w.matureLoans(e)
	if len(e.Loans) != 0 {
		t.Errorf("loan should leave the pending list once due, got %d", len(e.Loans))
	}
	if e.Debt != 1175 || e.LoanInstallment != 117 {
		t.Errorf("due loan: Debt=%d installment=%d, want 1175 and 117", e.Debt, e.LoanInstallment)
	}
	if e.Bank != 5_000 || e.Support != beforeSupport {
		t.Errorf("maturity took savings or support: bank=%d support %d->%d", e.Bank, beforeSupport, e.Support)
	}

	// No gold, no payment — the bank never touches savings.
	if paid := w.collectLoanInstallment(e); paid != 0 || e.Bank != 5_000 {
		t.Errorf("broke realm paid %d (bank %d), want 0 and savings untouched", paid, e.Bank)
	}
	// A short purse pays what it holds.
	e.Gold = 50
	if paid := w.collectLoanInstallment(e); paid != 50 || e.Gold != 0 || e.Debt != 1125 {
		t.Errorf("short purse: paid=%d gold=%d debt=%d, want 50, 0, 1125", paid, e.Gold, e.Debt)
	}
	// Nine full installments leave 1125 - 9x117 = 72 at the end of the day.
	e.Gold = 10_000
	for range 9 {
		w.collectLoanInstallment(e)
	}
	if e.Debt != 72 {
		t.Fatalf("after nine installments Debt=%d, want 72", e.Debt)
	}
	// The unpaid 72 grows by max(5.0, 5.0) + 6.0 = 11.0% overnight, truncated,
	// and the installment floor of 100 then clears it in one turn.
	w.GameDay = 8
	w.matureLoans(e)
	if e.Debt != 79 || e.LoanInstallment != 100 {
		t.Errorf("next day: Debt=%d installment=%d, want 79 and 100", e.Debt, e.LoanInstallment)
	}
	if paid := w.collectLoanInstallment(e); paid != 79 || e.Debt != 0 {
		t.Errorf("final turn paid %d leaving %d, want 79 and 0", paid, e.Debt)
	}
}

// An unpaid balance grows 11.0% a day on a board at 5.0%, and by the higher of
// the two bank rates plus 6.0 points in general (run_daily_maintenance 0x8e0c).
func TestOverdueLoanGrowth(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.InterestRate, w.InvestRate = 50, 50
	if got := w.growOverdue(1_000); got != 1_110 {
		t.Errorf("1,000 at 11.0%%: got %d, want 1,110", got)
	}
	if got := w.growOverdue(1_175); got != 1_304 {
		t.Errorf("1,175 at 11.0%%: got %d, want 1,304", got)
	}
	w.InvestRate = 72 // the investment rate is higher, so it sets the pace
	if got := w.growOverdue(1_000); got != 1_132 {
		t.Errorf("1,000 at 13.2%%: got %d, want 1,132", got)
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

// A debt left unpaid for years stays positive and stops at the money cap.
// Unbounded, growth this steep wraps int64 within a few hundred days.
func TestDebtGrowthIsBounded(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Debt = 1_000
	for day := 0; day < 2_000; day++ {
		e.Debt = w.growOverdue(e.Debt)
		if e.Debt <= 0 {
			t.Fatalf("day %d: debt went non-positive: %d", day, e.Debt)
		}
	}
	if e.Debt != 2_000_000_000 {
		t.Errorf("debt settled at %d, want the 2,000,000,000 cap", e.Debt)
	}
}
