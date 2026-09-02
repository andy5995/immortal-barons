package menu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The colours reach the page as class names, never as inline styles: a class is
// what a replacement stylesheet can restyle, and those names are the whole
// contract between a generated page and the sheet painting it (#245).
func TestANSIToHTMLCarriesColoursAsClasses(t *testing.T) {
	in := "\x1b[97mBright\x1b[0m plain \x1b[31mred\x1b[0m\n"
	got := ansiToHTML([]byte(in), "Immortal Barons")
	for _, want := range []string{
		`<pre class="ansi">`,
		`<span class="ansi-fg-15">Bright</span>`,
		` plain `,
		`<span class="ansi-fg-1">red</span>`,
		"</pre>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the block is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "style=") {
		t.Errorf("a colour was written as an inline style, which cannot be restyled:\n%s", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("an escape survived into the page:\n%s", got)
	}
}

// Bold brightens a dark foreground, the convention every terminal follows and
// the one BRE's own 1;30 gray relies on. A background reaches its own class.
func TestANSIToHTMLBoldAndBackground(t *testing.T) {
	if got := ansiToHTML([]byte("\x1b[1;30mgray"), "Immortal Barons"); !strings.Contains(got, `class="ansi-fg-8"`) {
		t.Errorf("bold did not brighten the foreground:\n%s", got)
	}
	if got := ansiToHTML([]byte("\x1b[44;37mon blue"), "Immortal Barons"); !strings.Contains(got, `class="ansi-fg-7 ansi-bg-4"`) {
		t.Errorf("the background did not reach the page:\n%s", got)
	}
	// Reverse video swaps the pair, so a reversed cell shows what a terminal
	// shows rather than nothing at all.
	if got := ansiToHTML([]byte("\x1b[7mhighlight"), "Immortal Barons"); !strings.Contains(got, `class="ansi-fg-0 ansi-bg-7"`) {
		t.Errorf("reverse video did not swap the colours:\n%s", got)
	}
}

// Everything that is not a colour is dropped: a page has no cursor to move and
// no wrap to toggle, and a stray escape byte on screen is the visible failure.
func TestANSIToHTMLDropsNonColourEscapes(t *testing.T) {
	got := ansiToHTML([]byte("\x1b[2J\x1b[H\x1b[?7lclean\x1b[?7h\x1b[2K"), "Immortal Barons")
	if !strings.Contains(got, ">clean<") && !strings.Contains(got, "\">clean") && !strings.Contains(got, "clean") {
		t.Fatalf("the text was lost:\n%s", got)
	}
	for _, gone := range []string{"2J", "?7l", "2K", "\x1b"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survived onto the page:\n%s", gone, got)
		}
	}
	// A two-byte escape (ESC c and the like) is measured so it can be dropped
	// whole; a trailing ESC with nothing after it costs its own byte and no
	// more, rather than swallowing the rest of the file.
	if got := ansiToHTML([]byte("a\x1bcb"), "Immortal Barons"); !strings.Contains(got, "ab") {
		t.Errorf("a two-byte escape was not dropped cleanly: %q", got)
	}
	if got := ansiToHTML([]byte("trailing\x1b"), "Immortal Barons"); !strings.Contains(got, "trailing") || strings.Contains(got, "\x1b") {
		t.Errorf("a trailing ESC was mishandled: %q", got)
	}
}

// Player-controlled text reaches the page as text, never as markup.
func TestANSIToHTMLEscapesTheContent(t *testing.T) {
	got := ansiToHTML([]byte("<script>alert(1)</script> & \"quoted\""), "Immortal Barons")
	if strings.Contains(got, "<script>") {
		t.Fatalf("a tag in the content reached the page unescaped:\n%s", got)
	}
	for _, want := range []string{"&lt;script&gt;", "&amp;"} {
		if !strings.Contains(got, want) {
			t.Errorf("the block is missing %q:\n%s", want, got)
		}
	}
}

