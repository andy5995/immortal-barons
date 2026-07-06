package game

import "testing"

func TestPlanetTotalsSumsOnlyLivingEmpires(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	alive := w.AddHuman("alive", "Alive")
	alive.People = 100
	alive.Land = 50
	alive.Alive = true

	dead := w.AddHuman("dead", "Dead")
	dead.People = 999
	dead.Land = 999
	dead.Alive = false

	got := planetTotals(w)
	want := PlanetTotals{
		Population: alive.People,
		Regions:    alive.Land,
		NetWorth:   w.NetWorth(alive),
	}
	if got != want {
		t.Errorf("planetTotals = %+v, want %+v", got, want)
	}
}

func TestPostNewsLandsInNewsToday(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.postNews("test line")
	if len(w.NewsToday) != 1 || w.NewsToday[0] != "test line" {
		t.Errorf("NewsToday = %v, want [\"test line\"]", w.NewsToday)
	}
}

func TestRollNewsRotatesBulletinAndNews(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.Pirates = nil // keep news deterministic: no random pirate-raid lines
	e := w.AddHuman("h", "Realm")
	w.LastMaintDate = "2026-07-01"

	// Day 1.
	w.postNews("day one news")
	w.DailyMaintenance("2026-07-02")

	day1Totals := planetTotals(w)
	if w.BulletinToday.Totals != day1Totals {
		t.Errorf("day1 BulletinToday.Totals = %+v, want %+v", w.BulletinToday.Totals, day1Totals)
	}
	if w.BulletinToday.Change != day1Totals {
		t.Errorf("first-ever day Change should equal Totals: got %+v, want %+v", w.BulletinToday.Change, day1Totals)
	}
	if w.NewsToday != nil {
		t.Errorf("NewsToday should be cleared after maintenance, got %v", w.NewsToday)
	}

	// Grow the empire so day 2's totals differ, then post more news before
	// the next maintenance cycle.
	e.People += 500
	e.Land += 25
	w.postNews("day two news")

	prevTotals := w.BulletinToday.Totals
	w.DailyMaintenance("2026-07-03")

	// Maintenance also posts its own economic/political news (invest-rate
	// moves, Planetary Master standing) alongside the manually-posted line,
	// so check "day two news" made it through rather than asserting an
	// exact line count.
	found := false
	for _, line := range w.NewsYesterday {
		if line == "day two news" {
			found = true
		}
	}
	if !found {
		t.Errorf("NewsYesterday = %v, want to contain %q", w.NewsYesterday, "day two news")
	}
	if w.NewsToday != nil {
		t.Errorf("NewsToday should be cleared after second maintenance, got %v", w.NewsToday)
	}

	wantBulletinYesterday := DailyBulletin{Totals: day1Totals, Change: day1Totals}
	if w.BulletinYesterday != wantBulletinYesterday {
		t.Errorf("BulletinYesterday = %+v, want %+v", w.BulletinYesterday, wantBulletinYesterday)
	}

	day2Totals := planetTotals(w)
	wantChange := PlanetTotals{
		Population: day2Totals.Population - prevTotals.Population,
		Regions:    day2Totals.Regions - prevTotals.Regions,
		NetWorth:   day2Totals.NetWorth - prevTotals.NetWorth,
	}
	if w.BulletinToday.Totals != day2Totals {
		t.Errorf("day2 BulletinToday.Totals = %+v, want %+v", w.BulletinToday.Totals, day2Totals)
	}
	if w.BulletinToday.Change != wantChange {
		t.Errorf("day2 BulletinToday.Change = %+v, want %+v", w.BulletinToday.Change, wantChange)
	}
}
