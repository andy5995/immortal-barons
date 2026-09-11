package game

import (
	"math/big"
	"testing"
)

// Production used to be computed as one int64 product of six factors, two of
// which — the mountain boost's numerator and denominator — are raw land counts.
// The product therefore grew with the SQUARE of the realm's size and passed
// int64 at around 6,000 industrial regions with research behind it, handing back
// counts that were wrong and often NEGATIVE, which were then added to the army.
//
// No realm, at any size, produces a negative or absurd figure.
func TestProductionNeverGoesNegative(t *testing.T) {
	for _, ind := range []int{1, 1_000, 14_400, 14_500, 100_000, 1_000_000, 50_000_000} {
		for _, mountain := range []int{0, ind / 3, ind} {
			w := NewWorldSeed(DefaultConfig(), 1)
			e := w.AddHuman("a", "A")
			e.Regions = RegionMix{Industrial: ind, Mountain: mountain}
			e.Specialized = Jet.Plural
			for i, n := range w.ProjectedProduction(e) {
				if n < 0 {
					t.Errorf("%d industrial, %d mountain: %s production is %d",
						ind, mountain, MilitaryGoods[i].Plural, n)
				}
			}
		}
	}
}

// Production scales with the realm, which a wrapping product did not: twice the
// industry is twice the units.
func TestProductionScalesPastTheOldCeiling(t *testing.T) {
	made := func(ind int) int {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("a", "A")
		e.Regions = RegionMix{Industrial: ind}
		return w.ProjectedProduction(e)[0]
	}
	small, big := made(50_000), made(100_000)
	if big != 2*small {
		t.Errorf("100,000 industrial made %d, want twice the 50,000 figure (%d)", big, 2*small)
	}
}

// The exact arithmetic reproduces the old int64 formula wherever that formula
// did not overflow — the rounding is binary-verified and must not have moved.
func TestExactProductionMatchesTheOldFormula(t *testing.T) {
	old := func(industrial, pct, spec, boostNum, boostDen, tech, cost int) (int, bool) {
		n := new(big.Int).SetInt64(int64(industrial))
		for _, f := range []int64{UnitPointsPerRegion, int64(pct), int64(spec), int64(boostNum), int64(tech)} {
			n.Mul(n, big.NewInt(f))
		}
		d := int64(cost) * 100 * 100 * int64(boostDen) * TechFactorUnit
		// Only meaningful where the old product AND its rounding term fit.
		if !n.IsInt64() || n.Int64() > (1<<62) {
			return 0, false
		}
		return int((n.Int64() + d/2) / d), true
	}
	for ind := 1; ind <= 6000; ind += 13 {
		for _, m := range []int{0, 1, 500, 3000} {
			bn, bd := industryMountainBoost(RegionMix{Industrial: ind, Mountain: m})
			for _, pct := range []int{1, 17, 100} {
				for _, spec := range []int{100 - SpecialtyPenaltyPct, 100, 100 + SpecialtyBonusPct} {
					for _, tech := range []int{TechFactorUnit, 14000} {
						for _, g := range MilitaryGoods {
							want, ok := old(ind, pct, spec, bn, bd, tech, g.Cost)
							if !ok {
								continue
							}
							if got := unitsMade(ind, pct, spec, bn, bd, tech, g.Cost); got != want {
								t.Fatalf("ind=%d m=%d pct=%d spec=%d tech=%d cost=%d: got %d, want %d",
									ind, m, pct, spec, tech, g.Cost, got, want)
							}
						}
					}
				}
			}
		}
	}
}
