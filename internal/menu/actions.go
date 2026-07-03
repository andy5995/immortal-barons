package menu

import (
	"fmt"
	"sort"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// buy2 wraps a "prompt for quantity, apply, report" economy action.
func buy2(label string, unit func(*game.World) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *game.World) Result {
		p := w.Player()
		n := promptInt(s, fmt.Sprintf("%s — %d gold each. How many?", label, unit(w)))
		if n <= 0 {
			return Stay
		}
		if err := apply(w, p, n); err != nil {
			fail(s, err)
		} else {
			ok(s, "Done. Gold remaining: %d", p.Gold)
		}
		return Stay
	}
}

// money wraps a bank action that moves a gold amount.
func money(label string, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *game.World) Result {
		p := w.Player()
		n := promptInt(s, label+" how much gold?")
		if n <= 0 {
			return Stay
		}
		if err := apply(w, p, n); err != nil {
			fail(s, err)
		} else {
			ok(s, "Gold: %d   Bank: %d   Debt: %d", p.Gold, p.Bank, p.Debt)
		}
		return Stay
	}
}

func regularAttack(s session.Session, w *game.World) Result {
	targets := w.Rivals()
	if len(targets) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	fmt.Fprintf(s, "\n%sChoose a target:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, e := range targets {
		fmt.Fprintf(s, "  %d) %-16s Land %-5d Army %-7d\n", i+1, e.Name, e.Land, e.Army())
	}
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(targets) {
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", w.Attack(w.Player(), targets[i-1]))
	pause(s)
	return Stay
}

func empireStatus(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, p.Name, ansi.Reset)
	fmt.Fprintf(s, "  Gold ........ %d\n", p.Gold)
	fmt.Fprintf(s, "  Bank ........ %d\n", p.Bank)
	fmt.Fprintf(s, "  Debt ........ %d\n", p.Debt)
	fmt.Fprintf(s, "  Food ........ %d\n", p.Food)
	fmt.Fprintf(s, "  Land ........ %d regions\n", p.Land)
	fmt.Fprintf(s, "  People ...... %d\n", p.People)
	fmt.Fprintf(s, "  Troopers .... %d\n", p.Troopers)
	fmt.Fprintf(s, "  Jets ........ %d\n", p.Jets)
	fmt.Fprintf(s, "  Tanks ....... %d\n", p.Tanks)
	fmt.Fprintf(s, "  Tax rate .... %d%%\n", p.Tax)
	fmt.Fprintf(s, "  Net worth ... %d\n", w.NetWorth(p))
	pause(s)
	return Stay
}

func seeScores(s session.Session, w *game.World) Result {
	printScores(s, w)
	pause(s)
	return Stay
}

func printScores(s session.Session, w *game.World) {
	type row struct {
		e  *game.Empire
		nw int
	}
	rows := make([]row, 0, len(w.Empires))
	for _, e := range w.Empires {
		rows = append(rows, row{e, w.NetWorth(e)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].nw > rows[j].nw })

	fmt.Fprintf(s, "\n%s%-4s %-18s %-8s %-10s%s\n",
		ansi.FgBrightCyan, "Rank", "Empire", "Land", "Net Worth", ansi.Reset)
	for i, r := range rows {
		name := r.e.Name
		if !r.e.Alive {
			name += " (dead)"
		}
		mark := "  "
		if r.e.Human {
			mark = "->"
		}
		fmt.Fprintf(s, "%s%2d %-18s %-8d %-10d\n", mark, i+1, name, r.e.Land, r.nw)
	}
}

func nextTurn(s session.Session, w *game.World) Result {
	// Optional end-of-turn conveniences.
	p := w.Player()
	if w.DepositEndTurn && p.Gold > 0 {
		w.Deposit(p, p.Gold)
	}
	log := w.EndTurn()

	fmt.Fprintf(s, "\n%sDaily maintenance complete.%s\n", ansi.FgBrightCyan, ansi.Reset)
	for _, l := range log {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgYellow, l, ansi.Reset)
	}
	if w.GameOver() {
		showFinal(s, w)
		return Quit
	}
	pause(s)
	return Stay
}

func showFinal(s session.Session, w *game.World) {
	fmt.Fprintf(s, "\n%s=== The game has ended ===%s\n", ansi.FgBrightYellow, ansi.Reset)
	printScores(s, w)
	p := w.Player()
	switch {
	case p == nil || !p.Alive:
		fmt.Fprintf(s, "\n%sYour empire has fallen.%s\n", ansi.FgRed, ansi.Reset)
	case len(w.Rivals()) == 0:
		fmt.Fprintf(s, "\n%sYou have conquered the realm!%s\n", ansi.FgGreen, ansi.Reset)
	default:
		fmt.Fprintf(s, "\n%sThe turns are spent. Final standings above.%s\n", ansi.FgGreen, ansi.Reset)
	}
	pause(s)
}