// Two pages per bulletin: the bare block to include in a page you already
// build, and the whole page to link. Boards do both things with a scoreboard.
func TestBulletinsWriteBothWebForms(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 2
	cfg.BoardID = "Alpha BBS"
	cfg.IBBS = true
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, base := range []string{"scores", "tdynews", "yesnews", "world", "bbsscore", "plywland"} {
		block, err := os.ReadFile(filepath.Join(dir, base+".inc.html"))
		if err != nil {
			t.Fatalf("%s.inc.html: %v", base, err)
		}
		page, err := os.ReadFile(filepath.Join(dir, base+".html"))
		if err != nil {
			t.Fatalf("%s.html: %v", base, err)
		}
		if !strings.HasPrefix(string(block), `<pre class="ansi">`) {
			t.Errorf("%s.inc.html is not a bare block: %.60s", base, block)
		}
		if strings.Contains(string(block), "<html") {
			t.Errorf("%s.inc.html carries a whole page, so it cannot be included", base)
		}
		if !strings.Contains(string(page), "<!DOCTYPE html>") {
			t.Errorf("%s.html is not a whole page", base)
		}
		// The page is the block with the wrappers round it, not a second render.
		if !strings.Contains(string(page), string(block)) {
			t.Errorf("%s.html does not carry the same block as %s.inc.html", base, base)
		}
	}
	// The bulletin's own name fills the title token, so a page is identifiable
	// in a browser tab and in a list of links.
	page, err := os.ReadFile(filepath.Join(dir, "bbswland.html"))
	if err != nil {
		t.Fatal(err)
	}
	// The title names the bulletin, the board and the game, in that order.
	if !strings.Contains(string(page), "<title>Top Planets by Net Worth Density | Immortal Barons | Alpha BBS</title>") {
		t.Errorf("bbswland.html is not titled by its ranking, board and game:\n%.400s", page)
	}
	for _, token := range []string{titleToken, bbsToken, gameToken} {
		if strings.Contains(string(page), token) {
			t.Errorf("the %s token was left unfilled in a page", token)
		}
	}
}

// The three files a sysop owns are written once and then never touched again:
// overwriting one would throw away the nav, the back-link and the palette it
// was edited to carry, which is why the old compiled-in template was removed.
func TestBulletinTemplatesAreWrittenOnceAndNotOverwritten(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BoardID = "Solo BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, name := range []string{"header.html", "footer.html", "bulletin.css"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
	}
	mine := "<!DOCTYPE html>\n<html><head><title>{{title}}</title></head><body><nav>mine</nav>\n"
	if err := os.WriteFile(filepath.Join(dir, "header.html"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bulletin.css"), []byte("/* mine */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	for name, want := range map[string]string{"header.html": mine, "bulletin.css": "/* mine */\n"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Errorf("%s was overwritten:\n%s", name, body)
		}
	}
	// The edited header is what wraps the next run's pages.
	page, err := os.ReadFile(filepath.Join(dir, "scores.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<nav>mine</nav>") {
		t.Errorf("the page did not use the edited header:\n%.400s", page)
	}
	if !strings.Contains(string(page), "<title>Scoreboard</title>") {
		t.Errorf("the edited header's title token was not filled:\n%.400s", page)
	}
	// A deleted template comes back, which is how a sysop gets a fresh copy.
	if err := os.Remove(filepath.Join(dir, "footer.html")); err != nil {
		t.Fatal(err)
	}
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "footer.html")); err != nil {
		t.Errorf("a deleted template did not come back: %v", err)
	}
}

// The game's own name is a hyperlink to its site. A bulletin reaches readers on
// a board's website who have never heard of the game, and the name is already
// at the top of every screen (#245).
func TestGameNameLinksToItsSite(t *testing.T) {
	got := ansiToHTML([]byte("\x1b[97mImmortal Barons\x1b[0m: rankings"), "Immortal Barons")
	want := `<a href="https://andy5995.github.io/immortal-barons/">Immortal Barons</a>`
	if !strings.Contains(got, want) {
		t.Errorf("the game name is not linked:\n%s", got)
	}
	// The link sits INSIDE the colour span, so it keeps the colour the screen
	// drew the name in rather than repainting a heading.
	if !strings.Contains(got, `<span class="ansi-fg-15"><a href=`) {
		t.Errorf("the link was written outside its colour span:\n%s", got)
	}
	// Nothing else on the page becomes a link.
	if strings.Count(got, "<a href=") != 1 {
		t.Errorf("expected exactly one link:\n%s", got)
	}
	if got := ansiToHTML([]byte("no name here"), "Immortal Barons"); strings.Contains(got, "<a ") {
		t.Errorf("a screen without the name got a link anyway:\n%s", got)
	}
}

