package game

import (
	"strings"
	"testing"
)

// The Planetary Master line opens the CURRENT day's news, as it does in the
// original (it is the first item under the Daily Bulletin box). It used to be
// posted before rollNews, which moved it straight into yesterday and left
// Today's News empty every day -- the "No planetary bulletins." report.
func TestMasterNewsLandsInTodaysNews(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.Pirates = nil
	e := w.AddHuman("h", "Realm")
	e.Land, e.Gold, e.Food, e.People = 500, 5_000_000, 500_000, 50_000
	e.LastPlayed = "2026-07-01" // an active realm; an unplayed one is swept as abandoned

	w.LastMaintDate = "2026-07-01"
	w.DailyMaintenance("2026-07-02")

	joined := strings.Join(w.NewsToday, "\n")
	if !strings.Contains(joined, "Planetary Master") {
		t.Fatalf("Today's News carries no Planetary Master line.\ntoday:     %v\nyesterday: %v",
			w.NewsToday, w.NewsYesterday)
	}
	for _, l := range w.NewsYesterday {
		if strings.Contains(l, "Planetary Master") {
			t.Errorf("the Master line was filed under yesterday: %q", l)
		}
	}
}

// A second day still opens with it, and the previous day's news is what rolls
// back -- so the roll is happening, not merely skipped.
func TestNewsRollsWhileTodayStillOpens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 2)
	w.Pirates = nil
	e := w.AddHuman("h", "Realm")
	e.Land, e.Gold, e.Food, e.People = 500, 5_000_000, 500_000, 50_000
	e.LastPlayed = "2026-07-01" // an active realm; an unplayed one is swept as abandoned

	w.LastMaintDate = "2026-07-01"
	w.DailyMaintenance("2026-07-02")
	w.postNews("something that happened during the day")
	w.LastMaintDate = "2026-07-02"
	w.DailyMaintenance("2026-07-03")

	if !strings.Contains(strings.Join(w.NewsToday, "\n"), "Planetary Master") {
		t.Errorf("day two did not open with the Master line: %v", w.NewsToday)
	}
	if !strings.Contains(strings.Join(w.NewsYesterday, "\n"), "something that happened") {
		t.Errorf("the day's own news did not roll back: %v", w.NewsYesterday)
	}
}
