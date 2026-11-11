package game

import "testing"

// Land waiting in the picker's pool is land the realm owns. A realm whose only
// land is there (a tiny all-waste realm that cleaned it all, then dropped at
// the picker) must survive the fallen-realm sweep.
func TestPendingLandKeepsARealmAlive(t *testing.T) {
	w := testWorld()
	w.DailyMaintenance("2026-11-11") // the day the game starts runs no sweep
	e := w.AddHuman("baron", "Poolland")
	e.Regions = RegionMix{}
	e.syncLand()
	e.PendingRegions = 10
	e.LastPlayed = "2026-11-11" // a never-played realm is reaped on its own rule
	w.DailyMaintenance("2026-11-12")
	if e = w.FindByName("Poolland"); e == nil || !e.Alive {
		t.Fatal("a realm holding 10 untyped regions was killed as landless")
	}

	e.PendingRegions = 0
	w.DailyMaintenance("2026-11-13")
	if e = w.FindByName("Poolland"); e != nil && e.Alive {
		t.Fatal("a realm with no land at all survived the sweep; the test no longer reaches it")
	}
}
