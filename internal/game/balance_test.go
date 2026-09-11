package game

import "testing"

// TestTaxIncomeExact pins the tax coefficient: with tf=0 (no Technology),
// Taxes = People * Tax/100 * TaxGoldPerCapita.
func TestTaxIncomeExact(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Regions = RegionMix{}
	e.Land = 0 // tf = 0
	e.People = 2000
	e.Tax = 7
	want := 2000 * 7 / 100 * TaxGoldPerCapita
	if got := w.IncomeThisTurn(e).Taxes; got != want {
		t.Errorf("Taxes = %d, want %d", got, want)
	}
}

// TestCoastalSupportFloor checks the 0.10 support floor: even at 0% support a
// Coastal region still yields gold (BRE's supportFactor = 0.10 + 0.90·support).
func TestCoastalSupportFloor(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Regions = RegionMix{Coastal: 10}
	e.syncLand()
	e.Support = 0
	e.People, e.Tax = 0, 0
	if got := w.IncomeThisTurn(e).Tourism; got <= 0 {
		t.Errorf("Coastal tourism at 0%% support should be > 0 (10%% floor), got %d", got)
	}
}

// TestUrbanTechnologyProduceNoGold: Urban and Technology regions give no direct
// gold (BRE-verified), so an empire of only those earns nothing but taxes.
func TestUrbanTechnologyProduceNoGold(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Regions = RegionMix{Urban: 50, Technology: 50}
	e.syncLand()
	e.People, e.Tax = 0, 0
	if got := w.IncomeThisTurn(e).Gold(); got != 0 {
		t.Errorf("Urban+Technology-only empire should earn 0 gold, got %d", got)
	}
}

// TestIndustrialGoldCreditedOnce: industrial gold flows only through
// CollectIncome; Manufacture credits no gold (the old double-count is gone).
func TestIndustrialGoldCreditedOnce(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Regions = RegionMix{Industrial: 20}
	e.syncLand()
	e.People, e.Tax = 0, 0
	e.Gold = 0

	w.Manufacture(e)
	if e.Gold != 0 {
		t.Fatalf("Manufacture credited gold (%d); it should produce units only", e.Gold)
	}

	wantIndustrial := int64(w.IncomeThisTurn(e).Industrial)
	w.CollectIncome(e)
	if e.Gold != wantIndustrial {
		t.Errorf("CollectIncome credited %d, want the single Industrial figure %d", e.Gold, wantIndustrial)
	}
	if want := int64(w.industrialGold(e)); wantIndustrial != want {
		t.Errorf("Industrial income = %d, want the industrialGold total %d", wantIndustrial, want)
	}
}

// TestRiverGoldHasNoBadYear holds the river to BRE's band. The original rolls
// only Random(4) for fishing and has no halving branch, so every hydropower turn
// pays Base + [0, Rate) less the food share. Golden literals: 5,000..5,099 at
// 75% is 3,750..3,824.
func TestRiverGoldHasNoBadYear(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	for day := 0; day < 200; day++ {
		w.GameDay = day
		if g := w.riverGold(e); g < 3750 || g > 3824 {
			t.Fatalf("day %d: riverGold=%d, want 3750..3824", day, g)
		}
	}
}

// TestInterestNoInt32Overflow guards turn.go's interest step against 32-bit int
// overflow: at a huge Bank and a high InterestRate the int64 arithmetic keeps
// the result positive and growing. A wrapped int32 would go negative and the
// balance would shrink instead of earning.
func TestInterestNoInt32Overflow(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	w.Config.InterestRate = 5000 // add ~50% of the balance this turn
	e := w.AddHuman("h", "Realm")
	e.Bank = 1_599_999_999
	e.Food = 1_000_000             // avoid starvation noise; irrelevant to the bank math
	e.Prefs.DepositEndTurn = false // the bank math is the subject; keep the turn's gold out of it
	w.processEconomy(e)
	// The balance earns its way to 2,399,999,998 (1,599,999,999 +
	// 1,599,999,999×5000/(1000×10)) and the cap then trims it to 2 billion. The
	// interesting part is the intermediate: it is past int32's 2,147,483,647, so
	// a 32-bit sum would land negative or small and could never come to rest ON
	// the cap. Seeing exactly the cap is what proves the arithmetic was 64-bit.
	if e.Bank != w.MoneyCap() {
		t.Errorf("Bank = %d, want the %d cap (an int32 sum would be negative or small)",
			e.Bank, w.MoneyCap())
	}
}

// A balance far past int32 must be TRIMMED to the cap by the turn, not corrupted
// by it. The old 2-billion ceiling was a 32-bit limit and gold above it used to
// vanish; the ceiling is a game rule now (#205) and the arithmetic that enforces
// it still has to be 64-bit.
func TestBankPastInt32TrimsToTheCap(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Bank = 500_000_000_000
	e.Food = 1_000_000
	w.processEconomy(e)
	if e.Bank != w.MoneyCap() {
		t.Errorf("Bank = %d, want the %d cap", e.Bank, w.MoneyCap())
	}
}
