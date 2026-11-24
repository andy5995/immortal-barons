package menu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

// The Game Setup screen is paged: the full ruleset does not fit 80x25, so it
// prints in two panels with a pause between them. Both must appear, and a limit
// whose zero means "no limit" must say so rather than printing a bare 0.
func TestGameSetupPagesAndNamesZeroLimits(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")} // dismiss both pauses
	w := newWorld()
	w.Config.GameLength = 0     // endless
	w.Config.MaxRegions = 0     // unlimited
	w.Config.IdleDaysRemove = 7 // a real limit, to check the unit
	gameSetup(f, w)
	out := f.out.String()

	for _, want := range []string{
		"The day and the game", "Land", "Money", // page one groups
		"Forces and trade", "Attacking", "This board", // page two groups
		"Turns per day", "New land per realm/day", "Regions bought/turn", // page one
		"Buy military", "Attack damage", // page two
		"Endless", "Unlimited", "7 days",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Paused"); n != 2 {
		t.Errorf("expected a pause after each of the two panels, got %d\n%s", n, out)
	}
}

// With Inter-BBS play on, the screen must say the ruleset is not this sysop's
// to set — the Coordinator broadcasts it to every board in the league.
func TestGameSetupNamesTheLeague(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")}
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "eye of the storm"
	gameSetup(f, w)
	out := f.out.String()

	for _, want := range []string{"The league", "eye of the storm", "Coordinator", "League number", "Individual attacks/day"} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q with IBBS on:\n%s", want, out)
		}
	}
	// The NUMBER is always there — it is what keeps two leagues apart, so a sysop
	// diagnosing one should not have to open bbs.cfg to read it.
	if !strings.Contains(out, "League number") {
		t.Errorf("Game Setup omits the league number:\n%s", out)
	}

	// With it off, the league group is absent entirely.
	f2 := &fakeSession{keys: []rune("  ")}
	w2 := newWorld()
	w2.Config.IBBS = false
	gameSetup(f2, w2)
	if strings.Contains(f2.out.String(), "Coordinator") {
		t.Error("league group should not appear on a standalone board")
	}
}

// The money cap has no row on Game Setup. It had one until 2026-09-01, when the
// setting stopped being editable (#205) and every new game began getting the
// same two billion — a figure the screen no longer has to carry because it
// cannot differ between boards a player might join.
func TestGameSetupOmitsTheMoneyCap(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("     ")}
	gameSetup(f, w)
	out := anyEscape.ReplaceAllString(f.out.String(), "")
	// Assert the screen was reached before asserting what is missing from it —
	// an empty screen would otherwise pass.
	if !strings.Contains(out, "Turns per day") {
		t.Fatalf("never reached Game Setup:\n%s", out)
	}
	if strings.Contains(out, "Most gold you can hold") {
		t.Error("Game Setup still carries the money cap row")
	}
}

// Game Setup must fit an 80x25 terminal, and a LEAGUE board is the worst case —
// it carries the league group and the whole interplanetary ruleset on top of
// everything a stand-alone board shows. This regressed once: one setting per
// line ran the second panel to 37 lines, so its first dozen rows scrolled off
// unread. The pairing in gameSetup is what holds it, and only a line count
// catches it going again.
func TestGameSetupPanelsFitTheScreen(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")}
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "Wildside"
	w.Config.LeagueNumber = 618
	w.LeagueNodes = []game.LeagueNode{{Number: 1, Name: "Wildside"}, {Number: 2, Name: "Faraway"}}
	gameSetup(f, w)

	// A panel gets 24 of the 25 rows; the pause prompt takes the last.
	const maxRows, maxCols = 24, 80
	for i, panel := range strings.Split(stripANSI(f.out.String()), ">Paused<") {
		lines := strings.Split(strings.Trim(panel, "\n"), "\n")
		if len(lines) > maxRows {
			t.Errorf("panel %d is %d lines, over the %d a screen holds:\n%s", i+1, len(lines), maxRows, panel)
		}
		for _, ln := range lines {
			if n := len([]rune(strings.TrimRight(ln, " "))); n > maxCols {
				t.Errorf("panel %d has a %d-column line: %q", i+1, n, ln)
			}
		}
	}
}

