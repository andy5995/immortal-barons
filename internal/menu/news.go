package menu

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// news.go — the Daily Bulletin and the planetary news feed: the masthead, the
// box they are drawn in, and the highlighting that picks realm names and
// figures out of a line of prose.

// News-screen chrome, matched to BRE's captured layout (docs/dev/bre-screens.md).
const (
	// newsBannerName is the masthead over the planetary news — IB's own flavor
	// name where BRE has "The Queen's Quadrant". Tunable; a single string.
	newsBannerName = "The Oligarchy Express"
	// newsItemArrow leads each planetary-news item: red "──" + bright-red "─".
	newsItemArrow = ansi.FgRed + "──" + ansi.FgBrightRed + "─" + ansi.Reset
	// newsItemIndent is the hanging indent under that arrow, 5 spaces as BRE
	// wraps it — a column wider than the arrow itself.
	newsItemIndent = "     "
	newsBoxIndent  = "    "
	// newsBoxInner is the printable width between the box's │ borders. BRE draws
	// the Daily Bulletin in SINGLE-line CP437 at 66 columns overall, indented 4
	// (docs/dev/bre-screens.md) — this was a 54-wide double-line box until
	// 2026-08-16, following a code block in that file that no capture produces.
	newsBoxInner = 64
)

// renderDailyBulletin draws the boxed blue Daily Bulletin: planet-wide totals
// with day-over-day change, the change bright cyan behind a bright-green "+" on
// a rising day and a plain bright-cyan "-" on a falling one. title is the
// board name in a league, or "" for a stand-alone board, which shows just
// "Daily Bulletin".
func renderDailyBulletin(s session.Session, b game.DailyBulletin, title string) {
	head := tr(s, "Daily Bulletin")
	if title != "" {
		// The board name is the sysop's, and a long one would blow the box out of
		// the screen — the masthead is drawn to a fixed rule.
		head = game.FitColumn(title, 30) + " — " + head
	}
	boxTop(s, head)

	row := func(label string, total, change int, fmtNum func(int) string) {
		sign := "+"
		abs := change
		if change < 0 {
			sign = "-"
			abs = -change
		}
		// BRE colors the change bright cyan whichever way it moved and paints only
		// a rising day's "+" bright green, so direction is carried by the sign, not
		// by color (docs/dev/bre-screens.md). IB drew a fall in dim red until
		// then — 2.7:1 on the VGA palette, under the 4.5:1 text minimum.
		signClr := ansi.FgBrightCyan
		if change > 0 {
			signClr = ansi.FgBrightGreen
		}
		lab := fmt.Sprintf("%-18s", tr(s, label)+":")
		val := fmt.Sprintf("%-16s", fmtNum(total))
		num := fmtNum(abs)
		inner := "  " + ansi.FgWhite + lab + ansi.FgCyan + val + ansi.FgWhite + tr(s, "Change") + ": " +
			signClr + sign + ansi.FgBrightCyan + num + ansi.Reset
		vis := 2 + len([]rune(lab)) + len([]rune(val)) + len([]rune(tr(s, "Change")+": ")) + 1 + len([]rune(num))
		boxRow(s, inner, vis)
	}

	locale := func(n int) string { return formatGold(n, sessionLang(s)) }
	row("Total Population", b.Totals.Population, b.Change.Population, locale)
	row("Total Regions", b.Totals.Regions, b.Change.Regions, locale)
	row("Total Net Worth", b.Totals.NetWorth, b.Change.NetWorth, abbrevMoney)
	boxBottom(s)
}

// boxTop draws the box's top border with head embedded (bright-white) in the ─
// line; boxRow draws one │…│ content line padded to newsBoxInner (innerVis is
// the printable width of inner, excluding ANSI); boxBottom closes it. BRE uses
// the single-line CP437 set here, not the double one (docs/dev/bre-screens.md).
func boxTop(s session.Session, head string) {
	tw := len([]rune(head))
	if tw > newsBoxInner {
		tw = newsBoxInner
	}
	left := (newsBoxInner - tw) / 2
	right := newsBoxInner - tw - left
	fmt.Fprintf(s, "\n%s%s┌%s%s%s%s%s┐%s\n",
		newsBoxIndent, ansi.FgBlue,
		strings.Repeat("─", left),
		ansi.FgBrightWhite, head, ansi.FgBlue,
		strings.Repeat("─", right), ansi.Reset)
}

func boxRow(s session.Session, inner string, innerVis int) {
	pad := newsBoxInner - innerVis
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(s, "%s%s│%s%s%s%s│%s\n",
		newsBoxIndent, ansi.FgBlue, ansi.Reset,
		inner, strings.Repeat(" ", pad),
		ansi.FgBlue, ansi.Reset)
}

func boxBottom(s session.Session) {
	fmt.Fprintf(s, "%s%s└%s┘%s\n",
		newsBoxIndent, ansi.FgBlue, strings.Repeat("─", newsBoxInner), ansi.Reset)
}

