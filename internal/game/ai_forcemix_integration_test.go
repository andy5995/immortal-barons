package game

import "testing"

// TestAIForceMixFiresInSeededGame is an end-to-end guard (#36): after a week of
// daily maintenance in a normally-seeded world, AI empires should have reached
// the food-healthy state and built a defensive-capable mix (turrets > 0), not
// just troopers. Catches regressions where AI realms never reach the force-mix
// step in a real game even though the unit tests (which hand-set food-healthy)
// still pass.
func TestAIForceMixFiresInSeededGame(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 3
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-31"
	for _, d := range []string{"2026-08-01", "2026-08-02", "2026-08-03", "2026-08-04",
		"2026-08-05", "2026-08-06", "2026-08-07", "2026-08-08"} {
		w.DailyMaintenance(d)
	}
	withTurrets := 0
	for _, e := range w.AIEmpires() {
		if e.Alive && e.Turrets > 0 && e.Defense() > 0 {
			withTurrets++
		}
	}
	if withTurrets == 0 {
		t.Error("no AI empire built turret defense after a week — force mix never fired in a seeded game")
	}
}
