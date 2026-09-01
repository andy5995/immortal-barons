package game

import (
	"strings"
	"testing"
)

func TestInvestLocksAndSchedules(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 5000
	w.InvestRate = 50 // 5.0%/day, in tenths
	w.GameDay = 10

	ret, err := w.Invest(e, 1000, 3)
	if err != nil {
		t.Fatalf("Invest: %v", err)
	}
	wantRet := int64(1157) // compound daily at 5%: 1000·1.05³, int-truncated each day (1000→1050→1102→1157)
	if ret != wantRet {
		t.Errorf("Invest return: want %d, got %d", wantRet, ret)
	}
	if e.Gold != 4000 {
		t.Errorf("Gold after invest: want 4000, got %d", e.Gold)
	}
	if len(e.Investments) != 1 {
		t.Fatalf("Investments: want 1, got %d", len(e.Investments))
	}
	inv := e.Investments[0]
	if inv.Amount != 1000 {
		t.Errorf("Amount: want 1000, got %d", inv.Amount)
	}
	if inv.Return != wantRet {
		t.Errorf("Return: want %d, got %d", wantRet, inv.Return)
	}
	if inv.MaturesDay != 13 {
		t.Errorf("MaturesDay: want 13, got %d", inv.MaturesDay)
	}
}

func TestInvestUnaffordable(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 500

	_, err := w.Invest(e, 1000, 3)
	if err != ErrCantAfford {
		t.Fatalf("Invest: want ErrCantAfford, got %v", err)
	}
	if e.Gold != 500 {
		t.Errorf("Gold should be unchanged: got %d", e.Gold)
	}
	if len(e.Investments) != 0 {
		t.Errorf("Investments should be unchanged: got %d", len(e.Investments))
	}
}

func TestInvestmentMaturesInMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-01-01"
	e := w.AddHuman("tester", "Testland")
	e.LastPlayed = "2026-01-01" // has played; otherwise maintenance erases an unplayed realm
	e.Gold = 0
	e.Investments = []Investment{
		{Amount: 1000, Return: 1150, MaturesDay: 1}, // matures this maintenance
		{Amount: 500, Return: 550, MaturesDay: 99},  // far future, stays locked
	}

	w.DailyMaintenance("2026-01-02")

	if len(e.Investments) != 1 {
		t.Fatalf("Investments after maintenance: want 1 remaining, got %d", len(e.Investments))
	}
	if e.Investments[0].MaturesDay != 99 {
		t.Errorf("remaining investment: want MaturesDay 99, got %d", e.Investments[0].MaturesDay)
	}
	if e.Gold < 1150 {
		t.Errorf("Gold should include matured return of 1150: got %d", e.Gold)
	}
}

func TestAdjustInvestRateClamps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 42)
	e := w.AddHuman("tester", "Testland")

	// Drive the rate down with heavy investing.
	w.InvestRate = MinInvestRate + 1
	e.Investments = []Investment{{Amount: 10_000_000, MaturesDay: 9999}}
	for i := 0; i < 50; i++ {
		w.adjustInvestRate()
		if w.InvestRate < MinInvestRate || w.InvestRate > MaxInvestRate {
			t.Fatalf("InvestRate out of bounds: %d", w.InvestRate)
		}
	}
	// The endpoint proves the nudge actually moves: with the mechanic a no-op
	// the rate would still sit at MinInvestRate+1 and the bounds alone would
	// pass. Deterministic under seed 42.
	if w.InvestRate != MinInvestRate {
		t.Errorf("heavy investing should drive the rate to the floor %d, got %d", MinInvestRate, w.InvestRate)
	}

	// Drive the rate up with no investing.
	e.Investments = nil
	w.InvestRate = MaxInvestRate - 1
	for i := 0; i < 50; i++ {
		w.adjustInvestRate()
		if w.InvestRate < MinInvestRate || w.InvestRate > MaxInvestRate {
			t.Fatalf("InvestRate out of bounds: %d", w.InvestRate)
		}
	}
	if w.InvestRate != MaxInvestRate {
		t.Errorf("no investing should drive the rate to the ceiling %d, got %d", MaxInvestRate, w.InvestRate)
	}
}

func TestPendingInvested(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Investments = []Investment{
		{Amount: 100, MaturesDay: 5},
		{Amount: 250, MaturesDay: 10},
	}
	if got := w.PendingInvested(e); got != 350 {
		t.Errorf("PendingInvested: want 350, got %d", got)
	}
}

