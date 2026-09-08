package game

import (
	"testing"
	"time"
)

// GameDay counts maintenance runs, and seasons (Config.GameLength) are measured
// in it -- so it has to mean the same thing on a board played every day and one
// played twice a month, or a league's boards would reach the end of a season at
// different real-world times. The catch-up is what makes that true: before it,
// GameDay tracked the game clock exactly but both lagged real time by a day for
// every day nobody logged in.
func TestGameDayTracksTheCalendarHoweverTheBoardIsPlayed(t *testing.T) {
	const start = "2026-01-01"
	cases := []struct {
		name string
		days []string
	}{
		{"played every day", nil}, // filled below
		{"played once, sixty days later", []string{"2026-03-02"}},
		{"played in three bursts", []string{"2026-01-14", "2026-02-09", "2026-03-02"}},
	}
	d, _ := time.Parse("2006-01-02", start)
	for i := 0; i < 60; i++ {
		d = d.AddDate(0, 0, 1)
		cases[0].days = append(cases[0].days, d.Format("2006-01-02"))
	}

	for _, c := range cases {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.AddHuman("me", "Mine")
		w.DailyMaintenance(start) // the first run only records the date
		for _, day := range c.days {
			w.DailyMaintenance(day)
		}
		from, _ := time.Parse("2006-01-02", start)
		to, err := time.Parse("2006-01-02", w.LastMaintDate)
		if err != nil {
			t.Fatalf("%s: unparseable clock %q", c.name, w.LastMaintDate)
		}
		elapsed := int(to.Sub(from).Hours() / 24)
		if elapsed != 60 {
			t.Errorf("%s: clock reached %q, want 60 days on from %s", c.name, w.LastMaintDate, start)
		}
		if w.GameDay != elapsed {
			t.Errorf("%s: GameDay is %d after %d calendar days", c.name, w.GameDay, elapsed)
		}
	}
}
