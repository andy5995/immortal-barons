package game

import "testing"

// The dial is part of the setting, in the original's own encoding: 0 None,
// 1 User Select, 2 Random, 200-210 a Constant firing (value - 200). Golden
// literals, not the constants — the numbers ARE the fidelity contract.
// The four modes and the one that prompts are the original's; the Constant's
// dial is a field of IB's own because the original has nowhere to put one.
func TestSabreModes(t *testing.T) {
	for _, c := range []struct {
		m    SabreMode
		want string
	}{
		{SabreNone, "None/Disabled"},
		{SabreUserSelect, "User Select/Original"},
		{SabreRandom, "Random"},
		{SabreConstant, "Constant"},
	} {
		if got := c.m.String(); got != c.want {
			t.Errorf("%d = %q, want %q", c.m, got, c.want)
		}
	}
}

// Only User Select uses the number the player typed; Random rolls one and a
// Constant ignores the player entirely.
func TestSabreDialForHonoursTheMode(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)

	w.Config.SabreHandling = SabreUserSelect
	if got := w.sabreDialFor(7); got != 7 {
		t.Errorf("User Select should take the player's 7, got %d", got)
	}
	w.Config.SabreHandling, w.Config.SabreConstantDial = SabreConstant, 4
	if got := w.sabreDialFor(7); got != 4 {
		t.Errorf("a Constant of 4 should ignore the player's 7, got %d", got)
	}
	w.Config.SabreHandling = SabreRandom
	seen := map[int]bool{}
	for i := 0; i < 300; i++ {
		d := w.sabreDialFor(7)
		if d < SabreDialMin || d > SabreDialMax {
			t.Fatalf("Random produced an out-of-range dial %d", d)
		}
		seen[d] = true
	}
	if len(seen) < 5 {
		t.Errorf("Random should spread across the range, saw only %d values", len(seen))
	}
}

// The renumbering swapped the meaning of 0, so an existing board's setting has
// to be carried across or every upgrade would silently disable the weapon.
func TestSabreHandlingMigration(t *testing.T) {
	for _, c := range []struct {
		old  SabreMode
		want SabreMode
	}{
		{0, SabreUserSelect}, // the old default, and the dangerous one
		{1, SabreNone},
		{2, SabreRandom},
		{3, SabreConstant},
	} {
		if got := MigrateSabreHandling(c.old); got != c.want {
			t.Errorf("legacy %d migrated to %v, want %v", c.old, got, c.want)
		}
	}
}
