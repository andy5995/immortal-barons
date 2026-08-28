package menu

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prodRow is one row of the Set Industries screen: a label and the percentage
// field it sets. The rows are the units Industrial regions build, in
// game.MilitaryGoods order — the same order ProjectedProduction returns, so row
// i's projection is projection i — then Gold. Gold is last and starts at 0:
// capacity left unallocated pays gold anyway, so the row is for reserving it
// deliberately.
type prodRow struct {
	name  string
	field func(*game.Empire) *int
}

// prodRows builds the screen's rows from the canonical unit table (#134), so a
// row can never come to label a different field than it sets.
func prodRows() []prodRow {
	rows := make([]prodRow, 0, prodUnitCount+1)
	for _, g := range game.MilitaryGoods {
		rows = append(rows, prodRow{g.Plural, g.Prod})
	}
	return append(rows, prodRow{"Gold", func(e *game.Empire) *int { return &e.ProdGold }})
}

// prodUnitCount is how many of those rows are units, and so how many have a
// per-year production figure. Gold sits past the end of ProjectedProduction.
var prodUnitCount = len(game.MilitaryGoods)

// industryRuleWidth is the width of the Industrial Production box. BRE sizes
// every menu box to its own content rather than to one house width — its
// captures run from 23 to 76 columns — and Industrial Production is 46
// (docs/dev/bre-screens.md).
const industryRuleWidth = 46

// specializationRuleWidth is the Specialization box, 14 columns, closed by a
// 14-column rule under a bare 16-column [Specialization] title that overhangs
// it (docs/dev/bre-screens.md). It is the one box whose title is wider than its
// content. The width grows to the widest item when a translation needs it,
// since sizing to content is what produces 14 in the first place.
const specializationRuleWidth = 14

// industryAccent is the bright accent for both screens; the rules are drawn in
// its dim form, as the menu engine does. BRE's accent for Spending / Industrial
// Production / Trading / Specialization is red (docs/dev/bre-screens.md).
var industryAccent = ansi.FgBrightRed

// industryRule is the Industrial Production box's closing rule.
func industryRule() string {
	return closingRule(industryAccent, industryRuleWidth)
}

// drawIndustry renders the Industrial Production box. It is separate from the
// flow below because the screen is shown again once the walk has set the new
// percentages: the table with the new figures in it is the confirmation, so
// there is no "updated" line to read past.
func drawIndustry(s session.Session, w *ctx, p *game.Empire, rows []prodRow) {
	proj := w.ProjectedProduction(p)
	fmt.Fprintf(s, "\n%s\n", titleRule(industryAccent, tr(s, "Industrial Production"), industryRuleWidth))
	allocated := 0
	for i, row := range rows {
		pct := *row.field(p)
		allocated += pct
		// Columns match BRE's capture: the colon at 16, the figure's paren at 29.
		fmt.Fprintf(s, "%-16s: %s%3d%%%s", tr(s, row.name), ansi.FgBrightYellow, pct, ansi.Reset)
		// Units come from the projection; the Gold row asks the same function
		// that will pay it, so the figure shown is the one credited.
		per := w.ProjectedIndustrialGold(p, pct)
		if i < prodUnitCount {
			per = proj[i]
		}
		fmt.Fprintf(s, "       %s"+tr(s, "(%s per year)")+"%s", ansi.FgRed, comma(per), ansi.Reset)
		// The original tags the specialized row at the end of the line, where it
		// wrote "Specialized"; three spaced asterisks say the same thing without
		// a word to translate or to run past the margin.
		if p.Specialized == row.name {
			fmt.Fprintf(s, "  %s* * *%s", ansi.FgBrightYellow, ansi.Reset)
		}
		fmt.Fprint(s, "\n")
	}
	if gold := 100 - allocated; gold > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "The remaining %d%% is turned into gold.")+"%s\n",
			ansi.FgBrightCyan, gold, ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", industryRule())
}

