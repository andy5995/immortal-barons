package menu

import (
	"fmt"
	"sort"
	"strings"

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

// regionTypeNames and regionTypeHints describe the 8 region types in the
// stable order RegionMix.fields()/e.Regions' own field order uses.
var regionTypeNames = []string{
	"Coastal", "Mountain", "Desert", "River",
	"Agricultural", "Urban", "Industrial", "Technology",
}

var regionTypeHints = []string{
	"gold", "gold, stable", "gold", "gold",
	"food", "people", "gold", "gold",
}

// regionField returns a pointer to the idx'th (0-based) field of p.Regions,
// in the same order as regionTypeNames.
func regionField(p *game.Empire, idx int) *int {
	fields := []*int{
		&p.Regions.Coastal, &p.Regions.Mountain, &p.Regions.Desert, &p.Regions.River,
		&p.Regions.Agricultural, &p.Regions.Urban, &p.Regions.Industrial, &p.Regions.Technology,
	}
	return fields[idx]
}

func printRegionTypes(s session.Session) {
	for i, name := range regionTypeNames {
		fmt.Fprintf(s, "  %d) %s (%s)\n", i+1, name, regionTypeHints[i])
	}
}

func buyLand(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%sBuy Regions — %d gold each. Choose a type:%s\n", ansi.FgBrightCyan, w.LandPrice(p), ansi.Reset)
	printRegionTypes(s)
	t := promptInt(s, "Region type (0 to cancel)?")
	if t < 1 || t > len(regionTypeNames) {
		return Stay
	}
	n := promptInt(s, "How many?")
	if n <= 0 {
		return Stay
	}
	if err := w.BuyRegions(p, regionField(p, t-1), n); err != nil {
		fail(s, err)
	} else {
		ok(s, "Bought %d regions. Gold: %d", n, p.Gold)
	}
	return Stay
}

func sellLand(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%sSell Regions — choose a type:%s\n", ansi.FgBrightCyan, ansi.Reset)
	printRegionTypes(s)
	t := promptInt(s, "Region type (0 to cancel)?")
	if t < 1 || t > len(regionTypeNames) {
		return Stay
	}
	n := promptInt(s, "How many?")
	if n <= 0 {
		return Stay
	}
	if err := w.SellRegions(p, regionField(p, t-1), n); err != nil {
		fail(s, err)
	} else {
		ok(s, "Sold %d regions. Gold: %d", n, p.Gold)
	}
	return Stay
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
	if cost > 0 {
		fmt.Fprintf(s, "\n%s%s — %d gold. Choose a target:%s\n", ansi.FgBrightCyan, label, cost, ansi.Reset)
	} else {
		fmt.Fprintf(s, "\n%s%s — choose a target:%s\n", ansi.FgBrightCyan, label, ansi.Reset)
	}
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
	return specialAttack(s, w, "Nuclear Attack", game.NukeCost, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Chemical Attack", game.ChemCost, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Biological Attack", game.BioCost, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
}

func sendSpy(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Send Spy", 0, func(a, d *game.Empire) (string, error) { return w.SendSpy(a, d) })
}

func specialOps(s session.Session, w *game.World) Result {
	return specialAttack(s, w, "Special Operations", 0, func(a, d *game.Empire) (string, error) { return w.Sabotage(a, d) })
}

func attackPirates(s session.Session, w *game.World) Result {
	fmt.Fprintf(s, "\n%s\n", w.RaidPirates(w.Player()))
	pause(s)
	return Stay
}

func sdiProgram(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%sSDI Program — current defense: %d%%%s\n", ansi.FgBrightCyan, p.SDI, ansi.Reset)
	gold := promptInt(s, "Fund SDI — gold to spend (10000 per +1%%, max 75%%)?")
	if gold <= 0 {
		return Stay
	}
	level, err := w.FundSDI(p, gold)
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "SDI is now %d%%.", level)
	return Stay
}

func gooieKablooie(s session.Session, w *game.World) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	answer := promptInt(s, fmt.Sprintf("A Gooie Kablooie costs %d gold. Launch? (1 = yes)", game.GooieCost))
	if answer != 1 {
		return Stay
	}
	report, err := w.GooieKablooie(w.Player())
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
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
	fmt.Fprintf(s, "  Land ........ %d regions (%s)\n", p.Land, regionBreakdown(p))
	fmt.Fprintf(s, "  People ...... %d\n", p.People)
	fmt.Fprintf(s, "  Troopers .... %d\n", p.Troopers)
	fmt.Fprintf(s, "  Jets ........ %d\n", p.Jets)
	fmt.Fprintf(s, "  Turrets ..... %d\n", p.Turrets)
	fmt.Fprintf(s, "  Tanks ....... %d\n", p.Tanks)
	fmt.Fprintf(s, "  Carriers .... %d\n", p.Carriers)
	fmt.Fprintf(s, "  Agents ...... %d\n", p.Agents)
	fmt.Fprintf(s, "  Tax rate .... %d%%\n", p.Tax)
	fmt.Fprintf(s, "  SDI ......... %d%%\n", p.SDI)
	fmt.Fprintf(s, "  Offense ..... %d\n", p.Offense())
	fmt.Fprintf(s, "  Defense ..... %d\n", p.Defense())
	fmt.Fprintf(s, "  Net worth ... %d\n", w.NetWorth(p))
	pause(s)
	return Stay
}

