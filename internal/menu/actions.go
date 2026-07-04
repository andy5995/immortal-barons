package menu

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// buy2 wraps a "prompt for quantity, apply, report" economy action. The
// max offered is what the empire can currently afford at unit's price.
func buy2(label string, unit func(*game.World) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *game.World) Result {
		p := w.Player()
		price := unit(w)
		max := 0
		if price > 0 {
			max = p.Gold / price
		}
		n := promptSuggested(s, fmt.Sprintf("%s — %d gold each. How many?", label, price), 0, max)
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

// sellUnit2 wraps a "prompt for quantity, sell, report" unit-selling action.
// The max offered is what the empire currently owns.
func sellUnit2(label string, owned func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *game.World) Result {
		p := w.Player()
		max := owned(p)
		n := promptSuggested(s, fmt.Sprintf("%s (half price)?", label), 0, max)
		if n <= 0 {
			return Stay
		}
		if err := apply(w, p, n); err != nil {
			fail(s, err)
		} else {
			ok(s, "Sold %d. Gold: %d", n, p.Gold)
		}
		return Stay
	}
}

// buildHQ starts HeadQuarters construction for the acting empire.
func buildHQ(s session.Session, w *game.World) Result {
	p := w.Player()
	if err := w.StartHQ(p); err != nil {
		fail(s, err)
	} else {
		ok(s, "You have started work on your HeadQuarters.")
	}
	return Stay
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
	n := promptSuggested(s, "How many?", 0, w.MaxAffordableRegions(p))
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
	field := regionField(p, t-1)
	n := promptSuggested(s, "How many?", 0, *field)
	if n <= 0 {
		return Stay
	}
	if err := w.SellRegions(p, field, n); err != nil {
		fail(s, err)
	} else {
		ok(s, "Sold %d regions. Gold: %d", n, p.Gold)
	}
	return Stay
}

// money wraps a bank action that moves a gold amount, offering max as the
// largest sensible value for that action (e.g. Withdraw's max is p.Bank).
func money(label string, max func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *game.World) Result {
		p := w.Player()
		n := promptSuggested(s, label+" how much gold?", 0, max(p))
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

// investFunds prompts for a term (days) and amount, shows the expected
// return, and locks the gold via w.Invest.
func investFunds(s session.Session, w *game.World) Result {
	p := w.Player()
	fmt.Fprintf(s, "\nCurrent investment rate: %d%% per day.\n", w.InvestRate)
	days := promptInt(s, "Invest for how many days?")
	if days < game.MinInvestDays {
		days = game.MinInvestDays
	}
	amount := promptSuggested(s, "How much to invest?", 0, p.Gold)
	if amount <= 0 {
		return Stay
	}
	expected := game.ExpectedReturn(amount, w.InvestRate, days)
	fmt.Fprintf(s, "\n  Expected return: ~%d\n", expected)
	ret, err := w.Invest(p, amount, days)
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Invested %d for %d days; ~%d returns on day %d.", amount, days, ret, w.GameDay+days)
	}
	return Stay
}

// listInvestments shows the player's pending investments and any debt.
func listInvestments(s session.Session, w *game.World) Result {
	p := w.Player()
	if len(p.Investments) == 0 && p.Debt == 0 {
		fmt.Fprint(s, "\nYou have no active investments or loans.\n")
		pause(s)
		return Stay
	}
	if len(p.Investments) > 0 {
		fmt.Fprint(s, "\n  Amount      Return    Matures Day\n")
		for _, inv := range p.Investments {
			fmt.Fprintf(s, "  %-10d  %-8d  %d\n", inv.Amount, inv.Return, inv.MaturesDay)
		}
	}
	if p.Debt > 0 {
		fmt.Fprintf(s, "\n  Debt owed: %d\n", p.Debt)
	}
	pause(s)
	return Stay
}

// bankRates shows the current savings and investment rates.
func bankRates(s session.Session, w *game.World) Result {
	fmt.Fprintf(s, "\n  Savings interest: ~1%% per game day.\n  Investment rate: %d%% per day.\n", w.InvestRate)
	pause(s)
	return Stay
}

func buyFoodMarket(s session.Session, w *game.World) Result {
	p := w.Player()
	n := promptSuggested(s, "How much food to buy?", 0, p.Gold/game.FoodBuyPrice)
	if n <= 0 {
		return Stay
	}
	if err := w.BuyFoodMarket(p, n); err != nil {
		fail(s, err)
	} else {
		ok(s, "Bought %d food. Gold: %d", n, p.Gold)
	}
	return Stay
}

