package game

import "testing"

func TestBuyLandCostsGold(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	gold, land := p.Gold, p.Land
	if err := w.BuyLand(p, 10); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if p.Land != land+10 {
		t.Errorf("land: want %d, got %d", land+10, p.Land)
	}
	if p.Gold != gold-10*w.Prices.Land {
		t.Errorf("gold: want %d, got %d", gold-10*w.Prices.Land, p.Gold)
	}
}

func TestBuyLandRejectsWhenBroke(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	p.Gold = 50
	if err := w.BuyLand(p, 100); err != ErrCantAfford {
		t.Errorf("want ErrCantAfford, got %v", err)
	}
	if p.Land != 100 {
		t.Errorf("land should be unchanged, got %d", p.Land)
	}
}

func TestDepositWithdrawRoundTrip(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	p.Gold, p.Bank = 1000, 0
	w.Deposit(p, 600)
	if p.Gold != 400 || p.Bank != 600 {
		t.Fatalf("after deposit gold=%d bank=%d", p.Gold, p.Bank)
	}
	if err := w.Withdraw(p, 1000); err != ErrNoBank {
		t.Errorf("over-withdraw should fail, got %v", err)
	}
	w.Withdraw(p, 600)
	if p.Gold != 1000 || p.Bank != 0 {
		t.Errorf("after withdraw gold=%d bank=%d", p.Gold, p.Bank)
	}
}

func TestEndTurnGivesIncomeAndAdvances(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	gold := p.Gold
	w.EndTurn()
	if w.Turn != 1 {
		t.Errorf("turn: want 1, got %d", w.Turn)
	}
	if p.Gold <= gold {
		t.Errorf("expected income to raise gold above %d, got %d", gold, p.Gold)
	}
}

func TestAttackChangesLand(t *testing.T) {
	w := NewSeed(1)
	a := w.Player()
	d := w.Rivals()[0]
	// Give the attacker an overwhelming force so the outcome is deterministic.
	a.Troopers = 100000
	totalLand := a.Land + d.Land
	w.Attack(a, d)
	if a.Land+d.Land != totalLand {
		t.Errorf("captured land should conserve total: was %d, now %d", totalLand, a.Land+d.Land)
	}
	if a.Land <= 100 {
		t.Errorf("overwhelming attacker should have gained land, got %d", a.Land)
	}
}

func TestOffenseDefenseValues(t *testing.T) {
	e := &Empire{Troopers: 10, Jets: 10, Turrets: 10, Tanks: 10, Carriers: 1}
	// offense: 10 + min(10,100)*2 + 10*4 = 70; defense: 10 + 10*2 + 10*4 = 70
	if o := e.Offense(); o != 70 {
		t.Errorf("offense: want 70, got %d", o)
	}
	if d := e.Defense(); d != 70 {
		t.Errorf("defense: want 70, got %d", d)
	}
}

func TestJetsNeedCarriers(t *testing.T) {
	e := &Empire{Jets: 500, Carriers: 1} // 1 carrier moves 100 jets
	if o := e.Offense(); o != 200 {
		t.Errorf("uncarried jets should not count: want 200, got %d", o)
	}
	e.Carriers = 5
	if o := e.Offense(); o != 1000 {
		t.Errorf("with carriers for all jets: want 1000, got %d", o)
	}
}

func TestBankInterestIsOnePercent(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	p.Bank = 100000
	w.EndTurn() // bank changes only through interest
	if p.Bank != 101000 {
		t.Errorf("1%% interest: want 101000, got %d", p.Bank)
	}
}

func TestInterestStopsAtCap(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	p.Bank = InterestCap + 100_000_000 // above the interest cap, below money cap
	w.EndTurn()
	want := InterestCap + 100_000_000 + InterestCap/100
	if p.Bank != want {
		t.Errorf("interest should be capped: want %d, got %d", want, p.Bank)
	}
}

func TestMoneyCapClamps(t *testing.T) {
	w := NewSeed(1)
	p := w.Player()
	p.Gold = MoneyCap + 1_000_000
	w.EndTurn()
	if p.Gold != MoneyCap {
		t.Errorf("gold should clamp to money cap: want %d, got %d", MoneyCap, p.Gold)
	}
}

func TestGameOverAtMaxTurns(t *testing.T) {
	w := NewSeed(1)
	w.Turn = w.MaxTurns
	if !w.GameOver() {
		t.Error("game should be over at MaxTurns")
	}
}
