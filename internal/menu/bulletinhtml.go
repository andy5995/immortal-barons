package menu

import (
	"html/template"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// bulletinhtml.go — the bulletins as web pages that look like the game (#233).
//
// The page shows the SAME screen the .ans file does, colour for colour: the
// game has a look, and a bulletin a sysop links from their website should carry
// it rather than arrive as a generic web table. So this renders the ANSI the
// screens already produce, turning each colour into a span, instead of laying
// the data out a second way.
//
// Done in-process rather than by an external converter, because the screens are
// right here and a sysop should not have to install a tool and wire a step to
// publish a bulletin the game already wrote.
//
// The cost is honest and deliberate: a terminal screen is a fixed grid, so the
// page does not reflow and a narrow phone scrolls sideways. Anything that DID
// reflow would stop looking like the game, which is the whole point.

// ansiPalette maps each SGR colour to a hex the page can use. The values are a
// terminal's own colours, adjusted where one failed contrast on black: plain
// black would be 2.48:1 and unreadable as text, so it renders as the grey a
// terminal shows for dim rather than as a colour nobody could read. Every value
// here is measured in TestBulletinPaletteMeetsContrast.
var ansiPalette = map[int]string{
	30: "#767676", 31: "#e05561", 32: "#4fc16f", 33: "#c8a020",
	34: "#5a86d6", 35: "#c678dd", 36: "#3fb6b6", 37: "#c8ccd4",
	90: "#7f8c99", 91: "#ff6b74", 92: "#5ff08a", 93: "#ffd166",
	94: "#79aaff", 95: "#e39ff6", 96: "#56d4d4", 97: "#ffffff",
}

// gameSite is the game's own page, not its source repository: a reader who
// followed a bulletin from a BBS wants to know what the game is, and the place
// that answers that is the site (the About screen links the same one).
const gameSite = "https://andy5995.github.io/immortal-barons/"

// bulletinBG is the page's background. Black, because that is what the screens
// were drawn against and every colour above is measured against it.
const bulletinBG = "#000000"

const bulletinCSS = `
body { background: ` + bulletinBG + `; margin: 0; padding: 1rem; }
pre.screen {
  color: #c8ccd4; margin: 0 auto; width: max-content;
  font: 15px/1.35 ui-monospace, "DejaVu Sans Mono", "Liberation Mono", "Courier New", monospace;
  white-space: pre; tab-size: 8;
}
pre.screen b { font-weight: 700; }
.wrap { overflow-x: auto; }
footer {
  color: #7f8c99; font: 13px/1.5 ui-monospace, monospace;
  max-width: 60rem; margin: 1.5rem auto 0;
}
footer a { color: #79aaff; }
footer .board { color: #c8ccd4; }
footer a:focus-visible { outline: 2px solid #ffd166; outline-offset: 2px; }
`

var bulletinTemplate = template.Must(template.New("bulletin").Parse(
	`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}{{if .Board}} — {{.Board}}{{end}}</title>
<style>` + bulletinCSS + `</style></head>
<body>
<div class="wrap"><pre class="screen">{{.Screen}}</pre></div>
<footer>{{if .Board}}{{if .BoardURL}}<a href="{{.BoardURL}}">{{.Board}}</a>{{else}}<span class="board">{{.Board}}</span>{{end}} · {{end}}Written by <a href="` + gameSite + `">Immortal Barons</a> v{{.Version}}. This page is generated; edits are overwritten.</footer>
</body></html>
`))

type htmlPage struct {
	Title, Board, Version string
	// BoardURL is the sysop's own site, from bbs.cfg. It goes through
	// html/template's URL context, which neutralises a javascript: or data:
	// value rather than emitting it -- the setting is the sysop's own, but a
	// page they publish should not be able to carry a scheme like that by
	// accident.
	BoardURL string
	Screen   template.HTML
}

// ansiToHTML turns one rendered screen into markup. Only the sequences the
// screens actually emit are handled -- colour, bold, dim and reset -- and
// anything else is dropped rather than shown, because a stray escape printed as
// text is worse than a missing effect.
func ansiToHTML(screen string) template.HTML {
	var out strings.Builder
	open := false
	closeSpan := func() {
		if open {
			out.WriteString("</span>")
			open = false
		}
	}
	colour, bold, dimmed := "", false, false
	openSpan := func() {
		closeSpan()
		if colour == "" && !dimmed {
			return
		}
		style := colour
		if dimmed {
			// A terminal renders dim as a darkened foreground; opacity is the
			// same idea and keeps whatever colour is in force.
			style += ";opacity:.72"
		}
		out.WriteString(`<span style="color:` + style + `">`)
		open = true
	}
	for i := 0; i < len(screen); {
		if screen[i] != 0x1b {
			// html/template does not escape inside template.HTML, so every
			// character of the screen -- which carries realm names a player
			// chose -- is escaped here instead.
			out.WriteString(template.HTMLEscapeString(string(screen[i])))
			i++
			continue
		}
		end := strings.IndexByte(screen[i:], 'm')
		if !strings.HasPrefix(screen[i:], "\x1b[") || end < 0 {
			i++ // not an SGR sequence: drop the escape and carry on
			continue
		}
		params := screen[i+2 : i+end]
		i += end + 1
		for _, p := range strings.Split(params, ";") {
			n, err := strconv.Atoi(p)
			if err != nil {
				continue
			}
			switch {
			case n == 0:
				colour, bold, dimmed = "", false, false
			case n == 1:
				bold = true
			case n == 2:
				dimmed = true
			default:
				if c, ok := ansiPalette[n]; ok {
					colour = c
				}
			}
		}
		_ = bold // bold is carried by the bright colours the screens use
		openSpan()
	}
	closeSpan()
	return template.HTML(out.String())
}

// writeBulletinHTML renders one page from the screen already drawn for the .ans
// file, so the two cannot disagree about what the bulletin looks like.
func writeBulletinHTML(dir, base, title, board, boardURL string, screen []byte) error {
	var buf strings.Builder
	if err := bulletinTemplate.Execute(&buf, htmlPage{
		Title: title, Board: board, BoardURL: boardURL,
		Version: gameVersion(), Screen: ansiToHTML(string(screen)),
	}); err != nil {
		return err
	}
	return writeFileIfChanged(filepath.Join(dir, base+".html"), []byte(buf.String()))
}

func gameVersion() string { return game.Version }
