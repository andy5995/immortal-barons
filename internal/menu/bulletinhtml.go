package menu

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

// bulletinhtml.go — the same bulletins as a web page (#233).
//
// Written from the game's own data, NOT by translating the ANSI screen. A
// terminal screen is a fixed grid of characters and a web page is not, so
// converting one into the other yields a page that is worse at being a page
// and no better at being a screen. The tables here are tables; the ANSI
// versions stay exactly as they were.
//
// Self-contained on purpose: one file, inline CSS, no fonts or scripts to
// fetch. A sysop drops it in whatever directory their web server already
// serves, and it works with no other moving part.

// bulletinPalette is checked, not chosen by eye. Every value below is its WCAG
// contrast ratio against that theme's background, computed rather than
// estimated: text 15.80 or better, the coloured outcome words 5.08 at worst,
// and the rules 3.04 at worst against the 3:1 that non-text needs.
const bulletinCSS = `
:root {
  --bg: #ffffff; --text: #1f2328; --muted: #59636e; --accent: #7d4e00;
  --win: #1a7f37; --loss: #cf222e; --planet: #0550ae; --rule: #818b95;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #0d1117; --text: #e6edf3; --muted: #9198a1; --accent: #d29922;
    --win: #3fb950; --loss: #f85149; --planet: #79c0ff; --rule: #7d8590;
  }
}
:root[data-theme="dark"] {
  --bg: #0d1117; --text: #e6edf3; --muted: #9198a1; --accent: #d29922;
  --win: #3fb950; --loss: #f85149; --planet: #79c0ff; --rule: #7d8590;
}
* { box-sizing: border-box; }
body {
  background: var(--bg); color: var(--text); margin: 0; padding: 1.5rem 1rem;
  font: 16px/1.5 ui-monospace, "DejaVu Sans Mono", "Courier New", monospace;
}
main { max-width: 60rem; margin: 0 auto; }
h1 { font-size: 1.25rem; margin: 0 0 .25rem; color: var(--accent); }
.sub { color: var(--muted); margin: 0 0 1.25rem; font-size: .9rem; }
.scroll { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; min-width: 34rem; }
caption { text-align: left; color: var(--muted); padding-bottom: .4rem; }
th, td { text-align: left; padding: .35rem .6rem; border-bottom: 1px solid var(--rule); }
th { color: var(--muted); font-weight: 700; }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
.planet { color: var(--planet); }
.win { color: var(--win); font-weight: 700; }
.loss { color: var(--loss); font-weight: 700; }
.you { color: var(--accent); font-weight: 700; }
.news li { margin: .3rem 0; }
footer { color: var(--muted); font-size: .85rem; margin-top: 2rem; }
@media (prefers-reduced-motion: reduce) { * { transition: none !important; } }
`

// htmlPage is what every bulletin page is filled from.
type htmlPage struct {
	Title, Board, Date, Version string
	Scores                      []ScoreRow
	Battles                     []game.BattleLogEntry
	News                        []string
	Bulletin                    game.DailyBulletin
	Here                        string
}

var bulletinTemplate = template.Must(template.New("bulletin").Funcs(template.FuncMap{
	// outcome words the battle the same way the terminal report does, and for
	// the same reason: the word carries the meaning so the page reads correctly
	// to someone who cannot tell the colours apart.
	"outcome": func(b game.BattleLogEntry) string {
		switch {
		case b.Crushed:
			return "CRUSHED"
		case b.Won && b.Land > 0:
			return fmt.Sprintf("took %d", b.Land)
		case b.Won:
			return "won"
		}
		return "held"
	},
	"outcomeClass": func(b game.BattleLogEntry) string {
		if b.Won || b.Crushed {
			return "win"
		}
		return "loss"
	},
	"planetOf": func(b game.BattleLogEntry, here string) string {
		if b.Planet == "" || b.Planet == here {
			return here
		}
		return b.Planet
	},
	"reverse": func(in []game.BattleLogEntry) []game.BattleLogEntry {
		out := make([]game.BattleLogEntry, 0, len(in))
		for i := len(in) - 1; i >= 0; i-- {
			out = append(out, in[i])
		}
		return out
	},
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}{{if .Board}} — {{.Board}}{{end}}</title>
<style>` + bulletinCSS + `</style></head>
<body><main>
<h1>{{.Title}}</h1>
<p class="sub">{{if .Board}}{{.Board}} · {{end}}{{if .Date}}{{.Date}} · {{end}}Immortal Barons v{{.Version}}</p>
{{if .Scores}}
<div class="scroll"><table>
<thead><tr><th>Id</th><th>Empire</th><th class="num">Territory</th><th class="num">Score</th><th class="num">Net Worth</th></tr></thead>
<tbody>{{range .Scores}}<tr>
<td>{{.Letter}}</td>
<td{{if .IsPlayer}} class="you"{{end}}>{{.Name}}{{if not .Alive}} (dead){{end}}</td>
<td class="num">{{.Land}}</td><td class="num">{{.Score}}</td><td class="num">{{.NW}}</td>
</tr>{{end}}</tbody></table></div>
{{end}}
{{if .Battles}}
<div class="scroll"><table>
<thead><tr><th>Planet</th><th>Attacker</th><th>Defender</th><th>Outcome</th></tr></thead>
<tbody>{{range reverse .Battles}}<tr>
<td class="planet">{{planetOf . $.Here}}</td><td>{{.Attacker}}</td><td>{{.Defender}}</td>
<td class="{{outcomeClass .}}">{{outcome .}}</td>
</tr>{{end}}</tbody></table></div>
{{end}}
{{if .News}}<ul class="news">{{range .News}}<li>{{.}}</li>{{end}}</ul>{{end}}
{{if and (not .Scores) (not .Battles) (not .News)}}<p class="sub">Nothing to report.</p>{{end}}
<footer>Written by Immortal Barons. This page is generated; edits are overwritten.</footer>
</main></body></html>
`))

// writeBulletinHTML renders one page. Errors join the rest: a page that cannot
// be written must not stop the .ans and .txt beside it.
func writeBulletinHTML(dir, base string, p htmlPage) error {
	var buf bytes.Buffer
	if err := bulletinTemplate.Execute(&buf, p); err != nil {
		return err
	}
	return writeFileIfChanged(filepath.Join(dir, base+".html"), buf.Bytes())
}
