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
		page, err := os.ReadFile(filepath.Join(dir, base+".html"))
		if err != nil {
			t.Fatalf("%s.html: %v", base, err)
		}
		if !strings.Contains(string(page), "<pre") {
			t.Errorf("%s.html is not the screen", base)
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

// The pages show the SAME screen the .ans does, coloured (#233), so a bulletin
// linked from a website carries the game's look rather than arriving as a
// generic web table.
func TestBulletinPagesShowTheColouredScreen(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BoardID = "Alpha BBS"
	cfg.IBBS = true
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
	page, err := os.ReadFile(filepath.Join(dir, "world.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"<pre", "World Report", "Apples", "Bananas", "took 12", "CRUSHED", "Delta BBS"} {
		if !strings.Contains(html, want) {
			t.Errorf("world.html lacks %q", want)
		}
	}
	// Coloured, not a monochrome dump: the outcome words carry colour on screen
	// and must carry it here.
	if !strings.Contains(html, "<span style=\"color:#") {
		t.Error("world.html carries no colour at all")
	}
	// And no raw escapes survived the conversion.
	if strings.Contains(html, "\x1b") || strings.Contains(html, "\u001b") {
		t.Error("world.html still contains ANSI escape bytes")
	}
	// Self-contained: a sysop drops it where their web server already looks.
	// A LINK is not a fetch -- the footer points at the game's own page, which
	// a reader who arrived from a BBS bulletin may well want -- so this forbids
	// resources the browser would load, not anchors.
	for _, fetched := range []string{"<script", "<link rel=\"stylesheet\"", "<img", "@import", "url("} {
		if strings.Contains(html, fetched) {
			t.Errorf("world.html loads something external: %q", fetched)
		}
	}
	// The board's own name belongs on the page when it has one: a bulletin
	// linked from a website should say whose board it came from.
	if !strings.Contains(html, "Alpha BBS") {
		t.Error("world.html does not name the board it came from")
	}
	if !strings.Contains(html, `<a href="https://andy5995.github.io/immortal-barons/"`) {
		t.Error("world.html has no link to the game's page")
	}
	// A realm name is player-supplied and reaches the page inside template.HTML,
	// which does NOT escape -- so ansiToHTML must, and this is where that is
	// proved.
	scores, err := os.ReadFile(filepath.Join(dir, "scores.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(scores), "<script>alert") {
		t.Error("a realm name was written into the page unescaped")
	}
	if !strings.Contains(string(scores), "&lt;script&gt;") {
		t.Error("the escaped realm name is missing, so the row was not rendered at all")
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
	for _, gone := range []string{"world.ans", "world.txt", "world.html"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("a stand-alone board wrote %s", gone)
		}
	}
	// It still gets the rest: its scoreboard and news are as worth showing as
	// any league board's.
	for _, want := range []string{"scores.ans", "scores.txt", "scores.html", "tdynews.ans", "yesnews.html"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("a stand-alone board did not write %s: %v", want, err)
		}
	}
}

// Contrast is measured, never eyeballed. The page renders the game's own
// colours on black, so every one of them is checked here: a colour that reads
// on a terminal is not automatically readable in a browser, and a later tweak
// must not quietly drop one below the threshold.
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
	for code, colour := range ansiPalette {
		if got := ratio(colour, bulletinBG); got < 4.5 {
			t.Errorf("SGR %d (%s) is %.2f:1 against %s, want 4.5:1 for text",
				code, colour, got, bulletinBG)
		}
	}
	// The link and the focus ring are the page's own colours rather than the
	// screen's, so they are held to the same bar: 4.5:1 for the link text, 3:1
	// for the ring as a non-text indicator.
	if got := ratio("#79aaff", bulletinBG); got < 4.5 {
		t.Errorf("the footer link is %.2f:1, want 4.5:1", got)
	}
	if got := ratio("#ffd166", bulletinBG); got < 3 {
		t.Errorf("the focus ring is %.2f:1, want 3:1", got)
	}
}

// The board's own site, from bbs.cfg, so a reader who lands on a bulletin can
// get back to the BBS it came from (#233). A board without one still names
// itself, in plain text.
func TestBulletinPagesLinkTheBoardsOwnSite(t *testing.T) {
	build := func(url string) string {
		cfg := game.DefaultConfig()
		cfg.AICount = 1
		cfg.BoardID = "Alpha BBS"
		cfg.BoardURL = url
		w := game.NewWorldSeed(cfg, 1)
		w.Today = "2026-08-27"
		dir := t.TempDir()
		if errs := WriteBulletins(w, dir); len(errs) != 0 {
			t.Fatal(errs)
		}
		body, err := os.ReadFile(filepath.Join(dir, "scores.html"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	linked := build("https://alpha.example/bbs")
	if !strings.Contains(linked, `<a href="https://alpha.example/bbs">Alpha BBS</a>`) {
		t.Errorf("the board name is not linked to its site:\n%s", linked[strings.Index(linked, "<footer"):])
	}
	plain := build("")
	if strings.Contains(plain, "<a href=\"\"") {
		t.Error("a board with no site got an empty link")
	}
	if !strings.Contains(plain, "Alpha BBS") {
		t.Error("a board with no site lost its name entirely")
	}
	// A sysop's typo should not become a scripted link on a page they publish.
	// html/template's URL context is what stops it, and this proves it is in
	// force rather than assumed.
	hostile := build("javascript:alert(1)")
	if strings.Contains(hostile, "javascript:alert") {
		t.Error("a javascript: URL reached the page's href")
	}
}

// BBSName is the board's own name, which is not always its planet name, and
// the game has no way to ask the BBS for it. Set, the pages use it; absent,
// they fall back to the planet rather than going nameless (#233).
func TestBulletinPagesPreferTheBBSName(t *testing.T) {
	page := func(bbsName string) string {
		cfg := game.DefaultConfig()
		cfg.AICount = 1
		cfg.BoardID = "Alpha BBS"
		cfg.BBSName = bbsName
		w := game.NewWorldSeed(cfg, 1)
		w.Today = "2026-08-27"
		dir := t.TempDir()
		if errs := WriteBulletins(w, dir); len(errs) != 0 {
			t.Fatal(errs)
		}
		body, err := os.ReadFile(filepath.Join(dir, "scores.html"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	named := page("The Alpha Board")
	if !strings.Contains(named, "The Alpha Board") {
		t.Error("the BBS name was set and not used")
	}
	fallback := page("")
	if !strings.Contains(fallback, "Alpha BBS") {
		t.Error("with no BBS name the page did not fall back to the planet name")
	}
}