// setIndustries lets the player set the percentage of Industrial production
// points spent on each unit type. Percentages need not sum to 100; capacity not
// allocated to units is paid out as gold (BRE's trade-off — see industrialGold).
func setIndustries(s session.Session, w *ctx) Result {
	p := w.Player()
	rows := prodRows()
	drawIndustry(s, w, p, rows)
	if !AskYesNo(s, "Change Production?", false) {
		return Stay
	}
	// Walk each unit like BRE (Troopers, Jets, …), capping the max at the budget
	// left so the total can't exceed 100% (BRE does this via a shrinking max).
	// Unlike BRE, the suggested value is the CURRENT % (clamped to what's left),
	// so pressing Enter keeps a unit unchanged instead of zeroing it (a deliberate
	// UX improvement — see the Set Industries note in docs/mechanics-reference.md).
	ns := make([]int, len(rows))
	remaining := 100
	// Pad every unit label to the widest (rune-based, so translations line up too)
	// and right-align the numbers, so the input column lines up down the list.
	labelW := 0
	for _, row := range rows {
		if wdt := utf8.RuneCountInString(tr(s, row.name)); wdt > labelW {
			labelW = wdt
		}
	}
	fmt.Fprint(s, "\n") // one blank line after "Change Production? y", then the units follow consecutively
	for i, row := range rows {
		// Nothing left to allocate: asking for a percentage whose only legal
		// answer is 0 is a dead question, so stop the walk and leave the rest of
		// ns at its zero value.
		if remaining == 0 {
			break
		}
		cur := *row.field(p)
		if cur > remaining {
			cur = remaining
		}
		ns[i] = promptProduction(s, tr(s, row.name), labelW, cur, remaining)
		remaining -= ns[i]
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		for i, n := range ns {
			*rows[i].field(p) = n
		}
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	drawIndustry(s, w, w.Player(), rows)
	pause(s)
	return Stay
}

// specializeIndustry lets the player concentrate all Industrial production
// into a single unit type. This is permanent, matching the original BRE's
// one-way specialization; once set it cannot be undone.
func specializeIndustry(s session.Session, w *ctx) Result {
	p := w.Player()
	if p.Specialized != "" {
		ok(s, "Your industry is already specialized in %s.", p.Specialized)
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "This choice is permanent and cannot be undone."), ansi.Reset)
	// Units only. Gold is a row on Set Industries but not a unit, and there is
	// nothing for a specialization's efficiency modifier to apply to.
	items := make([]string, 0, prodUnitCount+1)
	for i, g := range game.MilitaryGoods {
		items = append(items, fmt.Sprintf("  %d) %s", i+1, tr(s, g.Plural)))
	}
	items = append(items, fmt.Sprintf("  0) %s", tr(s, "Quit")))
	width := specializationRuleWidth
	for _, it := range items {
		if n := utf8.RuneCountInString(it); n > width {
			width = n
		}
	}
	// The original draws this as a red-accent menu, so it matches the rest of the
	// game rather than reading as a bare list (docs/dev/bre-screens.md).
	fmt.Fprintf(s, "%s\n", titleRule(industryAccent, tr(s, "Specialization"), width))
	for _, it := range items {
		fmt.Fprintf(s, "%s\n", it)
	}
	// The original closes every menu box with a rule, whether or not a status
	// line follows it (see the menu engine's draw).
	fmt.Fprintf(s, "%s\n", closingRule(industryAccent, width))
	t := ChoiceQuit(s, prodUnitCount)
	if t < 1 {
		ok(s, "Your industry was left unspecialized.")
		return Stay
	}
	// Inside the transaction, so a visit that specialized between the prompt and
	// here keeps its choice rather than being overwritten.
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.Specialize(p, game.MilitaryGoods[t-1])
	})
	switch {
	case errors.Is(err, game.ErrAlreadySpecialized):
		ok(s, "Your industry is already specialized.")
	case err != nil:
		fail(s, err)
	default:
		ok(s, "Your industry is now permanently specialized in %s.", game.MilitaryGoods[t-1].Plural)
	}
	return Stay
}
