package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

func spyEntry(board, empire string, filed time.Time, land, off, def int, gold int64) game.SpyEntry {
	return game.SpyEntry{
		SpyReport: game.SpyReport{Board: board, Empire: empire, Land: land, Offense: off, Defense: def, Gold: gold},
		Filed:     filed,
	}
}

// The viewer walks planet, then player, then that player's reports. The planet
// list offers only the planets holding intel, and a realm with more than one
// report ends with the change since the report before its last.
func TestSpyDatabaseWalksPlanetThenPlayer(t *testing.T) {
	w := leagueCtx(t)
	t0 := time.Date(2026, 9, 22, 14, 10, 0, 0, time.UTC)
	w.SpyDatabase = []game.SpyEntry{
		spyEntry("Nova Hub", "Victim", t0, 1200, 45000, 30000, 2_000_000),
		spyEntry("Nova Hub", "Other", t0.Add(time.Hour), 500, 10, 20, 30),
		spyEntry("Nova Hub", "Victim", t0.Add(19*time.Hour), 1150, 38000, 30000, 2_100_000),
		// The widest figures a report can hold, to prove the table still fits.
		spyEntry("Nova Hub", "Big", t0, 99_999_999, 2_000_000_000, 2_000_000_000, 99_999_999_999),
		spyEntry("Nova Hub", "Big", t0.Add(time.Hour), 0, 0, 0, 0),
	}
	// "?" lists the planets, "1" picks Nova Hub, "A" the first realm with intel.
	f := drive(t, w, "?1\rA\r1\rC\r\r", spyDatabase)
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "Big of Nova Hub") {
		t.Fatalf("never reached the second realm:\n%s", out)
	}
	list := out[strings.Index(out, "List of Planets"):strings.Index(out, "Our current relations")]
	if !strings.Contains(list, "Nova Hub") || strings.Contains(list, "The Eclipse") {
		t.Errorf("the planet list should hold only the planet with intel:\n%s", list)
	}
	if !strings.Contains(out, "We have information on the following players:") {
		t.Fatalf("never reached the player list:\n%s", out)
	}
	if !strings.Contains(out, "Victim of Nova Hub") {
		t.Fatalf("never showed the chosen realm:\n%s", out)
	}
	if strings.Contains(out, "Other of Nova Hub") {
		t.Errorf("showed a realm that was not chosen:\n%s", out)
	}
	want := "   Change    │        -50 │        -7,000 │             0 │       +100,000"
	if !strings.Contains(out, want) {
		t.Errorf("no change row %q:\n%s", want, out)
	}
	// As on the news screen, figures are bright cyan and only a rise's "+" is
	// bright green; a fall's sign stays cyan.
	raw := f.out.String()
	for _, want := range []string{ansi.FgBrightCyan + "        -50", ansi.FgBrightGreen + "+" + ansi.FgBrightCyan + "100,000", ansi.FgBrightCyan + "             0"} {
		if !strings.Contains(raw, want) {
			t.Errorf("change row lacks %q:\n%q", want, raw)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line runs %d columns: %q", len([]rune(line)), line)
		}
	}
}

// With no league there is no planet to pick, so the viewer goes straight to
// the players, and a baron reads only the local reports it bought.
func TestSpyDatabaseLocalGameShowsOnlyOwnReports(t *testing.T) {
	w := newWorld()
	me := w.Player().Name
	t0 := time.Date(2026, 9, 22, 14, 10, 0, 0, time.UTC)
	mine := spyEntry("", "Mine", t0, 900, 1, 2, 3)
	mine.By = me
	theirs := spyEntry("", "Theirs", t0, 800, 1, 2, 3)
	theirs.By = "Someone Else"
	w.SpyDatabase = []game.SpyEntry{mine, theirs, spyEntry("Nova Hub", "Remote", t0, 1, 1, 1, 1)}

	f := drive(t, w, "A\r\r", spyDatabase)
	out := stripANSI(f.out.String())
	if strings.Contains(out, "Enter Planet Name") {
		t.Errorf("a local game asked for a planet:\n%s", out)
	}
	if !strings.Contains(out, "We have information on the following players:") {
		t.Fatalf("never reached the player list:\n%s", out)
	}
	if strings.Contains(out, "Theirs") || strings.Contains(out, "Remote") {
		t.Errorf("listed a report this baron may not read:\n%s", out)
	}
	if strings.Count(out, "Mine") < 2 || !strings.Contains(out, "900") {
		t.Errorf("never showed the baron's own report:\n%s", out)
	}
}

// A baron's own planet is listed for its local reports, and choosing it prints
// no relations line: a planet holds none with itself.
func TestSpyDatabaseHomePlanetHasNoRelationLine(t *testing.T) {
	w := leagueCtx(t)
	mine := spyEntry("", "Neighbor", time.Now(), 900, 1, 2, 3)
	mine.By = w.Player().Name
	w.SpyDatabase = []game.SpyEntry{mine}

	f := drive(t, w, "3\rA\r\r", spyDatabase)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Neighbor of Eye of the Storm") {
		t.Fatalf("the local report is not under this planet:\n%s", out)
	}
	if strings.Contains(out, "Our current relations") {
		t.Errorf("printed a relations line for this planet:\n%s", out)
	}
}
