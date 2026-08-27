package menu

import (
	"fmt"
	"math"
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

// The HTML pages are built from the game's data, not translated from the ANSI
// screen (#233). This checks they carry the same facts and escape what a realm
// name could otherwise inject.
func TestBulletinPagesCarryTheDataAndEscapeIt(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BoardID = "Alpha BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-08-27"
	w.AddHuman("scripty", `<script>alert(1)</script>`)
	w.Battles = []game.BattleLogEntry{
		{Attacker: "Apples", Defender: "Bananas", Won: true, Land: 12},
		{Planet: "Delta BBS", Attacker: "Cherries", Defender: "Dates", Won: true, Crushed: true},
	}
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	world, err := os.ReadFile(filepath.Join(dir, "world.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(world)
	for _, want := range []string{"<table", "World Report", "Apples", "Bananas", "took 12", "CRUSHED", "Delta BBS"} {
		if !strings.Contains(page, want) {
			t.Errorf("world.html lacks %q", want)
		}
	}
	// A real table, not a <pre> of the terminal screen: that is the whole point
	// of rendering from data.
	if strings.Contains(page, "<pre") || strings.Contains(page, "\x1b[") {
		t.Error("world.html carries the terminal screen rather than a table")
	}
	// It stands alone: a sysop drops it where their web server already looks.
	for _, external := range []string{"<script src", "<link rel=\"stylesheet\"", "http://", "https://"} {
		if strings.Contains(page, external) {
			t.Errorf("world.html fetches something external: %q", external)
		}
	}
	scores, err := os.ReadFile(filepath.Join(dir, "scores.html"))
	if err != nil {
		t.Fatal(err)
	}
	// A realm name is player-supplied and reaches this page; it must arrive as
	// text, not markup.
	if strings.Contains(string(scores), "<script>alert") {
		t.Error("a realm name was written into the page unescaped")
	}
	if !strings.Contains(string(scores), "&lt;script&gt;") {
		t.Error("the escaped realm name is missing, so the row was not rendered at all")
	}
}

// Contrast is measured, never eyeballed: the palette is checked here so a later
// colour tweak cannot quietly drop a value below the threshold. WCAG 2.1 wants
// 4.5:1 for body text and 3:1 for a non-text boundary, in BOTH themes.
func TestBulletinPaletteMeetsContrast(t *testing.T) {
	lum := func(hex string) float64 {
		var r, g, b int
		fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
		f := func(v int) float64 {
			c := float64(v) / 255
			if c <= 0.03928 {
				return c / 12.92
			}
			return math.Pow((c+0.055)/1.055, 2.4)
		}
		return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
	}
	ratio := func(a, b string) float64 {
		la, lb := lum(a), lum(b)
		if la < lb {
			la, lb = lb, la
		}
		return (la + 0.05) / (lb + 0.05)
	}
	for _, theme := range []struct {
		name, bg string
		text     map[string]string
		rule     string
	}{
		{"light", "#ffffff", map[string]string{
			"text": "#1f2328", "muted": "#59636e", "accent": "#7d4e00",
			"win": "#1a7f37", "loss": "#cf222e", "planet": "#0550ae",
		}, "#818b95"},
		{"dark", "#0d1117", map[string]string{
			"text": "#e6edf3", "muted": "#9198a1", "accent": "#d29922",
			"win": "#3fb950", "loss": "#f85149", "planet": "#79c0ff",
		}, "#7d8590"},
	} {
		for token, colour := range theme.text {
			if got := ratio(colour, theme.bg); got < 4.5 {
				t.Errorf("%s theme: %s (%s) is %.2f:1 against %s, want 4.5:1 for text",
					theme.name, token, colour, got, theme.bg)
			}
			if !strings.Contains(bulletinCSS, colour) {
				t.Errorf("%s theme: %s (%s) is checked here but not in the stylesheet", theme.name, token, colour)
			}
		}
		if got := ratio(theme.rule, theme.bg); got < 3 {
			t.Errorf("%s theme: rule (%s) is %.2f:1, want 3:1 for a non-text boundary",
				theme.name, theme.rule, got)
		}
	}
}
