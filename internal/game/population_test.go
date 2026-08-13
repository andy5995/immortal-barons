package game

import "testing"

// Waste houses nobody. BRE's capacity routine (BRE.OVR 0xD08A) loads eight
// region counts and never reads the ninth, Waste at +0xb6 — so ruined land
// contributes nothing to carrying capacity. Asserted as a golden literal rather
// than against PopCapWaste, so giving Waste a weight has to come with evidence.
func TestWasteHousesNobody(t *testing.T) {
	base := &Empire{Support: 90, Tax: 10}
	base.Regions.Urban = 10
	before := base.popCapacity()

	ruined := *base
	ruined.Regions.Waste = 500
	if got := ruined.popCapacity(); got != before {
		t.Errorf("500 waste regions changed capacity from %d to %d; waste must house nobody", before, got)
	}

	// And the contrast: any other type does move it, so the test above is not
	// passing because popCapacity ignores its input.
	moved := *base
	moved.Regions.Desert = 500
	if moved.popCapacity() == before {
		t.Error("desert regions should raise capacity; the comparison above proves nothing")
	}
}
