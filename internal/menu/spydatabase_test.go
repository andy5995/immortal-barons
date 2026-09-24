package menu

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

func spyEntry(empire string, filed time.Time, land, off, def int, gold int64) game.SpyEntry {
	return game.SpyEntry{
		SpyReport: game.SpyReport{Board: "boardB", Empire: empire, Land: land, Offense: off, Defense: def, Gold: gold},
		Filed:     filed,
	}
}

// Reports are grouped by realm in arrival order, and a realm with more than one
// ends with the change since the report before its last.
func TestSpyDatabaseGroupsByRealmWithChange(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	t0 := time.Date(2026, 9, 22, 14, 10, 0, 0, time.UTC)
	w.SpyDatabase = []game.SpyEntry{
		spyEntry("Victim", t0, 1200, 45000, 30000, 2_000_000),
		spyEntry("Other", t0.Add(time.Hour), 500, 10, 20, 30),
		spyEntry("Victim", t0.Add(19*time.Hour), 1150, 38000, 30000, 2_100_000),
	}
	f := &fakeSession{keys: []rune("\r\r\r")}
	spyDatabase(f, &ctx{World: w})
	out := stripANSI(f.out.String())

	victim := strings.Index(out, "Victim of boardB")
	other := strings.Index(out, "Other of boardB")
	if victim < 0 || other < 0 || other < victim {
		t.Fatalf("realms not grouped in first-seen order:\n%s", out)
	}
	if strings.Count(out, "Victim of boardB") != 1 {
		t.Errorf("Victim's reports are not under one heading:\n%s", out)
	}
	want := fmt.Sprintf("  %-11s %9s %14s %14s %15s", "Change", "-50", "-7,000", "0", "+100,000")
	if !strings.Contains(out, want) {
		t.Errorf("no change row %q:\n%s", want, out)
	}
	if strings.Count(out, "Change") != 1 {
		t.Errorf("a realm with one report should have no change row:\n%s", out)
	}
}

// A full database pages on an 80x24 screen and never runs past 80 columns.
func TestSpyDatabasePagesAndFits(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	t0 := time.Date(2026, 9, 22, 14, 10, 0, 0, time.UTC)
	for realm := range 6 {
		for i := range game.SpyReportsPerRealm {
			w.SpyDatabase = append(w.SpyDatabase, spyEntry(fmt.Sprintf("Realm%d", realm),
				t0.Add(time.Duration(i)*time.Hour), 999_999, 2_000_000_000, 2_000_000_000, 99_999_999_999))
		}
	}
	f := &fakeSession{keys: []rune(strings.Repeat("\r", 20))}
	spyDatabase(f, &ctx{World: w})
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "Realm5 of boardB") {
		t.Fatalf("never reached the last realm:\n%s", out)
	}
	pages := strings.Split(out, "Paused")
	if len(pages) < 3 {
		t.Errorf("printed %d pages for 30 reports, want it to page", len(pages)-1)
	}
	for _, page := range pages {
		if n := strings.Count(page, "\n"); n > 24 {
			t.Errorf("a page ran %d lines:\n%s", n, page)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line runs %d columns: %q", len([]rune(line)), line)
		}
	}
}
