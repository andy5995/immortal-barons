package game

import "testing"

// Army food is TROOPERS ONLY, ~1 per ArmyFoodDivisor of them — live-verified: an
// empire with 42,259 troopers shows "Armed Forces Require 211 units of food"
// (42,259/200 = 211).
func TestFoodUpkeepArmyRate(t *testing.T) {
	var e Empire
	e.Troopers = 42259
	if got := e.FoodUpkeep(); got != 211 {
		t.Errorf("42,259 troopers should eat 211 food (BRE-verified), got %d", got)
	}
}

// Only troopers eat. Measured on a live realm: 7,212 troopers were billed 36
// food, and the bill stayed at 36 after 1,000 jets and 533 tanks were added.
// IB used to weigh jets and tanks double, which would bill 51 for that army.
func TestFoodUpkeepOnlyTroopersEat(t *testing.T) {
	plain := Empire{Troopers: 7212}
	if got := plain.FoodUpkeep(); got != 36 {
		t.Errorf("7,212 troopers should eat 36 food, got %d", got)
	}
	withAir := Empire{Troopers: 7212, Jets: 1000, Tanks: 533}
	if got := withAir.FoodUpkeep(); got != 36 {
		t.Errorf("jets and tanks must eat nothing: want 36 food, got %d", got)
	}
	bigger := Empire{Troopers: 14685, Jets: 1000, Tanks: 533}
	if got := bigger.FoodUpkeep(); got != 73 {
		t.Errorf("14,685 troopers (same jets/tanks) should eat 73 food, got %d", got)
	}
}

// The population share is unchanged by the army-rate fix.
func TestFoodUpkeepPeopleShare(t *testing.T) {
	e := Empire{People: 100_000} // 100,000 × 75 / 1000 = 7,500
	if got := e.FoodUpkeep(); got != 7500 {
		t.Errorf("100,000 people should eat 7,500 food, got %d", got)
	}
}
