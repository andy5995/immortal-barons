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
	// The funding that buys the cap over the land this realm holds, and then some.
	a.SDIFunding = int64(SDIMax*SDIMax) * SDIStrengthLandDivisor * int64(a.Land+1)
	a.syncSDI()

	level, err := w.FundSDI(a, SDIMinSpend)
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

// The funding-to-strength curve against the same captured game, as golden
// literals. Every screen in that capture was read at 8,321 regions — the
// Terrorist Ops price on the surrounding menu is regions x 64, which pins the
// figure — and all sixteen land exactly.
func TestSDIStrengthMatchesCapturedScreens(t *testing.T) {
	const land = 8_321
	for _, c := range []struct {
		funding int64
		want    int
	}{
		{0, 0},
		{250_000, 1},
		{500_000, 2},
		{750_000, 3},
		{1_000_000, 3},
		{1_250_000, 3},
		{1_500_000, 4},
		{1_800_000, 4},
		{2_160_000, 5},
		{2_592_000, 5},
		{3_110_000, 6},
		{3_732_000, 6},
		{4_478_000, 7},
		{5_373_000, 8},
		{5_899_000, 8},
		{7_078_000, 9},
	} {
		if got := SDIStrength(c.funding, land); got != c.want {
			t.Errorf("%d gold over %d regions = %d%%, want %d%%", c.funding, land, got, c.want)
		}
	}
}

// Land divides the shield, so the same program covers a bigger realm worse.
func TestSDIStrengthThinsOverMoreLand(t *testing.T) {
	if got := SDIStrength(7_078_000, 100); got != 83 {
		t.Errorf("7,078,000 over 101 regions = %d%%, want 83%%", got)
	}
	if got := SDIStrength(7_078_000, 8_321); got != 9 {
		t.Errorf("7,078,000 over 8,322 regions = %d%%, want 9%%", got)
	}
}

// An arriving individual strike loses 30% of its jets' contribution and 20% of
// its bombers' to a full shield, and keeps everything else.
func TestSDIBluntsArrivingJetsAndBombers(t *testing.T) {
	f := AttackForce{Troopers: 1_000, Jets: 1_000, Tanks: 1_000, Bombers: 1_000}
	whole := f.offense()
	jetLoss := 1_000 * 2 * 30 / 100
	bomberLoss := 1_000 * GroupAttackBomberOffense * 20 / 100
	if got, want := f.offenseAgainstSDI(SDIMax), whole-jetLoss-bomberLoss; got != want {
		t.Errorf("offense against a full shield = %d, want %d", got, want)
	}
	if got := f.offenseAgainstSDI(0); got != whole {
		t.Errorf("no shield changed the offense: %d, want %d", got, whole)
	}
}

// A group attack faces no shield at all: the original averages the planet's
// shields and then divides by 100 a second time, which can never reach 1.
func TestGroupAttackIgnoresTheDefendersSDI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	w := NewWorldSeed(cfg, 1)
	d := w.AddHuman("def", "Carthage")
	d.SDI = SDIMax
	f := AttackForce{Jets: 1_000, Bombers: 1_000}
	contributors := []Contribution{{Owner: "att", AttackForce: f}}
	group := RemoteAttack{Offense: f.offense(), Group: true, Contributors: contributors}
	if got := group.offenseAgainstSDI(d); got != f.offense() {
		t.Errorf("a group attack was blunted to %d, want its whole %d", got, f.offense())
	}
	solo := RemoteAttack{Offense: f.offense(), Kind: NormalAttack, Contributors: contributors}
	if got := solo.offenseAgainstSDI(d); got >= f.offense() {
		t.Errorf("an individual strike was not blunted: %d, want less than %d", got, f.offense())
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
