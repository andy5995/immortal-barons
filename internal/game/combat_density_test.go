package game

import "testing"

// A defender whose net worth is spread thinly over its land (low net-worth-per-
// region) yields a higher capture factor than a dense one, relative to the
// attacker.
func TestCaptureDensityFactor(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Land: 100, Troopers: 1_000_000}      // reference density
	soft := &Empire{Land: 1000}                       // land only: low NW/region
	dense := &Empire{Land: 100, Carriers: 10_000_000} // rich per region

	fs := w.captureDensityFactor(a, soft)
	fd := w.captureDensityFactor(a, dense)
	if !(fs > fd) {
		t.Errorf("soft target should give a higher factor than a dense one: soft=%d dense=%d", fs, fd)
	}
}

func TestCaptureDensityFactorClamps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Land: 100, Troopers: 1_000_000}
	verySoft := &Empire{Land: 1_000_000}                 // density near zero
	veryDense := &Empire{Land: 1, Carriers: 100_000_000} // density enormous

	if f := w.captureDensityFactor(a, verySoft); f != CaptureDensityMax {
		t.Errorf("very soft target should clamp to max %d, got %d", CaptureDensityMax, f)
	}
	if f := w.captureDensityFactor(a, veryDense); f != CaptureDensityMin {
		t.Errorf("very dense target should clamp to min %d, got %d", CaptureDensityMin, f)
	}
}

// An equal-density defender (same net-worth-per-region as the attacker) takes
// the base capture, no modifier.
func TestCaptureDensityFactorEqualIsBase(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Land: 500, Troopers: 200_000}
	d := &Empire{Land: 500, Troopers: 200_000}
	if f := w.captureDensityFactor(a, d); f != CaptureDensityBase {
		t.Errorf("equal density should give the base %d, got %d", CaptureDensityBase, f)
	}
}

// End to end: with the same land and the same (zero) defense, a soft target
// loses more regions than a dense one — only the density factor differs, since
// carriers add net worth but no Defense().
func TestAttackCapturesMoreFromSoftTarget(t *testing.T) {
	fight := func(carriers int) int {
		w := NewWorldSeed(DefaultConfig(), 1)
		a := &Empire{Name: "A", Troopers: 1_000_000, Morale: 100, Alive: true,
			Land: 1000, Regions: RegionMix{Coastal: 1000}}
		d := &Empire{Name: "D", Morale: 100, Alive: true, People: 1_000_000,
			Land: 2000, Regions: RegionMix{Coastal: 2000}, Carriers: carriers}
		_, captured := w.Attack(a, d, FullForce(a), false)
		return captured
	}
	soft := fight(0)
	dense := fight(50_000_000)
	if soft <= dense {
		t.Errorf("a soft (low net-worth-per-region) target should lose more regions: soft=%d dense=%d", soft, dense)
	}
}