func TestInvestRateMigration(t *testing.T) {
	// The rate is held in tenths of a percent per day. A save from before
	// investments existed reads zero; one written while the rate was a whole
	// percent reads somewhere in the old 1..25 band, which is below the new
	// floor and so is recognisable as the older unit.
	for _, c := range []struct{ stored, want int }{
		{0, DefaultInvestRate},
		{5, 50},            // 5%/day, in band once scaled
		{1, MinInvestRate}, // 1%/day is below BRE's floor
		{12, MaxInvestRate},
		{25, MaxInvestRate}, // the old ceiling was 2.5x BRE's
		{50, 50},            // already tenths: left alone
	} {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.InvestRate = c.stored

		w.EnsureInvestRate()

		if w.InvestRate != c.want {
			t.Errorf("stored %d: want %d, got %d", c.stored, c.want, w.InvestRate)
		}
	}
}

// The rate BRE quotes is what an investment actually pays: a term at the
// default 3.5%/day compounds to a figure the whole-percent rate could not
// express at all, and the old 25%/day ceiling turned a ten-day term into a
// ninefold return.
func TestInvestReturnUsesTenthsOfAPercent(t *testing.T) {
	if got := ExpectedReturn(1000, DefaultInvestRate, 2); got != 1071 { // 1000·1.035²
		t.Errorf("1000 for 2 days at 3.5%%/day: want 1071, got %d", got)
	}
	if got := ExpectedReturn(1000, MaxInvestRate, 10); got != 2593 { // 1000·1.10¹⁰
		t.Errorf("the ceiling should pay 2593 over ten days, got %d", got)
	}
}

func TestDepositRespectsMoneyCapWithoutLosingGold(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Bank = w.MoneyCap() - 100
	e.Gold = 10_000
	before := e.Gold + e.Bank
	if err := w.Deposit(e, 5_000); err != nil {
		t.Fatal(err)
	}
	if e.Bank != w.MoneyCap() {
		t.Errorf("Bank should sit at the cap, got %d", e.Bank)
	}
	if e.Gold+e.Bank != before {
		t.Errorf("gold vanished at the cap: before=%d after=%d", before, e.Gold+e.Bank)
	}
	if e.Gold != 10_000-100 {
		t.Errorf("only the 100 that fit should leave gold, got gold=%d", e.Gold)
	}
}

// One visit moves the whole amount: neither Deposit nor Withdraw has a
// per-transaction limit of its own, so the only bound is the cap.
func TestDepositAndWithdrawMoveTheWholeAmount(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("rich", "Croesus")

	e.Gold, e.Bank = 2_000_000_000, 0
	if err := w.Deposit(e, 2_000_000_000); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if e.Bank != 2_000_000_000 || e.Gold != 0 {
		t.Errorf("one visit banked %d leaving %d in hand, want all 2,000,000,000 banked", e.Bank, e.Gold)
	}
	if err := w.Withdraw(e, 2_000_000_000); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if e.Gold != 2_000_000_000 || e.Bank != 0 {
		t.Errorf("one visit withdrew %d leaving %d banked, want all 2,000,000,000 in hand", e.Gold, e.Bank)
	}
}

// A withdrawal must not push gold past the money cap. Deposit guards its side;
// Withdraw did not, so savings drawn onto a full treasury overshot the cap until
// the end-of-turn clamp caught it — and the clamp DISCARDS the excess, so the
// gold was destroyed rather than left safely in the bank.
func TestWithdrawLeavesGoldThatWouldNotFit(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold, e.Bank = w.MoneyCap()-100, 5_000

	if err := w.Withdraw(e, 5_000); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if e.Gold != w.MoneyCap() {
		t.Errorf("gold = %d, want it filled exactly to the %d cap", e.Gold, w.MoneyCap())
	}
	if e.Bank != 4_900 {
		t.Errorf("bank = %d, want the 4,900 that would not fit left where it was safe", e.Bank)
	}
}

// One investment is capped however deep the treasury, and the rest stays in hand
// rather than vanishing. Opening more investments is not limited.
func TestInvestCapsOnePrincipal(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold = 10_000_000_000

	if _, err := w.Invest(e, 10_000_000_000, MinInvestDays); err != nil {
		t.Fatalf("Invest: %v", err)
	}
	if len(e.Investments) != 1 || e.Investments[0].Amount != MaxInvestment {
		t.Errorf("invested %+v, want a single %d principal", e.Investments, int64(MaxInvestment))
	}
	if want := int64(10_000_000_000) - MaxInvestment; e.Gold != want {
		t.Errorf("gold left = %d, want %d — the rest must stay in hand", e.Gold, want)
	}
	// A second investment is allowed; only the per-investment size is bounded.
	if _, err := w.Invest(e, 1_000_000_000, MinInvestDays); err != nil {
		t.Fatalf("second Invest: %v", err)
	}
	if len(e.Investments) != 2 {
		t.Errorf("a baron may open as many investments as they like, got %d", len(e.Investments))
	}
}

