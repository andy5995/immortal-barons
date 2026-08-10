package menu

import (
	"fmt"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prodTypeNames and prodField describe the rows of the Set Industries screen in
// the order the player sees them: the 6 unit types Industrial regions can build,
// then Gold. Gold is last and starts at 0 — capacity left unallocated pays gold
// anyway, so the row is for reserving it deliberately.
var prodTypeNames = []string{"Troopers", "Jets", "Turrets", "Bombers", "Tanks", "Carriers", "Gold"}

// prodUnitCount is how many of those rows are units, and so how many have a
// per-year production figure. Gold sits past the end of ProjectedProduction.
const prodUnitCount = 6

func prodField(p *game.Empire, idx int) *int {
	fields := []*int{
		&p.ProdTroopers, &p.ProdJets, &p.ProdTurrets, &p.ProdBombers, &p.ProdTanks, &p.ProdCarriers,
		&p.ProdGold,
	}
	return fields[idx]
}

// setIndustries lets the player set the percentage of Industrial production
// points spent on each unit type. Percentages need not sum to 100; capacity not
// allocated to units is paid out as gold (BRE's trade-off — see industrialGold).
func setIndustries(s session.Session, w *ctx) Result {
	p := w.Player()
	proj := w.ProjectedProduction(p)
	fmt.Fprintf(s, "\n%s\n", titleRule(ansi.FgBrightRed, tr(s, "Industrial Production"), len([]rune(rule))))
	allocated := 0
	for i, name := range prodTypeNames {
		allocated += *prodField(p, i)
		fmt.Fprintf(s, "%-10s : %s%3d%%%s", tr(s, name), ansi.FgBrightYellow, *prodField(p, i), ansi.Reset)
		// Units come from the projection; the Gold row asks the same function
		// that will pay it, so the figure shown is the one credited.
		per := w.ProjectedIndustrialGold(p, *prodField(p, i))
		if i < prodUnitCount {
			per = proj[i]
		}
		fmt.Fprintf(s, "      %s"+tr(s, "(%s per year)")+"%s", ansi.FgRed, comma(per), ansi.Reset)
		// The original tags the specialized row at the end of the line, where it
		// wrote "Specialized"; three spaced asterisks say the same thing without
		// a word to translate or to run past the margin.
		if p.Specialized == name {
			fmt.Fprintf(s, "  %s* * *%s", ansi.FgBrightYellow, ansi.Reset)
		}
		fmt.Fprint(s, "\n")
	}
	if gold := 100 - allocated; gold > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "The remaining %d%% is turned into gold.")+"%s\n",
			ansi.FgBrightCyan, gold, ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	if !AskYesNo(s, "Change Production?", false) {
		return Stay
	}
	// Walk each unit like BRE (Troopers, Jets, …), capping the max at the budget
	// left so the total can't exceed 100% (BRE does this via a shrinking max).
	// Unlike BRE, the suggested value is the CURRENT % (clamped to what's left),
	// so pressing Enter keeps a unit unchanged instead of zeroing it (a deliberate
	// UX improvement — see the Set Industries note in docs/mechanics-reference.md).
	ns := make([]int, len(prodTypeNames))
	remaining := 100
	// Pad every unit label to the widest (rune-based, so translations line up too)
	// and right-align the numbers, so the input column lines up down the list.
	labelW := 0
	for _, name := range prodTypeNames {
		if wdt := utf8.RuneCountInString(tr(s, name)); wdt > labelW {
			labelW = wdt
		}
	}
	fmt.Fprint(s, "\n") // one blank line after "Change Production? y", then the units follow consecutively
	for i, name := range prodTypeNames {
		cur := *prodField(p, i)
		if cur > remaining {
			cur = remaining
		}
		ns[i] = promptProduction(s, tr(s, name), labelW, cur, remaining)
		remaining -= ns[i]
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		for i, n := range ns {
			*prodField(p, i) = n
		}
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Industry production percentages updated.")
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
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose one unit type. The choice is permanent and cannot be changed later."), ansi.Reset)
	// The original draws this as a red-accent menu, so it matches the rest of the
	// game rather than reading as a bare list (docs/dev/bre-screens.md).
	fmt.Fprintf(s, "%s\n", titleRule(ansi.FgBrightRed, tr(s, "Specialization"), len([]rune(rule))))
	// Units only. Gold is a row on Set Industries but not a unit, and there is
	// nothing for a specialization's efficiency modifier to apply to.
	for i, name := range prodTypeNames[:prodUnitCount] {
		fmt.Fprintf(s, "  %d) %s\n", i+1, tr(s, name))
	}
	fmt.Fprintf(s, "  0) %s\n", tr(s, "Quit"))
	// The original closes every menu box with a rule, whether or not a status
	// line follows it (see the menu engine's draw).
	fmt.Fprintf(s, "%s\n", rule)
	t := ChoiceQuit(s, prodUnitCount)
	if t < 1 {
		ok(s, "Your industry was left unspecialized.")
		return Stay
	}
	var already bool
	err := w.mutatePlayer(func(p *game.Empire) error {
		// Re-check against fresh state: specialization is permanent, so if another
		// visit set it between the prompt and here, keep the existing choice.
		if p.Specialized != "" {
			already = true
			return nil
		}
		p.Specialized = prodTypeNames[t-1]
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	if already {
		ok(s, "Your industry is already specialized.")
		return Stay
	}
	ok(s, "Your industry is now permanently specialized in %s.", prodTypeNames[t-1])
	return Stay
}
