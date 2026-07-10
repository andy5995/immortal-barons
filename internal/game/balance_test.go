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

	wantIndustrial := w.IncomeThisTurn(e).Industrial
	w.CollectIncome(e)
	if e.Gold != wantIndustrial {
		t.Errorf("CollectIncome credited %d, want the single Industrial figure %d", e.Gold, wantIndustrial)
	}
	if want := w.industrialGold(e) * 20; wantIndustrial != want {
		t.Errorf("Industrial income = %d, want industrialGold*count = %d", wantIndustrial, want)
	}
}

// TestRiverBadYearDud exercises both River branches across game-days: the dud
// (RiverBase/2, no swing) and the normal yield (>= RiverBase).
func TestRiverBadYearDud(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	sawDud, sawNormal := false, false
	for day := 0; day < 200 && !(sawDud && sawNormal); day++ {
		w.GameDay = day
		switch g := w.riverGold(e); {
		case g == RiverBase/2:
			sawDud = true
		case g >= RiverBase:
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
// overflow: at a near-cap Bank and a high InterestRate the int64 intermediate
// keeps the result positive and large (a wrapped int32 would go negative and
// the Bank would shrink instead of hitting the cap).
func TestInterestNoInt32Overflow(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.InterestRate = 5000 // add ~100% of min(Bank,InterestCap) this turn
	e := w.AddHuman("h", "Realm")
	e.Bank = InterestCap
	e.Food = 1_000_000 // avoid starvation noise; irrelevant to the bank math
	w.processEconomy(e)
	if e.Bank != MoneyCap {
		t.Errorf("Bank = %d, want %d (int32 overflow would make it negative/small)", e.Bank, MoneyCap)
	}
}