// The gold field is 64-bit, which the cap alone no longer proves: a stored 2
// billion fits an int32. What still needs the width is every intermediate the
// game computes before clamping — a projected return, a loan's total owed, gold
// plus bank — so the field is exercised past int32 directly.
func TestGoldFieldIsSixtyFourBit(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold = 5_000_000_000
	e.Gold += 3_000_000_000
	if e.Gold != 8_000_000_000 {
		t.Errorf("gold = %d, want 8,000,000,000 (an int32 field would wrap)", e.Gold)
	}
	if MoneyCapMax <= 2_147_483_647 {
		t.Errorf("MoneyCapMax = %d, which no longer gives projections room past int32", MoneyCapMax)
	}
}

// raisedCapConfig used to open the money cap all the way up for the tests that
// exercise large treasuries. The cap is fixed at 2 billion since 2026-09-01, so
// this is the default ruleset now; the helper survives to mark the call sites
// that once needed a raised one.
func raisedCapConfig() Config { return DefaultConfig() }

// The money cap is BRE's own 2 billion and is no longer a knob: World.MoneyCap
// returns it whatever config.json carries, a raised value a board saved while
// the setting existed included (#205, fixed 2026-09-01).
func TestMoneyCapIsFixed(t *testing.T) {
	for _, saved := range []int{0, MoneyCapMinBillions, 50, 5000} {
		cfg := DefaultConfig()
		cfg.MoneyCapBillions = saved
		if w := NewWorldSeed(cfg, 1); w.MoneyCap() != 2_000_000_000 {
			t.Errorf("saved %dB gave %d, want BRE's 2,000,000,000", saved, w.MoneyCap())
		}
	}

	// And it binds: a deposit stops at it rather than overflowing.
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold, e.Bank = 3_000_000_000, 0
	if err := w.Deposit(e, 3_000_000_000); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if e.Bank != 2_000_000_000 {
		t.Errorf("banked %d, want it stopped at the 2,000,000,000 cap", e.Bank)
	}
	if e.Gold != 1_000_000_000 {
		t.Errorf("gold left = %d, want the 1,000,000,000 that would not fit", e.Gold)
	}
}

// A played turn trims a treasury back to the cap whatever a board's config.json
// carries. This is the path that once threw away everything above a hard-coded
// 2 billion, and then honoured a saved raised value until the cap was fixed.
func TestTurnEndClampsToTheCap(t *testing.T) {
	for _, saved := range []int{MoneyCapMinBillions, 50} {
		cfg := DefaultConfig()
		cfg.MoneyCapBillions = saved
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("rich", "Croesus")
		e.Gold, e.Bank, e.Food = MoneyCapMax, MoneyCapMax, 1_000_000

		w.processEconomy(e)

		if e.Gold != 2_000_000_000 || e.Bank != 2_000_000_000 {
			t.Errorf("saved %dB: settled at %d in hand and %d banked, want 2,000,000,000 each",
				saved, e.Gold, e.Bank)
		}
	}
}

// Interest on a bank already at the cap is paid into the treasury instead of
// being destroyed. It used to be computed and then clamped away, so a baron at
// the ceiling earned nothing at all and never saw why.
func TestInterestOnAFullBankOverflowsIntoGold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InterestRate = 50 // the stock rate
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("rich", "Croesus")
	e.Bank, e.Gold, e.Food = w.MoneyCap(), 0, 1_000_000

	w.processEconomy(e)

	if e.Bank != w.MoneyCap() {
		t.Errorf("bank = %d, want it still exactly at the %d cap", e.Bank, w.MoneyCap())
	}
	// The whole balance earns: MoneyCap × 50 / (1000 × 10).
	want := w.MoneyCap() * 50 / (1000 * int64(cfg.TurnsPerDay))
	if e.Gold != want {
		t.Errorf("treasury received %d, want the %d of interest the full bank earned", e.Gold, want)
	}
}