// regionBreakdown formats the non-zero region counts of p, e.g.
// "Coastal 40, Agricultural 25, Urban 10".
func regionBreakdown(p *game.Empire) string {
	var parts []string
	for i, name := range regionTypeNames {
		if n := *regionField(p, i); n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", name, n))
		}
	}
	return strings.Join(parts, ", ")
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
	if w.LastMaster != "" {
		fmt.Fprintf(s, "\nLast Planetary Master: %s\n", w.LastMaster)
	}
}

// interbbsScores displays scores imported from other boards via inter-BBS
// packets (see internal/ibbs). v1 covers only score/news sharing.
func interbbsScores(s session.Session, w *game.World) Result {
	if len(w.RemoteBoards) == 0 {
		ok(s, "No inter-BBS scores have been imported yet.")
		return Stay
	}
	for _, b := range w.RemoteBoards {
		fmt.Fprintf(s, "\n%sBoard: %s (%s)%s\n", ansi.FgBrightCyan, b.BoardID, b.Date, ansi.Reset)
		scores := append([]game.RemoteScore(nil), b.Scores...)
		sort.Slice(scores, func(i, j int) bool { return scores[i].NetWorth > scores[j].NetWorth })
		for _, sc := range scores {
			fmt.Fprintf(s, "  %-18s %-8d %-10d\n", sc.Empire, sc.Land, sc.NetWorth)
		}
	}
	pause(s)
	return Stay
}

