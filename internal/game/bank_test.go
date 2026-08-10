package game

import (
	"strings"
	"testing"
)

func TestInvestLocksAndSchedules(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 5000
	w.InvestRate = 5
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
	w := NewWorldSeed(DefaultConfig(), 1)
	w.InvestRate = 0 // simulate a save written before InvestRate existed

	w.EnsureInvestRate()

	if w.InvestRate != DefaultInvestRate {
		t.Errorf("InvestRate after migration: want %d, got %d", DefaultInvestRate, w.InvestRate)
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

// Deposits and withdrawals are unbounded up to the money cap: nothing gates the
// bank per turn, so a per-action limit there only cost keystrokes. Investing is
// the case that stays capped.
func TestDepositAndWithdrawAreUnbounded(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("rich", "Croesus")

	e.Gold, e.Bank = 10_000_000_000, 0
	if err := w.Deposit(e, 10_000_000_000); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if e.Bank != 10_000_000_000 || e.Gold != 0 {
		t.Errorf("one visit banked %d leaving %d in hand, want all 10,000,000,000 banked", e.Bank, e.Gold)
	}
	if err := w.Withdraw(e, 10_000_000_000); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if e.Gold != 10_000_000_000 || e.Bank != 0 {
		t.Errorf("one visit withdrew %d leaving %d banked, want all 10,000,000,000 in hand", e.Gold, e.Bank)
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

// Gold past the old 2-billion ceiling must survive a credit rather than being
// clamped away: that ceiling was a 32-bit int limit, not a game rule.
func TestGoldHoldsPastTwoBillion(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("rich", "Croesus")
	e.Gold = 5_000_000_000
	e.Gold += 3_000_000_000
	if e.Gold != 8_000_000_000 {
		t.Errorf("gold = %d, want 8,000,000,000", e.Gold)
	}
	if e.Gold > w.MoneyCap() {
		t.Errorf("gold %d should be well under the %d cap", e.Gold, w.MoneyCap())
	}
}

// raisedCapConfig is the default ruleset with the money cap opened all the way
// up, for the tests that exercise treasuries past the stock 2-billion ceiling.
func raisedCapConfig() Config {
	c := DefaultConfig()
	c.MoneyCapBillions = MoneyCapMaxBillions
	return c
}

// The money cap is the sysop's knob. It defaults to BRE's own 2 billion, refuses
// to go below it (an old config.json has no such field at all, and a zero there
// must not mean "no money allowed"), and stops at the widest figure the
// abbreviated display renders.
func TestMoneyCapKnob(t *testing.T) {
	if w := NewWorldSeed(DefaultConfig(), 1); w.MoneyCap() != 2_000_000_000 {
		t.Errorf("stock cap = %d, want BRE's 2,000,000,000", w.MoneyCap())
	}

	cfg := DefaultConfig()
	cfg.MoneyCapBillions = 0 // a save written before the knob existed
	if w := NewWorldSeed(cfg, 1); w.MoneyCap() != 2_000_000_000 {
		t.Errorf("missing field gave %d, want the 2,000,000,000 default", w.MoneyCap())
	}

	cfg.MoneyCapBillions = 5000 // past the ceiling
	if w := NewWorldSeed(cfg, 1); w.MoneyCap() != MoneyCapMax {
		t.Errorf("over-max gave %d, want %d", w.MoneyCap(), MoneyCapMax)
	}

	cfg.MoneyCapBillions = 50
	w := NewWorldSeed(cfg, 1)
	if w.MoneyCap() != 50_000_000_000 {
		t.Errorf("cap = %d, want 50,000,000,000", w.MoneyCap())
	}
	// And the knob actually binds: a deposit stops at it rather than overflowing.
	e := w.AddHuman("rich", "Croesus")
	e.Gold, e.Bank = 60_000_000_000, 0
	if err := w.Deposit(e, 60_000_000_000); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if e.Bank != 50_000_000_000 {
		t.Errorf("banked %d, want it stopped at the 50,000,000,000 cap", e.Bank)
	}
	if e.Gold != 10_000_000_000 {
		t.Errorf("gold left = %d, want the 10,000,000,000 that would not fit", e.Gold)
	}
}

// A played turn trims a treasury back to the configured cap, so raising the knob
// genuinely lets a realm keep more and lowering it genuinely binds. This is the
// path that used to throw away everything above a hard-coded 2 billion.
func TestTurnEndClampsToConfiguredCap(t *testing.T) {
	for _, c := range []struct {
		billions int
		want     int64
	}{
		{MoneyCapMinBillions, 2_000_000_000},
		{50, 50_000_000_000},
		{MoneyCapMaxBillions, MoneyCapMax},
	} {
		cfg := DefaultConfig()
		cfg.MoneyCapBillions = c.billions
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("rich", "Croesus")
		e.Gold, e.Bank, e.Food = MoneyCapMax, MoneyCapMax, 1_000_000

		w.processEconomy(e)

		if e.Gold != c.want {
			t.Errorf("cap %dB: gold settled at %d, want %d", c.billions, e.Gold, c.want)
		}
		if e.Bank != c.want {
			t.Errorf("cap %dB: bank settled at %d, want %d", c.billions, e.Bank, c.want)
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
	// Interest is charged on the capped slice: InterestCap × 50 / (1000 × 10).
	want := InterestCap * 50 / (1000 * int64(cfg.TurnsPerDay))
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
	for _, want := range []string{"4,000", "a matured investment", "2.0000B"} {
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