// Every bulletin draws the name, so every page carries the link.
func TestEveryBulletinPageLinksTheGameName(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 2
	cfg.BoardID = "Alpha BBS"
	cfg.IBBS = true
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, base := range []string{"scores", "tdynews", "yesnews", "world", "bbsscore", "plywland"} {
		body, err := os.ReadFile(filepath.Join(dir, base+".inc.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `<a href="https://andy5995.github.io/immortal-barons/">`) {
			t.Errorf("%s.inc.html does not link the game name:\n%s", base, body)
		}
	}
	// The .ans and .txt beside them are unchanged: a terminal has no link.
	ans, err := os.ReadFile(filepath.Join(dir, "scores.ans"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ans), "andy5995.github.io") {
		t.Error("the URL leaked into the ANSI file")
	}
}

// A page title names the bulletin, the board and the game (#245). The board's
// name comes from BBSName, which is separate from BoardID because BoardID has
// to match the league roster character for character and a name written for a
// reader should not be pinned to that.
func TestPageTitleNamesTheBoard(t *testing.T) {
	title := func(t *testing.T, w *game.World) string {
		t.Helper()
		dir := t.TempDir()
		if errs := WriteBulletins(w, dir); len(errs) != 0 {
			t.Fatal(errs)
		}
		page, err := os.ReadFile(filepath.Join(dir, "scores.html"))
		if err != nil {
			t.Fatal(err)
		}
		open := strings.Index(string(page), "<title>")
		shut := strings.Index(string(page), "</title>")
		if open < 0 || shut < open {
			t.Fatalf("no title in the page:\n%.400s", page)
		}
		return string(page)[open+len("<title>") : shut]
	}

	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BoardID = "Avalon"
	cfg.BBSName = "The Dog House BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got, want := title(t, w), "Scoreboard | Immortal Barons | The Dog House BBS"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}

	// Left blank it falls back to BoardID, which for most boards is the same
	// word anyway.
	cfg.BBSName = ""
	w = game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got, want := title(t, w), "Scoreboard | Immortal Barons | Avalon"; got != want {
		t.Errorf("title with no BBSName = %q, want %q", got, want)
	}

	// A board that has set neither leaves no gap behind: the separators close
	// up rather than reading "Scoreboard | Immortal Barons | ".
	cfg.BoardID = ""
	w = game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got, want := title(t, w), "Scoreboard | Immortal Barons"; got != want {
		t.Errorf("title with no board name at all = %q, want %q", got, want)
	}
}

// The link in the block carries no underline: it keeps the colour the screen
// drew the name in, and a highlight bar marks it, the way a DOS menu marks the
// line under the cursor. The bar is what a reader sees without hovering, so the
// link is not distinguished by colour alone.
func TestBlockLinkIsMarkedWithoutAnUnderline(t *testing.T) {
	dir := t.TempDir()
	if errs := writeBulletinTemplates(dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	css, err := os.ReadFile(filepath.Join(dir, "bulletin.css"))
	if err != nil {
		t.Fatal(err)
	}
	block := string(css)[strings.Index(string(css), "pre.ansi a {"):]
	block = block[:strings.Index(block, "}")]
	if !strings.Contains(block, "text-decoration: none") {
		t.Errorf("the link is underlined:\n%s", block)
	}
	if !strings.Contains(block, "background:") {
		t.Errorf("nothing marks the link without a hover:\n%s", block)
	}
	// Motion is decoration and has to be optional.
	if !strings.Contains(string(css), "prefers-reduced-motion") {
		t.Error("the stylesheet does not honour prefers-reduced-motion")
	}
}

// The board's name links back to the board's own site when bbs.cfg names one,
// and is plain text when it does not -- a link to nowhere would land the reader
// back on the page they are already reading (#245).
func TestBoardNameLinksToTheBoardsSite(t *testing.T) {
	board := func(t *testing.T, w *game.World) string {
		t.Helper()
		dir := t.TempDir()
		if errs := WriteBulletins(w, dir); len(errs) != 0 {
			t.Fatal(errs)
		}
		page, err := os.ReadFile(filepath.Join(dir, "scores.html"))
		if err != nil {
			t.Fatal(err)
		}
		open := strings.Index(string(page), `<p class="board">`)
		if open < 0 {
			return ""
		}
		shut := strings.Index(string(page)[open:], "</p>")
		return string(page)[open+len(`<p class="board">`) : open+shut]
	}

	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BBSName = "The Dog House BBS"
	cfg.BoardURL = "https://doghouse.example/"
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got, want := board(t, w), `<a href="https://doghouse.example/">The Dog House BBS</a>`; got != want {
		t.Errorf("board line = %q, want %q", got, want)
	}

	cfg.BoardURL = ""
	w = game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got, want := board(t, w), "The Dog House BBS"; got != want {
		t.Errorf("board line with no URL = %q, want %q", got, want)
	}

	// No name and no URL leaves an empty element, which the stylesheet hides
	// rather than printing a blank line above every bulletin.
	cfg.BBSName, cfg.BoardID = "", ""
	w = game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	if got := board(t, w); got != "" {
		t.Errorf("board line with nothing set = %q, want empty", got)
	}
	dir := t.TempDir()
	if errs := writeBulletinTemplates(dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	css, err := os.ReadFile(filepath.Join(dir, "bulletin.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), ".board:empty { display: none }") {
		t.Error("the stylesheet does not hide an empty board line")
	}
}

// A nav a sysop builds in the header survives the empty-link cleanup: only an
// anchor whose href or text came out blank is touched.
func TestTemplateCleanupLeavesARealNavAlone(t *testing.T) {
	dir := t.TempDir()
	nav := `<!DOCTYPE html><html><head><title>{{title}} | {{bbs}}</title></head><body>` +
		`<nav><a href="/">Home</a> | <a href="/bbs/">The Board</a></nav>` +
		`<p class="board"><a href="{{boardurl}}">{{bbs}}</a></p>`
	if err := os.WriteFile(filepath.Join(dir, "header.html"), []byte(nav), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readTemplate(dir, "header.html", pageNames{title: "Scoreboard", bbs: "Avalon"})
	for _, want := range []string{`<a href="/">Home</a>`, `<a href="/bbs/">The Board</a>`} {
		if !strings.Contains(got, want) {
			t.Errorf("the cleanup ate a real link, %q:\n%s", want, got)
		}
	}
	// The pipes in the nav are the body's, not the title's, so the title tidy
	// must not have reached them.
	if !strings.Contains(got, "</a> | <a") {
		t.Errorf("the separator in the nav was collapsed:\n%s", got)
	}
	if !strings.Contains(got, `<p class="board">Avalon</p>`) {
		t.Errorf("the board name was not unlinked with no URL set:\n%s", got)
	}
}

// A header can carry its page's own address and date, which is what meta tags
// like og:url, rel=canonical and og:updated_time need (#245). The address comes
// from BulletinURL, the one thing the game cannot work out for itself.
func TestHeaderCanCarryPageURLAndDate(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.BBSName = "The Dog House BBS"
	cfg.BulletinURL = "https://doghouse.example/bulletins/"
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-09-02"
	dir := t.TempDir()
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	header := `<!DOCTYPE html><html><head><title>{{title}}</title>` +
		`<link rel="canonical" href="{{pageurl}}">` +
		`<meta property="og:url" content="{{pageurl}}">` +
		`<meta property="og:updated_time" content="{{date}}">` +
		`</head><body>`
	if err := os.WriteFile(filepath.Join(dir, "header.html"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := WriteBulletins(w, dir); len(errs) != 0 {
		t.Fatal(errs)
	}
	page, err := os.ReadFile(filepath.Join(dir, "scores.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="canonical" href="https://doghouse.example/bulletins/scores.html">`,
		`<meta property="og:url" content="https://doghouse.example/bulletins/scores.html">`,
		`<meta property="og:updated_time" content="2026-09-02">`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the page is missing %q:\n%.500s", want, page)
		}
	}
	// A trailing slash on the setting or none produces the same address.
	if got := bulletinPageURL("https://x.example/b", "world"); got != "https://x.example/b/world.html" {
		t.Errorf("page URL = %q", got)
	}
	if got := bulletinPageURL("", "world"); got != "" {
		t.Errorf("page URL with no BulletinURL = %q, want empty", got)
	}
}

// A board that has not said where its bulletins are served from gets no
// canonical link rather than one pointing at nothing: search engines read an
// empty canonical as a claim, not as a gap.
func TestEmptyMetaAndLinkTagsAreDropped(t *testing.T) {
	dir := t.TempDir()
	header := `<head><meta charset="utf-8">` + "\n" +
		`<link rel="canonical" href="{{pageurl}}">` + "\n" +
		`<meta property="og:url" content="{{pageurl}}">` + "\n" +
		`<meta property="og:site_name" content="{{bbs}}">` + "\n" + `</head>`
	if err := os.WriteFile(filepath.Join(dir, "header.html"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readTemplate(dir, "header.html", pageNames{title: "Scoreboard", bbs: "Avalon"})
	for _, gone := range []string{`href=""`, `content=""`, "canonical", "og:url"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survived with nothing to fill it:\n%s", gone, got)
		}
	}
	// The tags that DID fill in are untouched, charset included.
	for _, want := range []string{`<meta charset="utf-8">`, `<meta property="og:site_name" content="Avalon">`} {
		if !strings.Contains(got, want) {
			t.Errorf("the cleanup removed %q:\n%s", want, got)
		}
	}
}
