package menu

import (
	"embed"
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/screen"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The Empire Status is drawn from two artist-editable ANSI templates (CP437
// .ans, what PabloDraw/TheDraw produce). The templates own the layout and
// colors; the program fills their {label}/[value] tokens at display time (see
// internal/screen), so the screen can be reskinned without touching Go.
//
//go:embed screens/empire-status-1.ans screens/empire-status-2.ans
var empireStatusFS embed.FS

// empireStatusFields snapshots the player's empire together with its net worth
// (one consistent moment, even if another session mutates the world mid-render)
// and returns the value for each field token plus the translator for the label
// tokens. Values are plain strings; their colors come from the template.
func empireStatusFields(s session.Session, w *ctx) (map[string]string, func(string) string) {
	var p game.Empire
	var netWorth int
	w.With(func() {
		p = *w.Player()
		netWorth = w.NetWorth(w.Player())
	})
	pct := func(n int) string { return fmt.Sprintf("%d%%", n) }
	money := func(n int64) string { return formatGold(n, p.Language) }
	count := func(n int) string { return formatGold(n, p.Language) }
	f := map[string]string{
		"name":       p.Name,
		"turns":      comma(p.TurnsLeft),
		"score":      count(netWorth),
		"gold":       money(p.Gold),
		"bank":       money(p.Bank),
		"food":       count(p.Food),
		"debt":       money(p.Debt),
		"population": count(p.People),
		"tax":        pct(p.Tax),
		"support":    pct(p.Support),
		"morale":     pct(p.Morale),
		"sdi":        pct(p.SDI),
		"hq":         tr(s, hqStatus(&p)),
		"troopers":   comma(p.Troopers),
		"jets":       comma(p.Jets),
		"turrets":    comma(p.Turrets),
		"tanks":      comma(p.Tanks),
		"bombers":    comma(p.Bombers),
		"carriers":   comma(p.Carriers),
		"agents":     comma(p.Agents),
		"reign":      reignLine(s, &p),
	}
	for i, name := range regionTypeNames {
		f[strings.ToLower(name)] = comma(*regionField(&p, i))
	}
	return f, func(id string) string { return tr(s, id) }
}

// reignLine states how long the realm has stood, as BRE does: one sentence that
// counts down New Realm Protection while it lasts, then counts up. BRE's turn IS
// its year (its own wording is "Years of Protection"), so both halves are turns.
// It says "of your freedom" for the second, which reads as though the realm had
// been captive; IB names the thing the turn count actually measures.
func reignLine(s session.Session, p *game.Empire) string {
	if p.Protection > 0 {
		return fmt.Sprintf(tr(s, "You have %d years of protection left."), p.Protection)
	}
	return fmt.Sprintf(tr(s, "This is year %d of your reign."), p.TurnsPlayed)
}

// empireStatusPages are the template files, in display order.
var empireStatusPages = []string{"screens/empire-status-1.ans", "screens/empire-status-2.ans"}

// renderEmpireStatus draws the Empire Status pages, pausing between them so the
// first page can be read before the second scrolls it away. It does not pause
// after the last page — the caller pauses once for the final page (the turn
// summary shares that pause with the maintenance results; empireStatus adds its
// own).
func renderEmpireStatus(s session.Session, w *ctx) {
	fields, translate := empireStatusFields(s, w)
	for i, name := range empireStatusPages {
		if i > 0 {
			pause(s)
		}
		renderScreenPage(s, name, fields, translate)
	}
}

func renderScreenPage(s session.Session, name string, fields map[string]string, translate func(string) string) {
	tpl, _ := empireStatusFS.ReadFile(name)
	fmt.Fprint(s, screen.Render(tpl, fields, translate))
}
