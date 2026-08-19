package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Empire Status is BRE's status block, not a boxed screen: a `-*realm*-` title
// over a plain run of labelled lines, with nothing drawn around it (the capture
// and its colours are in docs/dev/bre-screens.md). Labels are white, every
// figure bright cyan, and a field whose figure is zero is left out — so the
// block is as tall as the realm is rich.
//
// The field set, its order, and the two row layouts come from the original's
// own status routine (BRE.OVR `show_empire_status`, 0x193b2, whose string table
// declares the labels in display order).

// statusMilitaryPerLine and statusRegionsPerLine are how many bracketed items
// the original puts on one line before indenting the rest under the label. Its
// helper takes the threshold as an argument: 3 for the Military row, 4 for
// Regions.
const (
	statusMilitaryPerLine = 3
	statusRegionsPerLine  = 4
)

// The block is bracketed by a blue inset rule — 70 columns, 5 single, 14
// double, 51 single — sitting immediately above the `-*realm*-` line and
// immediately below the reign line (cap/eots-ibbs-01.cap). A rule above and
// below is not a box: nothing runs down the sides and nothing is filled.
const (
	statusRuleWidth  = 70
	statusRuleDouble = 14
)

func statusRule() string {
	return ansi.FgBlue + insetRule(statusRuleWidth, statusRuleDouble) + ansi.Reset
}

// statusItem is one bracketed `[figure Label]` cell. width is its visible width,
// which the colour escapes in text make uncountable.
type statusItem struct {
	text  string
	width int
}

// hiFigure paints a figure the way the original's number formatter does —
// bright cyan — and hands the line back to the white it was written in.
func hiFigure(v string) string { return ansi.FgBrightCyan + v + ansi.FgWhite }

// statusCell builds one `[figure Label]` cell.
func statusCell(figure, label string) statusItem {
	return statusItem{
		text:  "[" + hiFigure(figure) + " " + label + "]",
		width: utf8.RuneCountInString(figure) + utf8.RuneCountInString(label) + 3,
	}
}

// renderEmpireStatus draws the Empire Status block. Callers pause after it: the
// System menu's action does so itself, and in the turn intro the block shares
// the pause that follows the maintenance results.
func renderEmpireStatus(s session.Session, w *ctx) {
	fmt.Fprint(s, empireStatusBlock(s, w))
}

// empireStatusBlock renders the block for the caller's own realm. It snapshots
// the empire first so every figure comes from one consistent moment, even if
// another session mutates the world mid-render.
func empireStatusBlock(s session.Session, w *ctx) string {
	var p game.Empire
	w.With(func() { p = *w.Player() })

	var b strings.Builder
	// Every figure goes through the formatter the rest of the game uses, so the
	// caller's thousands separator and the billions form hold here too.
	money := func(n int64) string { return formatGold(n, p.Language) }
	count := func(n int) string { return formatGold(n, p.Language) }
	// row writes one white line; the figures inside it carry their own colour.
	row := func(format string, a ...any) {
		fmt.Fprintf(&b, "%s%s%s\n", ansi.FgWhite, fmt.Sprintf(format, a...), ansi.Reset)
	}

	fmt.Fprintf(&b, "\n%s\n", statusRule())
	fmt.Fprintf(&b, "%s-*%s%s%s*-%s\n",
		ansi.FgBrightCyan, ansi.FgBrightWhite, p.Name, ansi.FgBrightCyan, ansi.Reset)

	row("%s: %s", tr(s, "Turns"), hiFigure(count(p.TurnsLeft)))
	row("%s: %s", tr(s, "Score"), hiFigure(count(p.Score)))
	if p.Gold > 0 {
		row("%s: %s", tr(s, "Gold"), hiFigure(money(p.Gold)))
	}
	if p.Bank > 0 {
		row("%s: %s", tr(s, "Bank"), hiFigure(money(p.Bank)))
	}
	row("%s: %s (%s: %s%%)", tr(s, "Population"), hiFigure(count(p.People)),
		tr(s, "Tax Rate"), hiFigure(count(p.Tax)))
	row("%s: %s%%", tr(s, "Popular Support"), hiFigure(count(p.Support)))
	if p.Food > 0 {
		row("%s: %s", tr(s, "Food"), hiFigure(count(p.Food)))
	}
	if p.Agents > 0 {
		row("%s: %s", tr(s, "Agents"), hiFigure(count(p.Agents)))
	}
	if p.HQ > 0 {
		row("%s: %s%% %s", tr(s, "Headquarters"), hiFigure(count(p.HQ)), tr(s, "Complete"))
	}

	// Military, then Regions: a label, its cells, and continuation lines indented
	// under the first bracket. Both labels pad to the same column so the two rows
	// and their continuations line up, which is what the original's fixed
	// ten-column prefixes do in English.
	milLabel, indent := statusRowPrefix(s, "Military")
	// The status screen's own order, which is not the Set Industries one —
	// hence a slice of the canonical rows rather than a re-use of MilitaryGoods.
	military := []*game.Good{game.Trooper, game.Jet, game.Turret, game.Tank, game.Bomber, game.Carrier}
	var cells []statusItem
	for _, g := range military {
		if n := *g.Count(&p); n > 0 {
			cells = append(cells, statusCell(count(n), tr(s, g.Plural)))
		}
	}
	if len(cells) == 0 {
		row("%s%s", milLabel, tr(s, "None"))
	} else {
		for _, l := range statusRows(milLabel, indent, cells, statusMilitaryPerLine) {
			row("%s", l)
		}
		// Morale rides the Military row on a continuation line of its own, as the
		// original writes it, and is left out entirely when there is nothing to
		// have morale.
		row("%s[%s%% %s]", indent, hiFigure(count(p.Morale)), tr(s, "Morale"))
	}
	if p.SDI > 0 {
		row("%s: %s%%", tr(s, "SDI Strength"), hiFigure(count(p.SDI)))
	}

	// Regions in the order the rest of the game uses (regionTypeNames, the Buy
	// Regions order), with Waste last as the region table lists it. The original
	// re-sorts this one row — its status block leads with Rivers and buries
	// Coastal seventh — while its record, its Buy Regions screen and every other
	// display keep the order used here.
	var regions []statusItem
	for i, name := range regionTypeNames {
		if n := *regionField(&p, i); n > 0 {
			regions = append(regions, statusCell(count(n), tr(s, name)))
		}
	}
	if p.Regions.Waste > 0 {
		regions = append(regions, statusCell(count(p.Regions.Waste), tr(s, "Waste")))
	}
	regLabel, _ := statusRowPrefix(s, "Regions")
	for _, l := range statusRows(regLabel, indent, regions, statusRegionsPerLine) {
		row("%s", l)
	}

	row("%s", reignLine(s, &p))
	fmt.Fprintf(&b, "%s\n", statusRule())
	return b.String()
}

