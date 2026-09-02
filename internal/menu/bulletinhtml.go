package menu

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bulletinhtml.go — the web form of each bulletin file (#245).
//
// IB carried HTML generation once and it was taken out, because the page was a
// template compiled into the binary and a sysop could not restyle it. The
// templates here are written to the bulletin directory instead, and only when
// they are absent, so an edited one survives every later run. Nothing in the
// generated page styles itself: the colours arrive as class names, and the
// stylesheet holding them is a starting point a sysop may replace outright.
//
// Two pages per bulletin, because boards do both things with a scoreboard:
//
//	scores.html      — the whole page, header and footer wrapped round it, to link
//	scores.inc.html  — the block on its own, to include in a page you already build
//
// That is the same shape as the .ans and .txt pair. A board that wants both is
// not asked to choose.

// bulletinTemplates are the three files a sysop owns. They are written once, on
// the first run that finds them missing, and never again -- overwriting one
// would throw away the nav, the back-link and the palette they were edited to
// carry, which is the whole reason the old compiled-in template was removed.
var bulletinTemplates = map[string]string{
	"header.html":  defaultHeaderHTML,
	"footer.html":  defaultFooterHTML,
	"bulletin.css": defaultBulletinCSS,
}

// The substitutions the header and footer understand. A sysop who does not want
// one deletes it; the page is then the same for every bulletin, or carries no
// board name, which is their call to make.
//
//	{{title}}    — the bulletin's own name, "Scoreboard" or "Top Planets by Land"
//	{{bbs}}      — what the board calls itself (BBSName in bbs.cfg, else BoardID)
//	{{boardurl}} — the board's own website (BoardURL in bbs.cfg)
//	{{pageurl}}  — this page's own address, for a canonical link or an og:url
//	               (BulletinURL in bbs.cfg, plus the page's file name)
//	{{date}}     — the game day this bulletin was written for
//	{{game}}     — the game's name
//
// A tag whose token fills in empty is taken out rather than left blank: a board
// that has set no BoardURL should read as plain text rather than link to the
// page it is already on, and a canonical link to nowhere is worse than none.
// That is nicer for a sysop than a rule about which tokens need which settings.
const (
	titleToken    = "{{title}}"
	bbsToken      = "{{bbs}}"
	boardURLToken = "{{boardurl}}"
	pageURLToken  = "{{pageurl}}"
	dateToken     = "{{date}}"
	gameToken     = "{{game}}"
)

const gameSiteURL = "https://andy5995.github.io/immortal-barons/"

const defaultHeaderHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{title}} | {{game}} | {{bbs}}</title>
<link rel="stylesheet" href="bulletin.css">
</head>
<body>
<header class="masthead">
<p class="board"><a href="{{boardurl}}">{{bbs}}</a></p>
<h1>{{title}}</h1>
</header>
`

const defaultFooterHTML = `</body>
</html>
`

// defaultBulletinCSS paints the sixteen ANSI colours and the small amount of
// page around them. The only thing that names it is the <link> in the default
// header, so pointing that at your own sheet is a one-line edit and this file
// then goes unused.
//
// The page is the terminal rather than a web page holding a picture of one:
// black to the edges, one monospace face throughout, and a measure of 80
// columns because that is the width every screen in the game was drawn to. So
// hierarchy is carried by weight and colour out of the sixteen, not by a second
// typeface, and the block needs no frame to separate it from its surroundings.
//
// Contrast, measured against #000. The eight bright colours and white all clear
// 4.5:1 (bright red 5.3, bright green 15.3, bright yellow 19.6, white 21), and
// so does light grey #AAAAAA at 9.0. The seven remaining dark ones do not, and
// cannot: dark blue #0000AA is 1.6:1 and dark red #AA0000 is 2.7:1. That is a
// property of the CGA palette rather than of this sheet, and repainting them
// would stop the art being the art. What saves the page is where IB puts them
// -- every figure and heading is on a bright colour or white, and the dark half
// draws parentheses and banner dashes. A reader who has asked their system for
// more contrast gets the dark half lifted anyway, in the block at the end.
//
// ONE deviation from CGA, and it is deliberate: colour 8 is #757575 here rather
// than #555555, which is 1.7:1 and unreadable. It is the colour IB rules and
// column separators are drawn in, so it carries structure on every bulletin,
// and #757575 is the least change that reaches 4.6:1. Terminals do not agree on
// this entry anyway. The other fifteen are the CGA values exactly.
const defaultBulletinCSS = `:root {
  --ground: #000000;
  --body: #aaaaaa;       /* CGA light grey, 9.0:1 */
  --bright: #ffffff;     /* 21:1 */
  --rule: #757575;       /* 4.6:1 */
  --accent: #55ffff;     /* CGA bright cyan, 16.0:1 */
  --measure: 80ch;
  --mono: "DejaVu Sans Mono", "Cascadia Mono", Menlo, Consolas, monospace;
}

