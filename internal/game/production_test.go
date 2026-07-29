package game

import "testing"

// TestProjectedProductionMatchesBRE pins unit manufacturing to figures captured
// from the original game. The formula was read out of BRE.OVR (0x34F49-0x3517A)
// and these are the live turns it was validated against: an empire specialized
// in Turrets with 100% of production allocated to them, across three captures
// and four different region counts.
//
// Only Industrial, Mountain and the region total feed the formula, so the other
// counts here are padding chosen to reproduce the captured total.
func TestProjectedProductionMatchesBRE(t *testing.T) {
	cases := []struct {
		industrial, mountain, total, want int
	}{
		{250, 255, 2635, 5645},
		{436, 435, 3517, 10461},
		{686, 635, 4017, 17698},
		{1086, 635, 4417, 27202},
	}
	for _, c := range cases {
		mix := RegionMix{Industrial: c.industrial, Mountain: c.mountain}
		mix.Desert = c.total - mix.Total() // padding to reach the captured total
		e := &Empire{Regions: mix, Specialized: "Turrets", ProdTurrets: 100}
		if got := (&World{}).ProjectedProduction(e)[2]; got != c.want {
			t.Errorf("industrial=%d mountain=%d total=%d: got %d turrets, want %d",
				c.industrial, c.mountain, c.total, got, c.want)
		}
	}
}

// TestIndustryMountainBoost covers the edges of the multiplier: no mountains is
// no boost, and a mountain-heavy realm is held at the cap.
func TestIndustryMountainBoost(t *testing.T) {
	ratio := func(r RegionMix) float64 {
		num, den := industryMountainBoost(r)
		return float64(num) / float64(den)
	}
	if got := ratio(RegionMix{Industrial: 100}); got != 1.0 {
		t.Errorf("no mountains: got %v, want 1.0", got)
	}
	if got := ratio(RegionMix{}); got != 1.0 {
		t.Errorf("empty realm: got %v, want 1.0", got)
	}
	if got := ratio(RegionMix{Mountain: 90, Industrial: 10}); got != 1.5 {
		t.Errorf("mountain-heavy: got %v, want the 1.5 cap", got)
	}
}
