package game

import "testing"

// The program takes gold a turn at a time, in whole thousands, and no more than
// the turn's allowance permits.
func TestFundSDIRespectsTheTurnAllowance(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100_000_000

	if _, err := w.FundSDI(a, 400_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A fresh program's allowance is the floor, so only that much goes in.
	if a.SDIFunding != SDIMinSpend {
		t.Errorf("funding = %d, want the allowance %d", a.SDIFunding, SDIMinSpend)
	}
	if want := int64(100_000_000 - SDIMinSpend); a.Gold != want {
		t.Errorf("gold = %d, want %d", a.Gold, want)
	}
	// The allowance is spent for the turn.
	if got := w.SDISpendAllowance(a); got != 0 {
		t.Errorf("allowance left = %d, want 0", got)
	}
	if _, err := w.FundSDI(a, 250_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.SDIFunding != SDIMinSpend {
		t.Errorf("a second payment got past the allowance: funding = %d", a.SDIFunding)
	}
}

// Odd gold is rounded down to a whole thousand, as the screen's note says.
func TestFundSDIRoundsToWholeThousands(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100_000_000

	if _, err := w.FundSDI(a, 1_750); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.SDIFunding != 1_000 {
		t.Errorf("funding = %d, want 1000", a.SDIFunding)
	}
}

// Strength stops at the cap however much gold the program holds.
func TestFundSDICapsAtMax(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 1_000_000_000
	a.SDIFunding = SDIMax * SDIStep
	a.syncSDI()

	level, err := w.FundSDI(a, SDIStep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != SDIMax {
		t.Errorf("expected SDI capped at %d, got %d", SDIMax, level)
	}
}

func TestFundSDICantAfford(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100

	_, err := w.FundSDI(a, SDIMinSpend)
	if err != ErrCantAfford {
		t.Fatalf("expected ErrCantAfford, got %v", err)
	}
	if a.Gold != 100 {
		t.Errorf("gold should be unchanged, got %d", a.Gold)
	}
}

// The upkeep and the allowance against the figures BRE printed, as golden
// literals rather than as the constants — the point of the fidelity contract is
// that a retune has to answer to new evidence (docs/dev/bre-screens.md).
func TestSDIUpkeepAndAllowanceMatchBRE(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	for _, c := range []struct{ funding, maint, allowance int64 }{
		{0, 0, 250_000},
		{250_000, 10_000, 250_000},
		{1_250_000, 50_000, 250_000},
		{1_500_000, 60_000, 300_000},
		{2_592_000, 103_680, 518_400},
		{7_078_000, 283_120, 1_415_600},
	} {
		a.SDIFunding = c.funding
		a.TurnProgress.SDIFunded = 0
		if got := w.SDIMaintenance(a); got != c.maint {
			t.Errorf("upkeep on %d = %d, want %d", c.funding, got, c.maint)
		}
		if got := w.SDISpendAllowance(a); got != c.allowance {
			t.Errorf("allowance on %d = %d, want %d", c.funding, got, c.allowance)
		}
	}
}

// Paying the upkeep short scales the program back to what was funded.
func TestPaySDIShortfallShrinksTheProgram(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.SDIFunding = 1_000_000
	a.syncSDI()
	a.Gold = 100_000

	w.PaySDI(a, 20_000) // half of the 40,000 due
	if a.SDIFunding != 500_000 {
		t.Errorf("funding after paying half = %d, want 500000", a.SDIFunding)
	}
	if a.Gold != 80_000 {
		t.Errorf("gold = %d, want 80000", a.Gold)
	}
}

func TestNuclearStrikeIgnoresSDI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1

	// The defenders need REAL regions, not a bare Land figure: the strike ends
	// in syncLand, which recomputes Land from the mix, so a fixture that only
	// sets Land measures the fixture's lie.
	w1 := NewWorldSeed(cfg, 42)
	a1 := w1.AddHuman("att", "Attacker")
	a1.Gold = 100_000_000
	d1 := w1.Empires[0]
	d1.Protection, a1.Protection = 0, 0
	d1.Regions = RegionMix{Agricultural: 1000}
	d1.syncLand()
	d1.SDI = 0

	w2 := NewWorldSeed(cfg, 42)
	a2 := w2.AddHuman("att", "Attacker")
	a2.Gold = 100_000_000
	d2 := w2.Empires[0]
	d2.Protection, a2.Protection = 0, 0
	d2.Regions = RegionMix{Agricultural: 1000}
	d2.syncLand()
	d2.SDI = SDIMax

	if _, err := w1.NuclearStrike(a1, d1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w2.NuclearStrike(a2, d2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A local nuclear strike is not intercepted — the original's routine never
	// reads the target's SDI. Both worlds share seed 42 and an identical call
	// sequence, so a mitigation reintroduced here would split these two figures.
	if d1.Regions.Waste != d2.Regions.Waste {
		t.Errorf("SDI changed nuclear damage: no-SDI ruined %d, max-SDI ruined %d",
			d1.Regions.Waste, d2.Regions.Waste)
	}
	if d1.Regions.Waste <= 0 {
		t.Errorf("expected the strike to ruin regions, ruined %d", d1.Regions.Waste)
	}
	// Ruined, not removed: the land stays on the books and keeps costing upkeep.
	if d1.Land != 1000 {
		t.Errorf("land should be unchanged at 1000, got %d", d1.Land)
	}
}

// A save written before the program kept a funding pool keeps the SDI it paid
// for: the backfill runs on load, so the first recompute does not wipe it.
func TestEnsureSDIFundingKeepsALegacyLevel(t *testing.T) {
	e := &Empire{SDI: 4}
	e.EnsureSDIFunding()
	e.syncSDI()
	if e.SDI != 4 {
		t.Errorf("a legacy SDI of 4 became %d", e.SDI)
	}
}

// One allowance per turn, not per visit: re-entering the SDI screen keeps
// drawing on the same allowance until the turn ends, so a baron cannot walk in
// and out of it funding the shield as much as their gold allows.
func TestSDIAllowanceIsPerTurnNotPerVisit(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100_000_000
	a.TurnsLeft = 5

	for visit := range 4 {
		if _, err := w.FundSDI(a, SDIMinSpend); err != nil {
			t.Fatalf("visit %d: %v", visit, err)
		}
	}
	if a.SDIFunding != SDIMinSpend {
		t.Errorf("four visits in one turn funded %d, want one allowance of %d", a.SDIFunding, SDIMinSpend)
	}

	w.PlayTurn(a, "2026-08-08")
	if got := w.SDISpendAllowance(a); got != SDIMinSpend {
		t.Errorf("allowance after the turn ended = %d, want it refilled to %d", got, SDIMinSpend)
	}
	if _, err := w.FundSDI(a, SDIMinSpend); err != nil {
		t.Fatal(err)
	}
	if a.SDIFunding != 2*SDIMinSpend {
		t.Errorf("funding after a second turn = %d, want %d", a.SDIFunding, 2*SDIMinSpend)
	}
}