* { box-sizing: border-box }

body {
  margin: 0;
  padding: 2rem 1.5rem 3rem;
  background: var(--ground);
  color: var(--body);
  font-family: var(--mono);
  font-size: 0.875rem;
  line-height: 1.4;
}

.masthead,
pre.ansi {
  max-width: var(--measure);
  margin-inline: auto;
}

/* The board's own name, set as a status line above the bulletin: small, wide,
   and quiet, so the bulletin's name is the thing read first. */
.masthead .board {
  margin: 0 0 0.25rem;
  color: var(--rule);
  font-size: 0.75rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}
.masthead .board:empty { display: none }
/* The board's name links back to the board's own site when bbs.cfg names one.
   Ordinary chrome, so it takes an ordinary hover rather than the highlight bar
   the bulletin's own link carries. */
.masthead .board a { color: inherit; text-decoration: none }
.masthead .board a:hover,
.masthead .board a:focus-visible { color: var(--accent); text-decoration: underline }

.masthead h1 {
  margin: 0 0 1.25rem;
  color: var(--bright);
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}

/* The bulletin. No frame, no rules and no ground of its own: the page is
   already the terminal it was drawn for, and the screen inside draws its own
   heading and its own rules.
   margin-inline stays auto here -- a shorthand "margin: 0" would cancel the
   centring set above and leave the block against the left edge while the
   heading over it stayed centred.
   overflow-y is named explicitly because setting overflow-x alone makes the
   other axis compute to auto, and the browser then paints a vertical
   scrollbar track down the side of a block that never scrolls. */
pre.ansi {
  margin-block: 0;
  overflow-x: auto;
  overflow-y: hidden;
  line-height: 1.2;
  white-space: pre;
}

/* The game's name links to its site wherever a screen draws it. It carries no
   underline and keeps whatever colour the screen gave it, so it does not
   repaint a heading; hovering lights a bar behind it in that same colour, the
   way a DOS menu highlights the line under the cursor. */
pre.ansi a {
  color: inherit;
  text-decoration: none;
  background: rgba(255, 255, 255, 0.18);
  background: color-mix(in srgb, currentColor 20%, transparent);
  transition: background-color 120ms ease-out;
}
pre.ansi a:hover,
pre.ansi a:focus-visible {
  background: rgba(255, 255, 255, 0.42);
  background: color-mix(in srgb, currentColor 45%, transparent);
}
a:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px }

@media (prefers-reduced-motion: reduce) {
  pre.ansi a { transition: none }
}

