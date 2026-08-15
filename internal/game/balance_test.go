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

// TestRiverBadYearDud exercises both River branches across game-days: the dud
// (RiverBase/2, no swing) and the normal yield (>= RiverBase).
func TestRiverBadYearDud(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	// riverGold is net of the food share (RiverFishShare), so both branches are
	// scaled the same way.
	share := func(n int) int { return n * (100 - RiverFishShare) / 100 }
	sawDud, sawNormal := false, false
	for day := 0; day < 200 && !(sawDud && sawNormal); day++ {
		w.GameDay = day
		switch g := w.riverGold(e); {
		case g == share(RiverBase/2):
			sawDud = true
		case g >= share(RiverBase):
			sawNormal = true
		default:
			t.Fatalf("day %d: riverGold=%d outside expected values", day, g)
		}
	}
	if !sawDud {
		t.Error("expected at least one River bad-year dud across 200 days")
	}
	if !sawNormal {
		t.Error("expected at least one normal River year across 200 days")
	}
}

// TestInterestNoInt32Overflow guards turn.go's interest step against 32-bit int
// overflow: at a huge Bank and a high InterestRate the int64 arithmetic keeps
// the result positive and growing. A wrapped int32 would go negative and the
// balance would shrink instead of earning.
func TestInterestNoInt32Overflow(t *testing.T) {
	// A raised cap, so the earned figure is the one under test rather than the
	// stock 2-billion ceiling clamping it on the way past.
	w := NewWorldSeed(raisedCapConfig(), 1)
	w.Config.InterestRate = 5000 // add ~50% of the balance this turn
	e := w.AddHuman("h", "Realm")
	e.Bank = 1_599_999_999
	e.Food = 1_000_000 // avoid starvation noise; irrelevant to the bank math
	w.processEconomy(e)
	// Golden figure for that balance at rate 5000 over the default 10 turns/day:
	// 1,599,999,999 + 1,599,999,999×5000/(1000×10).
	const want int64 = 2_399_999_998
	if e.Bank != want {
		t.Errorf("Bank = %d, want %d (int32 overflow would make it negative/small)", e.Bank, want)
	}
}

// A balance far past the old 2-billion ceiling must survive the turn intact:
// that ceiling was a 32-bit int limit, and gold above it used to vanish.
func TestBankHoldsPastTwoBillion(t *testing.T) {
	w := NewWorldSeed(raisedCapConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Bank = 500_000_000_000
	e.Food = 1_000_000
	w.processEconomy(e)
	if e.Bank < 500_000_000_000 {
		t.Errorf("Bank = %d, want at least the 500,000,000,000 it started with", e.Bank)
	}
}