func readMessages(s session.Session, w *game.World) Result {
	p := w.Player()
	if len(p.Mail) == 0 {
		fmt.Fprint(s, "\nYou have no messages.\n")
	} else {
		fmt.Fprintf(s, "\n%sYour messages:%s\n", ansi.FgBrightCyan, ansi.Reset)
		for _, m := range p.Mail {
			fmt.Fprintf(s, "  %s\n", m)
		}
		p.Mail = nil
	}
	if len(w.Bulletin) > 0 {
		fmt.Fprintf(s, "\n%sPlanetary Bulletin:%s\n", ansi.FgBrightCyan, ansi.Reset)
		for _, b := range w.Bulletin {
			fmt.Fprintf(s, "  %s\n", b)
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
	text := strings.TrimSpace(prompt(s, "Message:"))
	if text == "" {
		return Stay
	}
	w.SendMail(p, recipients[i-1], text)
	ok(s, "Message sent.")
	return Stay
}

func sendTradeDeal(s session.Session, w *game.World) Result {
	p := w.Player()
	var recipients []*game.Empire
	for _, e := range w.Empires {
		if e.Alive && e != p {
			recipients = append(recipients, e)
		}
	}
	if len(recipients) == 0 {
		ok(s, "There is no one to trade with.")
		return Stay
	}
	fmt.Fprintf(s, "\n%sChoose a recipient:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, e := range recipients {
		fmt.Fprintf(s, "  %d) %s\n", i+1, e.Name)
	}
	i := promptInt(s, "Trade with which empire (0 to cancel)?")
	if i < 1 || i > len(recipients) {
		return Stay
	}
	recipient := recipients[i-1]
	amount := promptInt(s, "How much gold?")
	if amount <= 0 {
		return Stay
	}
	if err := w.SendGold(p, recipient, amount); err != nil {
		fail(s, err)
	} else {
		ok(s, "Sent %d gold to %s.", amount, recipient.Name)
	}
	return Stay
}

func planetaryPost(s session.Session, w *game.World) Result {
	text := strings.TrimSpace(prompt(s, "Post to the planet:"))
	if text == "" {
		return Stay
	}
	w.PostBulletin(w.Player(), text)
	ok(s, "Posted.")
	return Stay
}

func modifyDiplomacy(s session.Session, w *game.World) Result {
	p := w.Player()
	var others []*game.Empire
	for _, e := range w.Empires {
		if e.Alive && e != p {
			others = append(others, e)
		}
	}
	if len(others) == 0 {
		ok(s, "There is no one to negotiate with.")
		return Stay
	}
	fmt.Fprintf(s, "\n%sChoose an empire:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, e := range others {
		status := ""
		switch {
		case w.AreAllied(p, e):
			status = " (allied)"
		case inList(p.AllianceOffers, e.Name):
			status = " (offered you)"
		case inList(e.AllianceOffers, p.Name):
			status = " (proposal sent)"
		}
		fmt.Fprintf(s, "  %d) %s%s\n", i+1, e.Name, status)
	}
	i := promptInt(s, "Negotiate with which empire (0 to cancel)?")
	if i < 1 || i > len(others) {
		return Stay
	}
	e := others[i-1]
	switch {
	case w.AreAllied(p, e):
		w.BreakAlliance(p, e)
		ok(s, "Alliance with %s broken.", e.Name)
	case inList(p.AllianceOffers, e.Name):
		w.AcceptAlliance(p, e.Name)
		ok(s, "You are now allied with %s.", e.Name)
	default:
		w.ProposeAlliance(p, e)
		ok(s, "Alliance proposed to %s.", e.Name)
	}
	return Stay
}

func inList(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
}

func viewDiplomacy(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%sYour allies:%s\n", ansi.FgBrightCyan, ansi.Reset)
	found := false
	for _, k := range w.Alliances {
		names := strings.SplitN(k, "\x00", 2)
		if len(names) != 2 {
			continue
		}
		var other string
		switch p.Name {
		case names[0]:
			other = names[1]
		case names[1]:
			other = names[0]
		default:
			continue
		}
		fmt.Fprintf(s, "  %s\n", other)
		found = true
	}
	if !found {
		fmt.Fprint(s, "  (none)\n")
	}
	fmt.Fprintf(s, "\n%sPending offers received:%s\n", ansi.FgBrightCyan, ansi.Reset)
	if len(p.AllianceOffers) == 0 {
		fmt.Fprint(s, "  (none)\n")
	} else {
		for _, o := range p.AllianceOffers {
			fmt.Fprintf(s, "  %s\n", o)
		}
	}
	pause(s)
	return Stay
}

func setTaxRate(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%sCurrent tax rate: %d%%%s\n", ansi.FgBrightCyan, p.Tax, ansi.Reset)
	rate := promptInt(s, "New tax rate (0-100)?")
	if rate < 0 || rate > 100 {
		fail(s, fmt.Errorf("tax rate must be between 0 and 100"))
		return Stay
	}
	p.Tax = rate
	ok(s, "Tax rate set to %d%%.", rate)
	return Stay
}