// statusRowPrefix returns the padded label a Military or Regions row opens with
// and the indent its continuation lines take. Both rows pad to the same column,
// so the two rows and every continuation line up under one bracket — which is
// what the original's fixed ten-column prefixes do in English.
func statusRowPrefix(s session.Session, msgid string) (label, indent string) {
	w := 0
	for _, id := range []string{"Military", "Regions"} {
		if n := utf8.RuneCountInString(tr(s, id)); n > w {
			w = n
		}
	}
	w += 2
	label = tr(s, msgid) + ":"
	return label + strings.Repeat(" ", w-utf8.RuneCountInString(label)), strings.Repeat(" ", w)
}

// statusRows lays a row of bracketed cells out under its label, the way the
// original lays out Military and Regions: cells separated by a space, the row
// breaking after perLine of them with each continuation indented under the
// first bracket.
//
// Two deliberate differences. The original breaks ONCE — its helper compares the
// running count for equality — so a realm holding all nine region types puts
// four cells on the first line and five on the second. IB breaks every perLine
// instead, and breaks early when the next cell would reach the right edge: its
// figures carry thousands separators and its region counts have run past
// 250,000 in a long game, where a fixed single break overruns the line. The
// early break stops one column short of the last, since painting column 80 is
// what triggers a terminal's own wrap on top of the row's newline.
func statusRows(label, indent string, cells []statusItem, perLine int) []string {
	var rows []string
	var line strings.Builder
	line.WriteString(label)
	width, n := utf8.RuneCountInString(label), 0
	for _, c := range cells {
		if n > 0 && (n%perLine == 0 || width+1+c.width >= ansi.ScreenCols) {
			rows = append(rows, line.String())
			line.Reset()
			line.WriteString(indent)
			width, n = utf8.RuneCountInString(indent), 0
		}
		if n > 0 {
			line.WriteString(" ")
			width++
		}
		line.WriteString(c.text)
		width += c.width
		n++
	}
	return append(rows, line.String())
}

// reignLine states how long the realm has stood, as the original does: one
// sentence that counts New Realm Protection down while it lasts, then counts up.
// Two deliberate divergences, both recorded in docs/dev/bre-screens.md. The
// original calls the protection count "Years"; IB names the unit it actually
// holds, which is turns. And it says "of your freedom" for the second half,
// which reads as though the realm had been captive; IB names the thing the turn
// count measures.
func reignLine(s session.Session, p *game.Empire) string {
	if p.Protection > 0 {
		return fmt.Sprintf(tr(s, "You have %s turns of protection left."), hiFigure(formatGold(p.Protection, p.Language)))
	}
	return fmt.Sprintf(tr(s, "This is year %s of your reign."), hiFigure(formatGold(p.TurnsPlayed, p.Language)))
}