// renderNewsMasthead prints the news screen's header: the program/version
// "News File" line with the date right-aligned, the centered banner, and a
// yellow rule — matched to BRE's captured layout.
func renderNewsMasthead(s session.Session, date string) {
	headVis := len([]rune(tr(s, "Immortal Barons") + " v" + game.Version + tr(s, ": News File")))
	fmt.Fprintf(s, "\n%s%s %sv%s%s%s",
		ansi.FgBrightYellow, tr(s, "Immortal Barons"),
		ansi.FgBrightWhite, game.Version,
		ansi.FgWhite, tr(s, ": News File"))
	if date != "" {
		pad := len([]rune(rule)) - headVis - len([]rune(date))
		if pad < 1 {
			pad = 1
		}
		fmt.Fprintf(s, "%s%s", strings.Repeat(" ", pad), date)
	}
	fmt.Fprintf(s, "%s\n", ansi.Reset)

	// The inner glyph is `═` (CP437 0xCD), not a guillemet: BRE draws this banner,
	// the Planetary Post one and the Planetary Treaties one all as ──═title═──
	// (docs/dev/bre-screens.md). The `─»>Paused<«─` bar is a different decoration
	// and does use guillemets.
	banner := ansi.FgRed + "──" + ansi.FgBrightRed + "═" + ansi.FgBrightWhite + tr(s, newsBannerName) + ansi.FgBrightRed + "═" + ansi.FgRed + "──" + ansi.Reset
	bannerVis := 2 + 1 + len([]rune(tr(s, newsBannerName))) + 1 + 2
	indent := (len([]rune(rule)) - bannerVis) / 2
	if indent < 0 {
		indent = 0
	}
	fmt.Fprintf(s, "\n%s%s\n", strings.Repeat(" ", indent), banner)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, rule, ansi.Reset)
}

// hiTerm is a name the news highlighter should color where it appears.
type hiTerm struct{ text, color string }

// hiNewsItem colors one planetary-news line the way BRE does: known names
// (empires, pirate factions, the Planetary Master title) in their accent color
// and digit runs in bright-white, over a white body. Single left-to-right pass
// matching the LONGEST term at each position, so "Iron Dominion" is not split by
// a shorter "Iron"; ANSI codes it inserts never contain a term, so there is no
// re-match. Replaces the numbers-only hiNums for news text.
func hiNewsItem(line string, terms []hiTerm) string {
	sort.SliceStable(terms, func(i, j int) bool { return len(terms[i].text) > len(terms[j].text) })
	runes := []rune(line)
	var b strings.Builder
	b.WriteString(ansi.FgWhite)
	for i := 0; i < len(runes); {
		matched := false
		for _, t := range terms {
			tr := []rune(t.text)
			if t.text != "" && i+len(tr) <= len(runes) && string(runes[i:i+len(tr)]) == t.text {
				b.WriteString(t.color + t.text + ansi.FgWhite)
				i += len(tr)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if isDigit(runes[i]) {
			j := i
			for j < len(runes) && (isDigit(runes[j]) || (runes[j] == ',' && j+1 < len(runes) && isDigit(runes[j+1]))) {
				j++
			}
			b.WriteString(ansi.FgBrightWhite + string(runes[i:j]) + ansi.FgWhite)
			i = j
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	b.WriteString(ansi.Reset)
	return b.String()
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// newsHighlightTerms is the set of names to color in planetary-news lines:
// every empire in bright-yellow, the pirate factions in bright-red, and the
// Planetary Master title in bright-white. Read off the live captures in `cap/`,
// where three different realms in one news screen all render `1;33` — BRE gives
// the reader's own realm no distinct color here, unlike the recap (bright-cyan)
// or the status screens. Factions are the one deliberate divergence: BRE uses plain
// `31`, which is 2.71:1 on black and under the legibility floor, so IB brightens
// to `1;31` within the same hue (docs/dev/bre-screens.md records it).
func newsHighlightTerms(w *ctx) []hiTerm {
	var terms []hiTerm
	w.With(func() {
		for _, e := range w.Empires {
			terms = append(terms, hiTerm{e.Name, ansi.FgBrightYellow})
		}
	})
	for _, f := range game.PirateFactions {
		terms = append(terms, hiTerm{f, ansi.FgBrightRed})
	}
	terms = append(terms, hiTerm{"Planetary Master", ansi.FgBrightWhite})
	return terms
}

// newsDate formats an ISO date (2006-01-02) as BRE's M-D-YYYY, or "" when empty
// or unparseable (the masthead then omits the date).
func newsDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d-%d", int(t.Month()), t.Day(), t.Year())
}

// prevISODate returns the ISO day before iso (for the Yesterday's News header),
// or "" if unparseable.
func prevISODate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}
