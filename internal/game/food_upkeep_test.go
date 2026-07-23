package game

import "testing"

// Army food is billed at ~1 per ArmyFoodDivisor army-units, live-verified against
// BRE: an empire with 42,259 troopers (no jets/tanks) shows "Armed Forces Require
// 211 units of food" — 42,259/200 = 211. Jets and tanks weigh 2× a trooper.
func TestFoodUpkeepArmyRate(t *testing.T) {
	var e Empire
	e.Troopers = 42259
	if got := e.FoodUpkeep(); got != 211 {
		t.Errorf("42,259 troopers should eat 211 food (BRE-verified), got %d", got)
	}

	// Jets and tanks count double against the divisor.
	e = Empire{Troopers: 200, Jets: 100, Tanks: 100} // (200 + 200 + 200)/200 = 3
	if got := e.FoodUpkeep(); got != 3 {
		t.Errorf("weighted army units 600/200 should eat 3 food, got %d", got)
	}
}

// The population share is unchanged by the army-rate fix.
func TestFoodUpkeepPeopleShare(t *testing.T) {
	e := Empire{People: 100_000} // 100,000 × 75 / 1000 = 7,500
	if got := e.FoodUpkeep(); got != 7500 {
		t.Errorf("100,000 people should eat 7,500 food, got %d", got)
	}
}
