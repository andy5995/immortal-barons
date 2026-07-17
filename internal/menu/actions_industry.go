package menu

import (
	"fmt"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prodTypeNames and prodField describe the 6 unit types Industrial regions
// can be set to build, in the order shown to the player.
var prodTypeNames = []string{"Troopers", "Jets", "Turrets", "Bombers", "Tanks", "Carriers"}

func prodField(p *game.Empire, idx int) *int {
	fields := []*int{
		&p.ProdTroopers, &p.ProdJets, &p.ProdTurrets, &p.ProdBombers, &p.ProdTanks, &p.ProdCarriers,
	}
	return fields[idx]
}

// setIndustries lets the player set the percentage of Industrial production
// points spent on each unit type. Percentages need not sum to 100; capacity not
// allocated to units is paid out as gold (BRE's trade-off — see industrialGold).
func setIndustries(s session.Session, w *ctx) Result {
	p := w.Player()
	proj := w.ProjectedProduction(p)
	fmt.Fprintf(s, "\n%s\n", titleRule(ansi.FgBrightRed, tr(s, "Industrial Production")))
	allocated := 0
	for i, name := range prodTypeNames {
		allocated += *prodField(p, i)
		fmt.Fprintf(s, "%-10s : %s%3d%%%s      %s"+tr(s, "(%d per year)")+"%s\n",
			tr(s, name), ansi.FgBrightYellow, *prodField(p, i), ansi.Reset, ansi.FgRed, proj[i], ansi.Reset)
	}
	if gold := 100 - allocated; gold > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "The remaining %d%% is turned into gold.")+"%s\n",
			ansi.FgBrightCyan, gold, ansi.Reset)
	}
	if p.Specialized != "" {
		fmt.Fprintf(s, "\n%s"+tr(s, "Specialized in %s: more of it, less of everything else.")+"%s\n",
			ansi.FgBrightCyan, tr(s, p.Specialized), ansi.Reset)
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
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Specialize Industry — choose a unit type. This is PERMANENT:"), ansi.Reset)
	for i, name := range prodTypeNames {
		fmt.Fprintf(s, "  %d) %s\n", i+1, tr(s, name))
	}
	t := promptInt(s, "Specialize in which unit (0 to cancel)?")
	if t < 1 || t > len(prodTypeNames) {
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