// modRefForSetup makes the saved copy of a mod a Game Setup screen reads. Its
// option label and chosen value must come from that copy, not a sysop's folder.
func modRefForSetup(name, label string) game.ModRef {
	manifest := fmt.Sprintf("name = %q\nversion = \"1.0\"\nlabel = %q\nmod_api = 1\nlicense = \"CC0-1.0\"\n"+
		"[options.tank_cost]\ntype = \"number\"\nlabel = \"Tank cost\"\ndefault = 500\nmin = 1\nmax = 1000\n",
		name, label)
	return game.ModRef{
		Name: name, Version: "1.0", Label: label,
		Files:   map[string]string{mod.ManifestFile: manifest},
		Options: map[string]int64{"tank_cost": 800},
	}
}

// A modded game gets a third screen naming each mod and its option values;
// unmodded games keep two. Option values are shown, not just mod names (#355).
func TestGameSetupListsTheMods(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")}
	w := newWorld()
	gameSetup(f, w)
	if strings.Contains(f.out.String(), "Mods") {
		t.Errorf("an unmodded game shows a Mods group:\n%s", f.out.String())
	}
	if n := strings.Count(f.out.String(), "Paused"); n != 2 {
		t.Errorf("unmodded game had %d pauses, want 2", n)
	}

	f = &fakeSession{keys: []rune("   ")}
	w.Mods = []game.ModRef{
		modRefForSetup("cheap-tanks", "Cheap Tanks"),
		{Name: "bare", Version: strings.Repeat("9", 100)},
	}
	gameSetup(f, w)
	out := stripANSI(f.out.String())
	for _, want := range []string{"Mods (page 1 of 1)", "Cheap Tanks", "1.0", "Tank cost: 800", "bare", "Paused"} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Tank cost: 500") {
		t.Error("Game Setup printed the option default, not this game's value")
	}
	for _, ln := range strings.Split(out, "\n") {
		if n := len([]rune(strings.TrimRight(ln, " "))); n > 80 {
			t.Errorf("a %d-column line: %q", n, ln)
		}
	}
}

// More mods than a screen holds get a second page, each with its own pause.
// A league board is the worst case, so every panel must still fit 80x24.
func TestGameSetupPagesTheMods(t *testing.T) {
	f := &fakeSession{keys: []rune("    ")}
	w := newWorld()
	w.Config.IBBS = true
	for i := 0; i < 20; i++ {
		w.Mods = append(w.Mods, game.ModRef{
			Name: fmt.Sprintf("mod-%02d", i), Version: "1.0",
			Label: fmt.Sprintf("Mod %02d", i),
		})
	}
	gameSetup(f, w)
	out := stripANSI(f.out.String())
	for _, want := range []string{"Mods (page 1 of 2)", "Mods (page 2 of 2)"} {
		if !strings.Contains(out, want) {
			t.Errorf("Game Setup missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Paused"); n != 4 {
		t.Errorf("expected four pauses (two rules, two mods), got %d", n)
	}
	const maxRows, maxCols = 24, 80
	for i, panel := range strings.Split(out, ">Paused<") {
		lines := strings.Split(strings.Trim(panel, "\n"), "\n")
		if len(lines) > maxRows {
			t.Errorf("panel %d is %d lines, over the %d a screen holds:\n%s", i+1, len(lines), maxRows, panel)
		}
		for _, ln := range lines {
			if n := len([]rune(strings.TrimRight(ln, " "))); n > maxCols {
				t.Errorf("panel %d has a %d-column line: %q", i+1, n, ln)
			}
		}
	}
}
