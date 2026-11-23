package game

import (
	"errors"
	"testing"
)

// Buying and building stop at MaxUnits instead of wrapping the count negative,
// which a cheap modded unit reaches in two purchases on a 32-bit door.
func TestUnitCountsStopAtTheCeiling(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Full")
	e.Troopers = MaxUnits - 5
	e.Gold = 1_000_000_000
	gold := e.Gold
	if err := w.Buy(e, Trooper, 10); !errors.Is(err, ErrUnitCap) {
		t.Fatalf("buying past the ceiling: err %v, want ErrUnitCap", err)
	}
	if e.Troopers != MaxUnits-5 || e.Gold != gold {
		t.Errorf("a refused buy changed the realm: troopers %d, gold %d", e.Troopers, e.Gold)
	}
	if err := w.Buy(e, Trooper, 5); err != nil || e.Troopers != MaxUnits {
		t.Errorf("buying up to the ceiling: troopers %d, err %v", e.Troopers, err)
	}

	e.Regions.Industrial = 100
	e.Specialized = ""
	*Trooper.Prod(e) = 100
	w.Manufacture(e)
	if e.Troopers != MaxUnits || e.MadeTroopers != 0 {
		t.Errorf("production at the ceiling: troopers %d, made %d (want none reported)", e.Troopers, e.MadeTroopers)
	}
}