func sellFoodMarket(s session.Session, w *game.World) Result {
	p := w.Player()
	suggested := max(0, p.Food-w.FoodNeededNextTurn(p))
	n := promptSuggested(s, "How much food to sell?", suggested, p.Food)
	if n <= 0 {
		return Stay
	}
	if err := w.SellFood(p, n); err != nil {
		fail(s, err)
	} else {
		ok(s, "Sold %d food. Gold: %d", n, p.Gold)
	}
	return Stay
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
	fmt.Fprintf(s, "\n%sPirate factions (strength is random; fat ones just raided someone):%s\n", ansi.FgBrightCyan, ansi.Reset)
	fmt.Fprintf(s, "  %-3s %-11s %-7s %-4s %-8s %s\n", "#", "Faction", "Forces", "Rgn", "Gold", "Loot T/J/U/K/A")
	for i, p := range w.Pirates {
		fmt.Fprintf(s, "  %d) %-11s %-7d %-4d %-8d %d/%d/%d/%d/%d\n",
			i+1, p.Name, p.Forces, p.Land, p.Gold,
			p.LootTroopers, p.LootJets, p.LootTurrets, p.LootTanks, p.LootAgents)
	}
	f := promptInt(s, "Raid which faction (0 to cancel)?")
	if f < 1 || f > len(w.Pirates) {
		return Stay
	}
	p := w.Player()
	troopers := promptSuggested(s, "Commit how many Troopers?", 0, p.Troopers)
	jets := promptSuggested(s, "Commit how many Jets?", 0, p.Jets)
	tanks := promptSuggested(s, "Commit how many Tanks?", 0, p.Tanks)

	fmt.Fprintf(s, "\n%s\n", w.RaidFaction(p, f-1, troopers, jets, tanks))
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

// empireStatus prints a compact, multi-item status screen (BRE-style),
// packing values across lines instead of one dot-leader row each.
func empireStatus(s session.Session, w *game.World) Result {
	p := w.Player()
	c, r := ansi.FgBrightCyan, ansi.Reset
	fmt.Fprintf(s, "\n%s-*%s*-%s\n", c, p.Name, r)
	fmt.Fprintf(s, "Turns left: %d    Net worth: %d\n", p.TurnsLeft, w.NetWorth(p))
	fmt.Fprintf(s, "Gold: %d    Bank: %d    Debt: %d\n", p.Gold, p.Bank, p.Debt)
	fmt.Fprintf(s, "Population: %d (Tax %d%%)    Support: %d%%\n", p.People, p.Tax, p.Support)
	fmt.Fprintf(s, "Food: %d    HeadQuarters: %s    SDI: %d%%\n", p.Food, hqStatus(p), p.SDI)
	fmt.Fprintf(s, "Offense: %d    Defense: %d\n", p.Offense(), p.Defense())
	fmt.Fprintf(s, "Military: [%d Troopers] [%d Jets] [%d Turrets] [%d Tanks] [%d Bombers] [%d Carriers] [%d Agents]\n",
		p.Troopers, p.Jets, p.Turrets, p.Tanks, p.Bombers, p.Carriers, p.Agents)
	fmt.Fprintf(s, "Regions: %s\n", regionBreakdown(p))
	fmt.Fprintf(s, "Protection: %d turns left\n", p.Protection)
	pause(s)
	return Stay
}

func hqStatus(p *game.Empire) string {
	switch {
	case p.HQ == 0:
		return "None"
	case p.HQ >= 100:
		return "Complete"
	default:
		return fmt.Sprintf("%d%%", p.HQ)
	}
}

// regionBreakdown formats the non-zero region counts of p as bracketed
// items, e.g. "[40 Coastal] [25 Agricultural] [10 Urban]".
func regionBreakdown(p *game.Empire) string {
	var parts []string
	for i, name := range regionTypeNames {
		if n := *regionField(p, i); n > 0 {
			parts = append(parts, fmt.Sprintf("[%d %s]", n, name))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d regions", p.Land)
	}
	return strings.Join(parts, " ")
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

// recipients lists the living empires other than the player.
func recipients(w *game.World) []*game.Empire {
	var r []*game.Empire
	for _, e := range w.Empires {
		if e.Alive && e != w.Player() {
			r = append(r, e)
		}
	}
	return r
}

// recipientIndex maps an uppercased letter key ('A'..) to an index into a
// recipient list of length n, capped at 25 entries (A..Y). Returns -1 for
// keys with no match.
func recipientIndex(r rune, n int) int {
	idx := int(unicode.ToUpper(r) - 'A')
	if idx < 0 || idx >= n || idx >= 25 {
		return -1
	}
	return idx
}

// pickRecipient shows a lettered columns table (Id / Empire / Land / Score /
// Net Worth) and reads a SINGLE keypress: a letter A.. selects that empire;
// '0' cancels (returns nil, false). If allowAll, 'Z' returns (nil, true)
// meaning "all". Returns (empire, all).
func pickRecipient(s session.Session, w *game.World, prompt string, allowAll bool) (*game.Empire, bool) {
	rs := recipients(w)
	if len(rs) == 0 {
		ok(s, "There is no one to reach.")
		return nil, false
	}
	fmt.Fprintf(s, "\n%s%-4s %-20s %-6s %-6s %s%s\n", ansi.FgBrightCyan, "Id", "Empire", "Land", "Score", "Net Worth", ansi.Reset)
	for i, e := range rs {
		if i >= 25 { // A..Y
			break
		}
		fmt.Fprintf(s, "(%c) %-20s %-6d %-6d %d\n", 'A'+i, e.Name, e.Land, e.Land, w.NetWorth(e))
	}
	extra := ""
	if allowAll {
		extra = ", Z=All"
	}
	fmt.Fprintf(s, "\n%s(A-%c%s, 0=cancel) %s%s ", ansi.FgBrightWhite, 'A'+min(len(rs), 25)-1, extra, prompt, ansi.Reset)
	r, err := s.ReadKey()
	if err != nil {
		return nil, false
	}
	if allowAll && (r == 'z' || r == 'Z') {
		fmt.Fprint(s, "All\n")
		return nil, true
	}
	idx := recipientIndex(r, len(rs))
	if idx < 0 {
		fmt.Fprint(s, "\n")
		return nil, false
	}
	fmt.Fprintf(s, "%s\n", rs[idx].Name)
	return rs[idx], false
}

func sendMessage(s session.Session, w *game.World) Result {
	p := w.Player()
	to, all := pickRecipient(s, w, "Send to:", true)
	if !all && to == nil {
		return Stay
	}
	text := strings.TrimSpace(prompt(s, "Message:"))
	if text == "" {
		return Stay
	}
	if all {
		for _, e := range recipients(w) {
			w.SendMail(p, e, text)
		}
	} else {
		w.SendMail(p, to, text)
	}
	ok(s, "Message sent.")
	return Stay
}

func sendTradeDeal(s session.Session, w *game.World) Result {
	p := w.Player()
	to, _ := pickRecipient(s, w, "Trade with:", false)
	if to == nil {
		return Stay
	}
	amount := promptInt(s, "How much gold?")
	if amount <= 0 {
		return Stay
	}
	if err := w.SendGold(p, to, amount); err != nil {
		fail(s, err)
	} else {
		ok(s, "Sent %d gold to %s.", amount, to.Name)
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
// points spent on each unit type. Percentages need not sum to 100; the
// manufacturing split uses the raw percentages, so if they sum to less than
// 100 some production points go unused (v1 choice — no normalization).
func setIndustries(s session.Session, w *game.World) Result {
	p := w.Player()
	if p.Specialized != "" {
		ok(s, "Your industry is specialized in %s and can no longer be split.", p.Specialized)
		return Stay
	}
	fmt.Fprintf(s, "\n%sSet Industries — percentage of production spent on each unit:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, name := range prodTypeNames {
		cur := *prodField(p, i)
		n := promptSuggested(s, name+" %", cur, 100)
		*prodField(p, i) = n
	}
	ok(s, "Industry production percentages updated.")
	return Stay
}

// specializeIndustry lets the player concentrate all Industrial production
// into a single unit type. This is permanent, matching the original BRE's
// one-way specialization; once set it cannot be undone.
func specializeIndustry(s session.Session, w *game.World) Result {
	p := w.Player()
	if p.Specialized != "" {
		ok(s, "Your industry is already specialized in %s.", p.Specialized)
		return Stay
	}
	fmt.Fprintf(s, "\n%sSpecialize Industry — choose a unit type. This is PERMANENT:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, name := range prodTypeNames {
		fmt.Fprintf(s, "  %d) %s\n", i+1, name)
	}
	t := promptInt(s, "Specialize in which unit (0 to cancel)?")
	if t < 1 || t > len(prodTypeNames) {
		return Stay
	}
	p.Specialized = prodTypeNames[t-1]
	ok(s, "Your industry is now permanently specialized in %s.", p.Specialized)
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
