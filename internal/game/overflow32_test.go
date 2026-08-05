package game

import "testing"

// A 32-bit build (a Win32 BBS door) has 32-bit ints, and several of the game's
// products pass 2^31 at ordinary empire sizes. These run on any build — the
// arithmetic they pin is the same width everywhere once the intermediates are
// int64 — but they are the cases that were actually wrong on 32-bit, so keep
// them and run the suite with GOARCH=386 when touching this math.

// The reported case: a 710-region realm on a 32-bit door was billed -210,763
// gold in region maintenance, because Land*913*10000 wrapped inside techLower.
func TestRegionUpkeepStaysPositiveAtDoorScale(t *testing.T) {
	for _, land := range []int{235, 710, 6883, 20000} {
		e := &Empire{Land: land}
		if got := e.RegionUpkeep(); got <= 0 {
			t.Errorf("land=%d: region upkeep came out %d, want a positive charge", land, got)
		}
		if want := land * RegionUpkeepPerLand; e.RegionUpkeep() != want {
			t.Errorf("land=%d: region upkeep %d, want %d with no technology", land, e.RegionUpkeep(), want)
		}
	}
}

func TestTechScalingSurvivesLargeValues(t *testing.T) {
	// Two hundred million, well past what wraps at 32 bits once multiplied by
	// TechFactorUnit.
	const v = 200_000_000
	if got := techLower(v, TechFactorUnit); got != v {
		t.Errorf("techLower with a neutral factor = %d, want %d", got, v)
	}
	if got := techRaise(v, TechFactorUnit); got != v {
		t.Errorf("techRaise with a neutral factor = %d, want %d", got, v)
	}
}

func TestPctOfSurvivesMoneyScale(t *testing.T) {
	if got := pctOf(50_000_000, 80); got != 40_000_000 {
		t.Errorf("pctOf(50m, 80) = %d, want 40,000,000", got)
	}
	if got := pctOf(2_000_000_000, 100); got != 2_000_000_000 {
		t.Errorf("pctOf at the money cap = %d, want it unchanged", got)
	}
}

// A rich AI must still spend: its budget is a percentage of gold, which wrapped
// negative on 32-bit and left it buying nothing at all.
func TestRichAIStillBuysForces(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AIProfile, e.AISkill = AIProfileAggressor, AISkillSharp
	e.Regions = RegionMix{Coastal: 50}
	e.syncLand()
	e.Gold = 500_000_000
	w.aiBuildForces(e)
	if e.Troopers+e.Turrets+e.Tanks+e.Jets == 0 {
		t.Error("an AI holding 500m gold bought no forces at all")
	}
	if e.Gold < 0 {
		t.Errorf("spending drove gold negative: %d", e.Gold)
	}
}
