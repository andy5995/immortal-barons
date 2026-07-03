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
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	targets := w.Targets(w.Player())
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

// specialAttack shares the target-selection loop used by the nuclear,
// chemical, and biological attacks.
func specialAttack(s session.Session, w *game.World, label string, cost int, strike func(a, d *game.Empire) (string, error)) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	targets := w.Targets(w.Player())
	if len(targets) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s Attack — %d gold. Choose a target:%s\n", ansi.FgBrightCyan, label, cost, ansi.Reset)
	for i, e := range targets {
		fmt.Fprintf(s, "  %d) %-16s Land %-5d Army %-7d\n", i+1, e.Name, e.Land, e.Army())
	}
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(targets) {
		return Stay
	}
	report, err := strike(w.Player(), targets[i-1])
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

func nuclearAttack(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Nuclear", game.NukeCost, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Chemical", game.ChemCost, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Biological", game.BioCost, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
}

func attackPirates(s session.Session, w *game.World) Result {
	fmt.Fprintf(s, "\n%s\n", w.RaidPirates(w.Player()))
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
	fmt.Fprintf(s, "  Turrets ..... %d\n", p.Turrets)
	fmt.Fprintf(s, "  Tanks ....... %d\n", p.Tanks)
	fmt.Fprintf(s, "  Carriers .... %d\n", p.Carriers)
	fmt.Fprintf(s, "  Tax rate .... %d%%\n", p.Tax)
	fmt.Fprintf(s, "  Offense ..... %d\n", p.Offense())
	fmt.Fprintf(s, "  Defense ..... %d\n", p.Defense())
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
		if r.e == w.Player() {
			mark = "->"
		}
		fmt.Fprintf(s, "%s%2d %-18s %-8d %-10d\n", mark, i+1, name, r.e.Land, r.nw)
	}
}

func readMessages(s session.Session, w *game.World) Result {
	p := w.Player()
	if len(p.Mail) == 0 {
		ok(s, "You have no messages.")
		return Stay
	}
	fmt.Fprintf(s, "\n%sMessages:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for _, m := range p.Mail {
		fmt.Fprintf(s, "  %s\n", m)
	}
	p.Mail = nil
	if len(w.Bulletin) > 0 {
		fmt.Fprintf(s, "\n%sPlanetary Bulletin:%s\n", ansi.FgBrightCyan, ansi.Reset)
		for _, m := range w.Bulletin {
			fmt.Fprintf(s, "  %s\n", m)
		}
	}
	pause(s)
	return Stay
}

func sendMessage(s session.Session, w *game.World) Result {
	p := w.Player()
	var recipients []*game.Empire
	for _, e := range w.Empires {
		if e.Alive && e != p {
			recipients = append(recipients, e)
		}
	}
	if len(recipients) == 0 {
		ok(s, "There is no one to message.")
		return Stay
	}
	fmt.Fprintf(s, "\n%sChoose a recipient:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, e := range recipients {
		fmt.Fprintf(s, "  %d) %s\n", i+1, e.Name)
	}
	i := promptInt(s, "Message which empire (0 to cancel)?")
	if i < 1 || i > len(recipients) {
		return Stay
	}
	text := prompt(s, "Message:")
	if text == "" {
		return Stay
	}
	w.SendMail(p, recipients[i-1], text)
	ok(s, "Message sent.")
	return Stay
}

func planetaryPost(s session.Session, w *game.World) Result {
	text := prompt(s, "Post to the planet:")
	if text == "" {
		return Stay
	}
	w.PostBulletin(w.Player(), text)
	ok(s, "Posted.")
	return Stay
}

func nextTurn(s session.Session, w *game.World) Result {
	p := w.Player()
	if p.TurnsLeft <= 0 {
		ok(s, "You are out of turns for today. Come back after the next maintenance.")
		return Stay
	}
	w.PlayTurn(p, w.Today)
	ok(s, "Turn complete. Turns left: %d", p.TurnsLeft)
	return Stay
}
