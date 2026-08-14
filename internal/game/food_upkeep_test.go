package game

import "testing"

// Every figure asserted here is a golden literal read from BRE — a live prompt
// or, for the weights, the two need routines in BRE.OVR's food overlay unit.
// Do not rewrite one as an expression over a balance.go constant: the point is
// that a retune has to produce new evidence, not a quietly following test.

// The armed forces bill 1 food per 200 troopers. Live-verified twice: 42,259
// troopers show "Armed Forces Require 211 units of food", and a second realm
// billed 36 for 7,212.
func TestForcesFoodTrooperRate(t *testing.T) {
	if got := (&Empire{Troopers: 42259}).ForcesFoodUpkeep(); got != 211 {
		t.Errorf("42,259 troopers should eat 211 food, got %d", got)
	}
	if got := (&Empire{Troopers: 7212}).ForcesFoodUpkeep(); got != 36 {
		t.Errorf("7,212 troopers should eat 36 food, got %d", got)
	}
}

// Every other military type bills 1 food per 10,000, and the total truncates.
// Pinned in play on a tank-only realm: 30,000 tanks bill 3 and 29,999 bill 2,
// which allows only 9,999.67 … 10,000 and proves truncation over rounding. A
// turret-only realm billed 81 for 816,657 turrets.
func TestForcesFoodUnitRate(t *testing.T) {
	for _, tc := range []struct {
		name string
		e    Empire
		want int
	}{
		{"30,000 tanks", Empire{Tanks: 30000}, 3},
		{"29,999 tanks", Empire{Tanks: 29999}, 2},
		{"816,657 turrets", Empire{Turrets: 816657}, 81},
		{"12,846 turrets", Empire{Turrets: 12846}, 1},
		{"8,960 turrets", Empire{Turrets: 8960}, 0},
	} {
		if got := tc.e.ForcesFoodUpkeep(); got != tc.want {
			t.Errorf("%s: want %d food, got %d", tc.name, tc.want, got)
		}
	}
}

// All six military types are charged — BRE's own changelog says "All military
// units now require food to survive", and its armed-forces routine sums a term
// for each. IB charged troopers only until #91.
func TestForcesFoodChargesEveryUnitType(t *testing.T) {
	e := Empire{Jets: 10000, Turrets: 10000, Bombers: 10000, Tanks: 10000, Carriers: 10000}
	if got := e.ForcesFoodUpkeep(); got != 5 {
		t.Errorf("10,000 of each non-trooper type should eat 5 food, got %d", got)
	}
}

// The bill is ONE weighted sum truncated once, not a truncation per unit type:
// five types holding 9,999 each are 4.9995 food together, which BRE drops to 4,
// where per-type truncation would drop all five to zero.
func TestForcesFoodTruncatesOnceOverTheWholeArmy(t *testing.T) {
	e := Empire{Jets: 9999, Turrets: 9999, Bombers: 9999, Tanks: 9999, Carriers: 9999}
	if got := e.ForcesFoodUpkeep(); got != 4 {
		t.Errorf("five types at 9,999 each should eat 4 food, got %d", got)
	}
}

// The old measurement that "cleared" jets and tanks added 1,000 jets and 533
// tanks to 7,212 troopers and saw the bill stay at 36. It still is 36 — those
// counts are worth 0.15 food, so the test never had the power to detect them.
func TestForcesFoodSmallAirWingIsInvisible(t *testing.T) {
	e := Empire{Troopers: 7212, Jets: 1000, Tanks: 533}
	if got := e.ForcesFoodUpkeep(); got != 36 {
		t.Errorf("want 36 food, got %d", got)
	}
}

// Units escrowed on the Trading Market eat too: BRE reads each type from both
// its home field and its market slot into the same sum.
func TestForcesFoodCountsMarketEscrow(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("feeder", "Feedoria")
	e.Troopers, e.Tanks = 20000, 20000
	if got := e.ForcesFoodUpkeep(); got != 102 {
		t.Fatalf("held army should eat 102 food, got %d", got)
	}
	if err := w.SetMarketListing(e, "Trooper", 8000, 10); err != nil {
		t.Fatalf("listing troopers: %v", err)
	}
	if got := e.ForcesFoodUpkeep(); got != 62 {
		t.Fatalf("escrow leaves 12,000 troopers in hand, eating 62 food, got %d", got)
	}
	if got := w.ForcesFoodDue(e); got != 102 {
		t.Errorf("listing troopers must not cut the food bill: want 102, got %d", got)
	}
}

// The people eat 1.5 food per million, and BRE counts population in millions
// while IB counts people. Live prompts: 651M bills 976, 1,161M bills 1,741 —
// both trunc(x1.5) of a half-unit figure.
func TestPeopleFoodMatchesBREPerMillion(t *testing.T) {
	for _, tc := range []struct {
		millions, want int
	}{
		{651, 976},
		{890, 1335},
		{1161, 1741},
		{2131, 3196},
	} {
		e := Empire{People: tc.millions * PopBREUnitScale}
		if got := e.PeopleFoodUpkeep(); got != tc.want {
			t.Errorf("%dM people should eat %d food, got %d", tc.millions, tc.want, got)
		}
	}
}

// The two obligations are truncated SEPARATELY and then added. A turret-only
// realm of 25,865M people and 49,840 turrets consumed 38,801 food; one
// accumulator truncated once gives 38,802.
func TestFoodUpkeepTruncatesTheTwoObligationsSeparately(t *testing.T) {
	for _, tc := range []struct {
		millions, turrets, want int
	}{
		{25865, 49840, 38801},
		{27573, 49840, 41363},
		{35207, 219032, 52831},
		{36623, 278857, 54961},
		{32861, 0, 49291}, // "Military: None" control turn
	} {
		e := Empire{People: tc.millions * PopBREUnitScale, Turrets: tc.turrets}
		if got := e.FoodUpkeep(); got != tc.want {
			t.Errorf("%dM people + %d turrets should eat %d food, got %d",
				tc.millions, tc.turrets, tc.want, got)
		}
	}
}
