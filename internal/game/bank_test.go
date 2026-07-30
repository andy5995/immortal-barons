package game

import "testing"

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
	wantRet := 1157 // compound daily at 5%: 1000·1.05³, int-truncated each day (1000→1050→1102→1157)
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

	// Drive the rate up with no investing.
	e.Investments = nil
	w.InvestRate = MaxInvestRate - 1
	for i := 0; i < 50; i++ {
		w.adjustInvestRate()
		if w.InvestRate < MinInvestRate || w.InvestRate > MaxInvestRate {
			t.Fatalf("InvestRate out of bounds: %d", w.InvestRate)
		}
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
	e.Bank = MoneyCap - 100
	e.Gold = 10_000
	before := e.Gold + e.Bank
	if err := w.Deposit(e, 5_000); err != nil {
		t.Fatal(err)
	}
	if e.Bank != MoneyCap {
		t.Errorf("Bank should sit at the cap, got %d", e.Bank)
	}
	if e.Gold+e.Bank != before {
		t.Errorf("gold vanished at the cap: before=%d after=%d", before, e.Gold+e.Bank)
	}
	if e.Gold != 10_000-100 {
		t.Errorf("only the 100 that fit should leave gold, got gold=%d", e.Gold)
	}
}