// ...but the treasury has the same cap, so a baron whose hand is ALSO full does
// lose the overflow. That is the documented behaviour, not an accident: raising
// the sysop's Money Cap is the way out.
func TestInterestIsLostWhenBothPursesAreFull(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Bank, e.Gold, e.Food = w.MoneyCap(), w.MoneyCap(), 1_000_000

	w.processEconomy(e)

	if e.Bank != w.MoneyCap() || e.Gold != w.MoneyCap() {
		t.Errorf("both purses should sit at the %d cap, got gold=%d bank=%d", w.MoneyCap(), e.Gold, e.Bank)
	}
}

// Gold destroyed at the cap tells the owner so. The number used to just stop
// growing with nothing on screen to explain it, which is what made the old
// hard-coded ceiling read as a bug rather than a rule.
func TestGoldLostToTheCapRaisesAnEvent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold = w.MoneyCap() - 1_000
	before := len(e.Events)

	w.creditGold(e, 5_000, "a matured investment")

	if e.Gold != w.MoneyCap() {
		t.Errorf("gold = %d, want it held at the %d cap", e.Gold, w.MoneyCap())
	}
	if len(e.Events) != before+1 {
		t.Fatalf("got %d new events, want exactly one", len(e.Events)-before)
	}
	msg := e.Events[len(e.Events)-1].Text
	for _, want := range []string{"4,000", "a matured investment", "2,000,000,000"} {
		if !strings.Contains(msg, want) {
			t.Errorf("event %q should name %q — the amount lost and where it came from", msg, want)
		}
	}
}

// A credit that fits raises nothing: the event marks a real loss, not every
// payment into a large treasury.
func TestCreditThatFitsIsSilent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold = 1_000
	before := len(e.Events)

	w.creditGold(e, 5_000, "a matured investment")

	if e.Gold != 6_000 {
		t.Errorf("gold = %d, want 6,000", e.Gold)
	}
	if len(e.Events) != before {
		t.Errorf("a credit that fits raised %d events, want none", len(e.Events)-before)
	}
}

// The end-of-turn deposit is banked BEFORE the interest is credited, so gold
// earned this turn earns on this turn (#216). It was banked in the menu flow
// after PlayTurn until 2026-08-25, which meant every deposit missed the turn
// that made it.
func TestEndOfTurnDepositEarnsInterestSameTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Food = 1_000_000 // starvation is not the subject
	e.Bank, e.Gold = 0, 1_000_000

	w.processEconomy(e)

	interest := int64(1_000_000) * int64(w.Config.InterestRate) / (1000 * int64(w.Config.TurnsPerDay))
	if interest == 0 {
		t.Fatal("the default config pays no interest; this test proves nothing")
	}
	if e.Gold != 0 {
		t.Errorf("Gold = %d, want 0 — the whole hand is deposited at end of turn", e.Gold)
	}
	if want := 1_000_000 + interest; e.Bank != want {
		t.Errorf("Bank = %d, want %d — the deposit must earn on the turn it was made", e.Bank, want)
	}
	if e.LastInterest != interest {
		t.Errorf("LastInterest = %d, want %d — the start of the next turn reports this", e.LastInterest, interest)
	}
}

// With the preference off the gold stays in hand and earns nothing.
func TestNoEndOfTurnDepositLeavesGoldInHand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Food = 1_000_000
	e.Bank, e.Gold = 0, 1_000_000
	e.Prefs.DepositEndTurn = false

	w.processEconomy(e)

	if e.Gold != 1_000_000 {
		t.Errorf("Gold = %d, want the 1,000,000 left in hand", e.Gold)
	}
	if e.Bank != 0 || e.LastInterest != 0 {
		t.Errorf("Bank/LastInterest = %d/%d, want 0/0 — an empty bank earns nothing", e.Bank, e.LastInterest)
	}
}

// Daily maintenance records what the day's matured investments paid, because
// every turn of that day reports the same figure (cap/eots-ibbs-01.cap).
func TestMaintenanceRecordsTheDaysInvestmentReturns(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-01-01"
	e := w.AddHuman("tester", "Testland")
	e.LastPlayed = "2026-01-01"
	e.Gold = 0
	e.Investments = []Investment{
		{Amount: 1000, Return: 1150, MaturesDay: 1},
		{Amount: 2000, Return: 2300, MaturesDay: 1},
		{Amount: 500, Return: 550, MaturesDay: 99},
	}

	w.DailyMaintenance("2026-01-02")

	if e.InvestReturnsToday != 3450 {
		t.Errorf("InvestReturnsToday = %d, want 3450 — both matured returns, and not the locked one", e.InvestReturnsToday)
	}
}
