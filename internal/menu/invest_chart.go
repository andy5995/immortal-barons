package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
)

// The Bank menu's investment chart: one column per day ahead, filled by how
// much of that date's MaxReturnsPerDate limit its returns already take,
// counting what the hold will pour onto it at the next maintenance. Every
// column is gold: the height carries the figure. IB's
// own addition; BRE's Bank menu has no chart.
//
// A column is investChartRows tall and fills in shades (shadeCells), so a full
// column always means a full date and a date holding anything at all shows at
// least a partly filled cell.
const (
	investChartRows = 5
	investChartDays = game.MaxInvestDays
	// investChartCol is a column's width, and investChartGutter the scale's
	// ("100% ") to the left of the first column.
	investChartCol    = 3
	investChartGutter = 5
	investChartColor  = ansi.FgBrightYellow
)

// investChart draws the chart for the caller's realm, centered in width, or
// nothing when no investment matures in the next investChartDays days. It is
// called inside the menu's world read, so it reads the player directly.
func investChart(w *ctx, width int) string {
	p := w.Player()
	if p == nil {
		return ""
	}
	pcts := make([]int, investChartDays)
	pour := p.PendingHoldPour(w.World.GameDay)
	due := false
	for d := range pcts {
		r := p.ReturnsOn(w.World.GameDay + d + 1)
		if d < len(pour) {
			r += pour[d]
		}
		pcts[d] = int(r * 100 / game.MaxReturnsPerDate)
		if r > 0 {
			// Under 1% floors to 0, which would draw no column at all.
			pcts[d] = max(pcts[d], 1)
			due = true
		}
	}
	if !due {
		return ""
	}

	lang := playerLang(w)
	// The block is centered under the menu box: a % scale in a gutter on the
	// left, the columns, a baseline rule and the day labels under it.
	plot := investChartDays*(investChartCol+1) - 1
	off := strings.Repeat(" ", max((width-investChartGutter-plot)/2, 0))
	var b strings.Builder
	head := i18n.T(lang, "Investment returns by day")
	headOff := max((investChartGutter+plot-utf8.RuneCountInString(head))/2, 0)
	fmt.Fprintf(&b, "%s%s%s%s%s\n", off, strings.Repeat(" ", headOff), ansi.FgWhite, head, ansi.Reset)
	axis := map[int]string{investChartRows: "100%", (investChartRows + 1) / 2: "50%"}
	for row := investChartRows; row >= 1; row-- {
		fmt.Fprintf(&b, "%s%s%*s%s ", off, ansi.FgWhite, investChartGutter-1, axis[row], ansi.Reset)
		for i, pct := range pcts {
			if i > 0 {
				b.WriteString(" ")
			}
			switch full, part := shadeCells(pct, investChartRows); {
			case row <= full:
				fmt.Fprintf(&b, "%s%s%s", investChartColor, strings.Repeat(shadeFull, investChartCol), ansi.Reset)
			case row == full+1 && part:
				fmt.Fprintf(&b, "%s%s%s", investChartColor, strings.Repeat(shadePart, investChartCol), ansi.Reset)
			default:
				b.WriteString(strings.Repeat(" ", investChartCol))
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s%s%*s %s%s\n", off, ansi.FgWhite, investChartGutter-1, "0%", strings.Repeat("─", plot), ansi.Reset)
	fmt.Fprintf(&b, "%s%s%s", off, strings.Repeat(" ", investChartGutter), ansi.FgBrightGreen)
	for d := range pcts {
		if d > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%-*s", investChartCol, fmt.Sprintf(" %d", d+1))
	}
	b.WriteString(ansi.Reset + "\n")
	return b.String()
}