.ansi-fg-0  { color: #000000 }
.ansi-fg-1  { color: #aa0000 }
.ansi-fg-2  { color: #00aa00 }
.ansi-fg-3  { color: #aa5500 }
.ansi-fg-4  { color: #0000aa }
.ansi-fg-5  { color: #aa00aa }
.ansi-fg-6  { color: #00aaaa }
.ansi-fg-7  { color: #aaaaaa }
.ansi-fg-8  { color: #757575 } /* lifted from CGA #555555; see above */
.ansi-fg-9  { color: #ff5555 }
.ansi-fg-10 { color: #55ff55 }
.ansi-fg-11 { color: #ffff55 }
.ansi-fg-12 { color: #5555ff }
.ansi-fg-13 { color: #ff55ff }
.ansi-fg-14 { color: #55ffff }
.ansi-fg-15 { color: #ffffff }

.ansi-bg-0  { background: #000000 }
.ansi-bg-1  { background: #aa0000 }
.ansi-bg-2  { background: #00aa00 }
.ansi-bg-3  { background: #aa5500 }
.ansi-bg-4  { background: #0000aa }
.ansi-bg-5  { background: #aa00aa }
.ansi-bg-6  { background: #00aaaa }
.ansi-bg-7  { background: #aaaaaa }
.ansi-bg-8  { background: #555555 }
.ansi-bg-9  { background: #ff5555 }
.ansi-bg-10 { background: #55ff55 }
.ansi-bg-11 { background: #ffff55 }
.ansi-bg-12 { background: #5555ff }
.ansi-bg-13 { background: #ff55ff }
.ansi-bg-14 { background: #55ffff }
.ansi-bg-15 { background: #ffffff }

/* A reader who has asked for more contrast gets the dark half of the palette
   lifted to its bright counterpart, which clears 4.5:1 on black. The colours
   stop being the CGA ones; that is the trade the request asks for. */
@media (prefers-contrast: more) {
  pre.ansi { color: #e0e0e0 }
  .ansi-fg-0 { color: #6a6a6a }
  .ansi-fg-1 { color: #ff5555 }
  .ansi-fg-2 { color: #55ff55 }
  .ansi-fg-3 { color: #ffff55 }
  .ansi-fg-4 { color: #7d7dff }
  .ansi-fg-5 { color: #ff55ff }
  .ansi-fg-6 { color: #55ffff }
  .ansi-fg-7 { color: #ffffff }
  .ansi-fg-8 { color: #9a9a9a }
}
`

// writeBulletinTemplates puts the header, footer and stylesheet in place the
// first time each is missing. A file that is already there is left alone, so a
// sysop's edits survive; a file they delete comes back, which is how they get a
// fresh copy to start from again.
func writeBulletinTemplates(dir string) []error {
	var errs []error
	for name, body := range bulletinTemplates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// writeBulletinHTML writes the two web forms of one bulletin: the bare block to
// include, and the whole page to link.
//
// The header and footer are read from the directory on every bulletin rather
// than cached, because a sysop editing them mid-run should see the change on
// the next file rather than the next day. A missing one is treated as empty:
// the page is then the block alone, which is still valid to serve, and the
// include file is unaffected either way.
func writeBulletinHTML(dir, base string, names pageNames, rendered []byte) error {
	block := ansiToHTML(rendered, names.game)
	if err := writeFileIfChanged(filepath.Join(dir, base+".inc.html"), []byte(block)); err != nil {
		return err
	}
	head := readTemplate(dir, "header.html", names)
	foot := readTemplate(dir, "footer.html", names)
	return writeFileIfChanged(filepath.Join(dir, base+".html"), []byte(head+block+foot))
}

// pageNames are the three things a wrapper can name: this bulletin, this board,
// and the game.
type pageNames struct {
	title    string
	bbs      string
	boardURL string
	pageURL  string
	date     string
	game     string
}

// readTemplate reads one wrapper and fills its tokens in. A wrapper that cannot
// be read is empty rather than an error: a page missing its nav is worth
// serving, and failing the whole bulletin over it is not.
func readTemplate(dir, name string, names pageNames) string {
	body, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	out := strings.NewReplacer(
		titleToken, html.EscapeString(names.title),
		bbsToken, html.EscapeString(names.bbs),
		boardURLToken, html.EscapeString(names.boardURL),
		pageURLToken, html.EscapeString(names.pageURL),
		dateToken, html.EscapeString(names.date),
		gameToken, html.EscapeString(names.game),
	).Replace(string(body))
	return tidyTitle(dropEmptyMarkup(out))
}

// tidyTitle closes the gap a board with no name of its own leaves behind. The
// default header's title line is "{{title}} | {{game}} | {{bbs}}", and a board
// that has set neither BBSName nor BoardID would otherwise read
// "Scoreboard | Immortal Barons | ".
//
// The bulletin's own name leads because a browser tab truncates from the right:
// with the board's name first, every one of a board's twelve bulletin tabs
// reads the same word and none of them can be told apart, and bookmarks and
// history search have the same problem.
//
// Only the <title> element is touched, never the body. A sysop's nav may well
// separate its own links with a pipe, and a tidy pass over the whole page would
// eat those.
// dropEmptyMarkup takes out what a token left blank: an anchor is unwrapped and
// keeps its text, a <link> or <meta> is removed outright. A board with no
// BoardURL would otherwise have its name linked to the page the reader is
// already on, and one with no BulletinURL would carry a canonical link to
// nothing at all, which search engines read as a claim rather than as a gap.
//
// It is deliberately narrow: an empty href, an empty content, and an anchor
// with nothing between its tags are all things nobody writes on purpose, so
// this cannot touch a nav a sysop built. A <meta charset> has no content
// attribute and is never matched.
func dropEmptyMarkup(page string) string {
	page = emptyHrefLink.ReplaceAllString(page, "$1")
	page = emptyTextLink.ReplaceAllString(page, "")
	page = emptyHrefTag.ReplaceAllString(page, "")
	return emptyContentTag.ReplaceAllString(page, "")
}

var (
	emptyHrefLink   = regexp.MustCompile(`<a\b[^>]*\bhref=""[^>]*>([^<]*)</a>`)
	emptyTextLink   = regexp.MustCompile(`<a\b[^>]*>\s*</a>`)
	emptyHrefTag    = regexp.MustCompile(`[ \t]*<link\b[^>]*\bhref=""[^>]*>\n?`)
	emptyContentTag = regexp.MustCompile(`[ \t]*<meta\b[^>]*\bcontent=""[^>]*>\n?`)
)

func tidyTitle(page string) string {
	open := strings.Index(page, "<title>")
	if open < 0 {
		return page
	}
	start := open + len("<title>")
	shut := strings.Index(page[start:], "</title>")
	if shut < 0 {
		return page
	}
	shut += start
	var fields []string
	for _, f := range strings.Split(page[start:shut], "|") {
		if f = strings.TrimSpace(f); f != "" {
			fields = append(fields, f)
		}
	}
	return page[:start] + strings.Join(fields, " | ") + page[shut:]
}

// gameName is the name to hyperlink wherever it appears in the screen -- the
// game's own, which every bulletin already prints at the top. It is taken from
// the caller rather than written here because the screens draw it through tr(),
// so a translated bulletin links a translated name.
func ansiToHTML(rendered []byte, gameName string) string {
	link := ""
	if gameName != "" {
		link = html.EscapeString(gameName)
	}
	var out strings.Builder
	out.WriteString(`<pre class="ansi">`)
	var st sgrState
	st.fg, st.bg = -1, -1
	open := ""
	// text is written a run at a time -- everything up to the next escape --
	// rather than a rune at a time, so one span carries a whole coloured field
	// and html.EscapeString runs once over it.
	write := func(run string) {
		if run == "" {
			return
		}
		if class := st.class(); class != open {
			if open != "" {
				out.WriteString("</span>")
			}
			if class != "" {
				out.WriteString(`<span class="` + class + `">`)
			}
			open = class
		}
		escaped := html.EscapeString(run)
		// The name sits inside one colour run at every site that draws it, so
		// this finds it whole. A run that does not carry it is untouched.
		if link != "" && strings.Contains(escaped, link) {
			escaped = strings.ReplaceAll(escaped,
				link, `<a href="`+gameSiteURL+`">`+link+`</a>`)
		}
		out.WriteString(escaped)
	}
	text := string(rendered)
	for {
		i := strings.IndexByte(text, 0x1b)
		if i < 0 {
			write(text)
			break
		}
		write(text[:i])
		params, isSGR, n := scanEscape(text[i:])
		if n == 0 {
			// A lone ESC that begins nothing recognisable: drop the byte, and go
			// on from after it so an unterminated sequence cannot stall the loop.
			text = text[i+1:]
			continue
		}
		if isSGR {
			st.apply(params)
		}
		text = text[i+n:]
	}
	if open != "" {
		out.WriteString("</span>")
	}
	out.WriteString("</pre>\n")
	return out.String()
}

// sgrState is the colour the writer is currently in: the two indices into the
// sixteen-colour palette, plus the two attributes that move a colour between
// its halves.
type sgrState struct {
	fg, bg  int // -1 is the terminal's own default, which the stylesheet paints
	bold    bool
	reverse bool
}

// apply folds one SGR sequence's parameters into the state. A parameter this
// does not know is ignored rather than reset: the bulletins emit a small set,
// and a sequence from somewhere else should not blank the colour mid-line.
func (s *sgrState) apply(params []int) {
	for _, p := range params {
		switch {
		case p == 0:
			*s = sgrState{fg: -1, bg: -1}
		case p == 1:
			s.bold = true
		case p == 2 || p == 22:
			s.bold = false
		case p == 7:
			s.reverse = true
		case p == 27:
			s.reverse = false
		case p >= 30 && p <= 37:
			s.fg = p - 30
		case p == 39:
			s.fg = -1
		case p >= 40 && p <= 47:
			s.bg = p - 40
		case p == 49:
			s.bg = -1
		case p >= 90 && p <= 97:
			s.fg = p - 90 + 8
		case p >= 100 && p <= 107:
			s.bg = p - 100 + 8
		}
	}
}

// class is the span's class list, empty when the state is the page's default.
//
// Bold brightens a dark foreground, which is the convention every terminal
// follows and the one BRE's own 1;30 gray relies on. Reverse swaps the two,
// after that brightening, so a reversed cell shows the colours a terminal shows.
func (s sgrState) class() string {
	fg, bg := s.fg, s.bg
	if s.bold && fg >= 0 && fg < 8 {
		fg += 8
	}
	if s.reverse {
		// A default swapped into the other slot has to become a real colour, or
		// the reversal shows nothing: black text on the page's own foreground.
		if fg < 0 {
			fg = 7
		}
		if bg < 0 {
			bg = 0
		}
		fg, bg = bg, fg
	}
	var classes []string
	if fg >= 0 {
		classes = append(classes, fmt.Sprintf("ansi-fg-%d", fg))
	}
	if bg >= 0 {
		classes = append(classes, fmt.Sprintf("ansi-bg-%d", bg))
	}
	return strings.Join(classes, " ")
}

// scanEscape measures the escape sequence at the head of text and reports
// whether it is an SGR (colour) one, with its parameters. Everything else --
// cursor moves, the erase codes, the DECAWM wrap toggles -- is measured so it
// can be dropped, since a page has no cursor to move.
//
// It reports n == 0 for an ESC that begins nothing recognisable, so the caller
// drops the single byte rather than swallowing the line after it.
func scanEscape(text string) (params []int, isSGR bool, n int) {
	if len(text) < 2 || text[0] != 0x1b {
		return nil, false, 0
	}
	if text[1] != '[' {
		// A two-byte escape (ESC c and the like). None of the bulletins emit
		// one, but measuring it is what keeps a stray byte off the page.
		return nil, false, 2
	}
	for i := 2; i < len(text); i++ {
		c := text[i]
		if c >= 0x40 && c <= 0x7e { // the final byte of a CSI sequence
			if c != 'm' {
				return nil, false, i + 1
			}
			return parseSGRParams(text[2:i]), true, i + 1
		}
	}
	return nil, false, 0 // unterminated: not a sequence at all
}

// parseSGRParams splits an SGR body into its numbers. An empty body is ESC[m,
// which every terminal reads as ESC[0m; an empty field between semicolons is
// read the same way.
func parseSGRParams(body string) []int {
	if body == "" {
		return []int{0}
	}
	// A private-parameter introducer (ESC[?...m) is not a colour; ignore it.
	if body[0] < '0' || body[0] > '9' {
		if body[0] != ';' {
			return nil
		}
	}
	var out []int
	for _, field := range strings.Split(body, ";") {
		if field == "" {
			out = append(out, 0)
			continue
		}
		n := 0
		ok := true
		for _, c := range field {
			if c < '0' || c > '9' {
				ok = false
				break
			}
			n = n*10 + int(c-'0')
		}
		if ok {
			out = append(out, n)
		}
	}
	return out
}
