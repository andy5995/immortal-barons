package menu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// #233: the files a BBS shows on its own bulletin menu. Each is written twice,
// coloured and plain, and the plain one is the same screen with the escapes
// stripped -- not a second layout that could drift.
func TestWriteBulletinsWritesBothFormsOfEach(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 2
	cfg.BoardID = "Alpha BBS"
	cfg.IBBS = true // a league board, so the World Report is among the set
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-08-27"
	dir := t.TempDir()

	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatalf("writing bulletins: %v", errs)
	}
	for _, base := range []string{"scores", "tdynews", "yesnews", "world"} {
		ansi, err := os.ReadFile(filepath.Join(dir, base+".ans"))
		if err != nil {
			t.Fatalf("%s.ans: %v", base, err)
		}
		plain, err := os.ReadFile(filepath.Join(dir, base+".txt"))
		if err != nil {
			t.Fatalf("%s.txt: %v", base, err)
		}
		if len(ansi) == 0 || len(plain) == 0 {
			t.Errorf("%s: empty (ans %d bytes, txt %d)", base, len(ansi), len(plain))
		}
		if !strings.Contains(string(ansi), "\x1b[") {
			t.Errorf("%s.ans carries no ANSI at all", base)
		}
		if strings.Contains(string(plain), "\x1b[") {
			t.Errorf("%s.txt still carries ANSI escapes", base)
		}
	}
	// A blank directory is how a sysop turns the whole set off.
	if errs := WriteBulletins(w, ""); errs != nil {
		t.Errorf("a blank directory should write nothing, got %v", errs)
	}
}

// The world report is the league's wars. It shows attacks and nothing else --
// a weapon landing on a city never enters the log it draws from, which is what
// keeps it out rather than a filter here that could be forgotten.
func TestWorldReportShowsAttacksAndNamesTheOutcome(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 2
	cfg.BoardID = "Alpha BBS"
	cfg.IBBS = true // the World Report is the league's, so only a league board writes one
	w := game.NewWorldSeed(cfg, 1)
	w.Battles = []game.BattleLogEntry{
		{Date: "2026-08-27", Attacker: "Apples", Defender: "Bananas", Won: true, Land: 12},
		{Date: "2026-08-27", Planet: "Delta BBS", Attacker: "Cherries", Defender: "Dates", Crushed: true, Won: true},
		{Date: "2026-08-27", Planet: "Delta BBS", Attacker: "Elderberry", Defender: "Figs"},
	}
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	body, err := os.ReadFile(filepath.Join(dir, "world.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"Apples", "Bananas", "took 12", "CRUSHED", "Delta BBS", "held"} {
		if !strings.Contains(text, want) {
			t.Errorf("the world report does not mention %q:\n%s", want, text)
		}
	}
	// Newest first: a reader scanning the top wants today's fighting.
	if strings.Index(text, "Elderberry") > strings.Index(text, "Apples") {
		t.Error("the report is oldest-first; the newest battle should lead")
	}
	// Colour never carries the meaning alone, so the plain file says as much as
	// the coloured one.
	if !strings.Contains(text, "CRUSHED") || !strings.Contains(text, "held") {
		t.Error("the plain report lost an outcome that only colour distinguished")
	}
}

// A board playing alone has no world to report on, so it writes no World
// Report -- a page under that heading listing one planet's skirmishes would
// promise something the board does not have.
func TestStandAloneBoardWritesNoWorldReport(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BoardID = "Solo BBS"
	cfg.IBBS = false
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-08-27"
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, gone := range []string{"world.ans", "world.txt"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("a stand-alone board wrote %s", gone)
		}
	}
	// It still gets the rest: its scoreboard and news are as worth showing as
	// any league board's.
	for _, want := range []string{"scores.ans", "scores.txt", "tdynews.ans", "tdynews.txt", "yesnews.ans"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("a stand-alone board did not write %s: %v", want, err)
		}
	}
}
