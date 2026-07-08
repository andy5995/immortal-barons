package menu

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// buy2 wraps a "prompt for quantity, apply, report" economy action. The
// max offered is what the empire can currently afford at unit's price.
func buy2(label string, military bool, unit func(*ctx) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		// The league's Buy Military knob can forbid buying army units on the
		// open market (players must then build them through industry). Limited
		// mode's daily market pool isn't built yet, so it behaves like Yes.
		if military && w.Config.BuyMilitary == game.BuyNo {
			fail(s, fmt.Errorf("Buying military units is disabled in this league."))
			return Stay
		}
		price := unit(w)
		max := 0
		if price > 0 {
			max = p.Gold / price
		}
		n := promptSuggested(s, fmt.Sprintf("%s — %d gold each. How many?", label, price), 0, max)
		if n <= 0 {
			return Stay
		}
		var err error
		w.With(func() { err = apply(w.World, p, n) }) // apply re-checks gold atomically
		if err != nil {
			fail(s, err)
		} else {
			// No pause: the Spending menu (NoClear) redraws right after with
			// updated Owned counts, so the confirmation stays visible above it
			// instead of forcing an extra keypress (BRE-style). Gold isn't
			// repeated here — it's in the Spending menu's status footer.
			fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
				fmt.Sprintf(tr(s, "%d %s purchased."), n, label), ansi.Reset)
		}
		return Stay
	}
}

// sellUnit2 wraps a "prompt for quantity, sell, report" unit-selling action.
// The max offered is what the empire currently owns.
func sellUnit2(label string, owned func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		max := owned(p)
		n := promptSuggested(s, fmt.Sprintf("%s (half price)?", label), 0, max)
		if n <= 0 {
			return Stay
		}
		var err error
		w.With(func() { err = apply(w.World, p, n) }) // apply re-checks stock atomically
		if err != nil {
			fail(s, err)
		} else {
			ok(s, "Sold %d. Gold: %d", n, p.Gold)
		}
		return Stay
	}
}

// buildHQ starts HeadQuarters construction for the acting empire.
func buildHQ(s session.Session, w *ctx) Result {
	p := w.Player()
	var err error
	w.With(func() { err = w.World.StartHQ(p) })
	if err != nil {
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

// regionTypeKeys are the single-letter selection keys (BRE style), in the same
// order as regionTypeNames.
var regionTypeKeys = []byte{'C', 'M', 'D', 'R', 'A', 'U', 'I', 'T'}

// regionField returns a pointer to the idx'th (0-based) field of p.Regions,
// in the same order as regionTypeNames.
func regionField(p *game.Empire, idx int) *int {
	fields := []*int{
		&p.Regions.Coastal, &p.Regions.Mountain, &p.Regions.Desert, &p.Regions.River,
		&p.Regions.Agricultural, &p.Regions.Urban, &p.Regions.Industrial, &p.Regions.Technology,
	}
	return fields[idx]
}

// printRegionTable renders the BRE-style region picker: a Key / Name / Produces
// / Owned table, colored (magenta keys, yellow names) so buy and drop share one
// look.
func printRegionTable(s session.Session, p *game.Empire) {
	fmt.Fprintf(s, "%s%-5s%-15s%-15s%s%s\n", ansi.FgBrightWhite, tr(s, "Key"), tr(s, "Name"), tr(s, "Produces"), tr(s, "Owned"), ansi.Reset)
	for i, name := range regionTypeNames {
		fmt.Fprintf(s, " %s(%c)%s %s%-14s%s %-14s %5d\n",
			ansi.FgBrightMagenta, regionTypeKeys[i], ansi.Reset,
			ansi.FgBrightYellow, name, ansi.Reset,
			regionTypeHints[i], *regionField(p, i))
	}
}

// promptRegionType reads a single-letter region choice (case-insensitive),
// returning its 0-based index or -1 to cancel.
func promptRegionType(s session.Session) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your choice? (0 to cancel)"), ansi.Reset)
	for {
		r, err := s.ReadKey()
		if err != nil {
			return -1
		}
		if r == '0' {
			fmt.Fprint(s, "0\n")
			return -1
		}
		u := byte(unicode.ToUpper(r))
		for i, k := range regionTypeKeys {
			if k == u {
				fmt.Fprintf(s, "%c\n", u) // echo the single keypress; no Enter needed
				return i
			}
		}
		// invalid key — ignore and wait for a valid region letter or 0
	}
}

// advisorsChoice is returned by promptBuyRegionType when the player picks
// the "(*) Advisors" entry instead of a region type.
const (
	advisorsChoice  = -2
	redisplayChoice = -3
)

// promptBuyRegionType is promptRegionType plus a '*' key for Advisors (BRE's
// Buy Regions screen lists "(*) Advisors" at the bottom of the region list) and
// a '?' key to redisplay the region list (the list is only drawn on entry, then
// on demand, so repeat purchases don't rescroll it).
func promptBuyRegionType(s session.Session) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your choice? (? to list, 0 to cancel)"), ansi.Reset)
	for {
		r, err := s.ReadKey()
		if err != nil {
			return -1
		}
		if r == '0' {
			fmt.Fprint(s, "0\n")
			return -1
		}
		if r == '?' {
			fmt.Fprint(s, "?\n")
			return redisplayChoice
		}
		if r == '*' {
			fmt.Fprint(s, "*\n")
			return advisorsChoice
		}
		u := byte(unicode.ToUpper(r))
		for i, k := range regionTypeKeys {
			if k == u {
				fmt.Fprintf(s, "%c\n", u) // echo the single keypress; no Enter needed
				return i
			}
		}
		// invalid key — ignore and wait for a valid region letter, '*', or 0
	}
}

// buyLand is the Buy Regions action. It loops the region-type picker so a
// player can buy several region types in one visit; the loop only ends when
// they quit (0) or run out of input. Picking "(*) Advisors" shows the
// Advisors screen and returns to the region list instead of leaving.
func buyLand(s session.Session, w *ctx) Result {
	p := w.Player()
	showMenu := func() {
		fmt.Fprintf(s, "\n%s"+tr(s, "Buy Regions — %d gold each.")+"%s\n", ansi.FgBrightCyan, w.LandPrice(p), ansi.Reset)
		fmt.Fprintf(s, "%s\n", tr(s, "Note: Region prices rise as you expand, so the price shown is only\n      the cost of the first region you buy."))
		fmt.Fprintf(s, tr(s, "You can afford %s%d%s regions.")+"\n\n", ansi.FgBrightCyan, w.MaxAffordableRegions(p), ansi.Reset)
		printRegionTable(s, p)
		fmt.Fprintf(s, " %s(*)%s %s%s%s\n", ansi.FgBrightMagenta, ansi.Reset, ansi.FgBrightYellow, tr(s, "Advisors"), ansi.Reset)
	}
	showMenu()
	for {
		switch t := promptBuyRegionType(s); {
		case t == redisplayChoice:
			showMenu()
		case t == advisorsChoice:
			renderAdvisors(s, w)
			pause(s)
			showMenu()
		case t < 0:
			return Stay
		default:
			n := promptSuggested(s, fmt.Sprintf("Buy how many %s regions?", regionTypeNames[t]), 0, w.MaxAffordableRegions(p))
			if n <= 0 {
				continue
			}
			var err error
			var gold int
			w.With(func() {
				err = w.World.BuyRegions(p, regionField(p, t), n)
				gold = p.Gold
			})
			if err != nil {
				// No pause: the message stays above the next prompt; the player
				// keeps buying without the region list rescrolling each time.
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgRed, tr(s, err.Error()), ansi.Reset)
			} else {
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
					fmt.Sprintf(tr(s, "%d %s regions purchased. Gold: %d"), n, regionTypeNames[t], gold), ansi.Reset)
			}
		}
	}
}

func sellLand(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "NOTE: You cannot sell Regions, only drop them..."), ansi.Reset)
	printRegionTable(s, p)
	t := promptRegionType(s)
	if t < 0 {
		return Stay
	}
	field := regionField(p, t)
	n := promptSuggested(s, fmt.Sprintf("Drop how many %s regions?", regionTypeNames[t]), 0, *field)
	if n <= 0 {
		return Stay
	}
	var err error
	w.With(func() { err = w.World.DropRegions(p, field, n) })
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "%d %s regions dropped. You now hold %d land.", n, regionTypeNames[t], p.Land)
	}
	return Stay
}

// money wraps a bank action that moves a gold amount, offering max as the
// largest sensible value for that action (e.g. Withdraw's max is p.Bank).
func money(label string, max func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		n := promptSuggested(s, label+" how much gold?", 0, max(p))
		if n <= 0 {
			return Stay
		}
		var err error
		w.With(func() { err = apply(w.World, p, n) })
		if err != nil {
			fail(s, err)
		} else {
			ok(s, "Gold: %d   Bank: %d   Debt: %d", p.Gold, p.Bank, p.Debt)
		}
		return Stay
	}
}

// investFunds prompts for a term (days) and amount, shows the expected
// return, and locks the gold via w.Invest.
func investFunds(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n"+tr(s, "Current investment rate: %d%% per day.")+"\n", w.InvestRate)
	days := promptInt(s, "Invest for how many days?")
	if days < game.MinInvestDays {
		days = game.MinInvestDays
	}
	amount := promptSuggested(s, "How much to invest?", 0, p.Gold)
	if amount <= 0 {
		return Stay
	}
	expected := game.ExpectedReturn(amount, w.InvestRate, days)
	fmt.Fprintf(s, "\n  "+tr(s, "Expected return: ~%d")+"\n", expected)
	var ret int
	var err error
	w.With(func() { ret, err = w.World.Invest(p, amount, days) })
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Invested %d for %d days; ~%d returns on day %d.", amount, days, ret, w.GameDay+days)
	}
	return Stay
}

// abdicate deletes the player's empire from the game (BRE.DOC: "immediately
// delete your empire from the game so you may start over the next day"). It
// is irreversible, so the player must retype their realm name to confirm.
// Removing the empire and quitting is enough: play.go persists the world on
// exit, and the caller's next visit finds no empire and onboards a fresh one.
func abdicate(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s"+tr(s, "Abdicating deletes %s permanently. This cannot be undone.")+"%s\n",
		ansi.FgBrightRed, p.Name, ansi.Reset)
	typed := prompt(s, fmt.Sprintf(tr(s, "Type your realm name (%s) to confirm, or anything else to cancel"), p.Name))
	if strings.TrimSpace(typed) != p.Name {
		fmt.Fprintf(s, "\n%s\n", tr(s, "Abdication cancelled."))
		pause(s)
		return Stay
	}
	w.With(func() { w.World.RemoveEmpire(p) })
	w.active = nil
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "Your empire is no more. Fare thee well."), ansi.Reset)
	pause(s)
	return Quit
}

// writeMacros is BRE's Macro Editor: it lists the player's saved macros and
// lets them set or clear one, keyed by a letter A-Z. In game the macro replays
// when the player presses Ctrl-<letter> (see session.MacroExpander).
// macroKeys is BRE's fixed set of macro slots, each triggered in game by
// Ctrl-<letter> (see the Macro Editor, Image #9).
const macroKeys = "DEFRIOKL"

// writeMacros is BRE's Macro Editor: it lists the eight macro slots, lets the
// player pick one, then records keystrokes live until the player presses that
// same Ctrl-<letter> to end the edit.
func writeMacros(s session.Session, w *ctx) Result {
	p := w.Player()
	if p.Macros == nil {
		p.Macros = map[string]string{}
	}
	fmt.Fprintf(s, "\n%s%s%s\n\n", ansi.FgBrightCyan, tr(s, "Macro Editor"), ansi.Reset)
	for _, k := range macroKeys {
		val := p.Macros[string(k)]
		if val == "" {
			val = tr(s, "None")
		}
		fmt.Fprintf(s, "Ctrl-%c: %s%s%s\n", k, ansi.FgGreen, val, ansi.Reset)
	}

	fmt.Fprintf(s, "\n%s ", tr(s, "Edit which macro [D,E,F,R,I,O,K,L]?"))
	r, err := s.ReadKey()
	if err != nil {
		return Stay
	}
	letter := byte(r)
	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}
	if !strings.ContainsRune(macroKeys, rune(letter)) {
		fmt.Fprint(s, "\n")
		return Stay
	}
	fmt.Fprintf(s, "%c\n", letter)

	// Clear the slot first so pressing its own Ctrl-<letter> ends the edit
	// (passes through the expander) instead of replaying the old macro.
	w.With(func() { delete(p.Macros, string(letter)) })
	ctrl := rune(letter - 'A' + 1)
	fmt.Fprintf(s, "\n"+tr(s, "Editing Macro Ctrl-%c    Press Ctrl-%c to end edit.")+"\n", letter, letter)
	var seq []rune
	for {
		k, err := s.ReadKey()
		if err != nil || k == ctrl {
			break
		}
		seq = append(seq, k)
		if k >= 32 { // echo printable keys as they are recorded
			fmt.Fprintf(s, "%c", k)
		}
	}
	if len(seq) > 0 {
		w.With(func() { p.Macros[string(letter)] = string(seq) })
		ok(s, "Macro Ctrl-%c saved.", letter)
	} else {
		ok(s, "Macro Ctrl-%c cleared.", letter)
	}
	return Stay
}

// listInvestments shows the player's pending investments and any debt.
func listInvestments(s session.Session, w *ctx) Result {
	p := w.Player()
	var investments []game.Investment
	var debt int
	w.With(func() {
		investments = append([]game.Investment(nil), p.Investments...)
		debt = p.Debt
	})
	if len(investments) == 0 && debt == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "You have no active investments or loans."))
		pause(s)
		return Stay
	}
	if len(investments) > 0 {
		fmt.Fprintf(s, "\n  %s\n", tr(s, "Amount      Return    Matures Day"))
		for _, inv := range investments {
			fmt.Fprintf(s, "  %-10d  %-8d  %d\n", inv.Amount, inv.Return, inv.MaturesDay)
		}
	}
	if debt > 0 {
		fmt.Fprintf(s, "\n  "+tr(s, "Debt owed: %d")+"\n", debt)
	}
	pause(s)
	return Stay
}

// bankRates shows the current savings and investment rates.
func bankRates(s session.Session, w *ctx) Result {
	fmt.Fprintf(s, "\n  "+tr(s, "Savings interest: ~1%% per game day.")+"\n  "+tr(s, "Investment rate: %d%% per day.")+"\n", w.InvestRate)
	pause(s)
	return Stay
}

func buyFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	n := promptSuggested(s, "How much food to buy?", 0, p.Gold/game.FoodBuyPrice)
	if n <= 0 {
		return Stay
	}
	var err error
	w.With(func() { err = w.World.BuyFoodMarket(p, n) })
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Bought %d food. Gold: %d", n, p.Gold)
	}
	return Stay
}

func sellFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	suggested := max(0, p.Food-w.FoodNeededNextTurn(p))
	n := promptSuggested(s, "How much food to sell?", suggested, p.Food)
	if n <= 0 {
		return Stay
	}
	var err error
	w.With(func() { err = w.World.SellFood(p, n) })
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Sold %d food. Gold: %d", n, p.Gold)
	}
	return Stay
}

// targetRow snapshots the identity plus displayed fields of one attackable
// empire, taken under the world lock so the picker list can be rendered and
// prompted over safely (w.Targets ranges the shared w.Empires slice).
type targetRow struct {
	e          *game.Empire
	name       string
	land, army int
}

// snapshotTargets takes w.Targets(w.Player()) under the lock, copying the
// display fields the picker needs. The *game.Empire pointers stay valid
// afterward (empires are never reallocated), but re-validate with
// stillTarget inside the resolving w.With before acting on a choice — the
// target may have died or gone under protection in the meantime.
func snapshotTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.With(func() {
		for _, e := range w.Targets(w.Player()) {
			rows = append(rows, targetRow{e, e.Name, e.Land, e.Army()})
		}
	})
	return rows
}

// stillTarget reports whether target is still among w.Player()'s valid
// targets. Call only from inside a w.With block.
func stillTarget(w *ctx, target *game.Empire) bool {
	for _, t := range w.Targets(w.Player()) {
		if t == target {
			return true
		}
	}
	return false
}

func regularAttack(s session.Session, w *ctx) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose a target:"), ansi.Reset)
	printTargetRows(s, rows)
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	var report string
	var err error
	w.With(func() {
		if !stillTarget(w, target) {
			err = fmt.Errorf("that empire is no longer a valid target")
			return
		}
		report = w.World.Attack(w.Player(), target)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

// printTargetRows lists attackable empires with their Land and Army columns.
func printTargetRows(s session.Session, rows []targetRow) {
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %-16s %s %-5d %s %-7d\n", i+1, r.name, tr(s, "Land"), r.land, tr(s, "Army"), r.army)
	}
}

// specialAttack shares the target-selection loop used by the nuclear,
// chemical, and biological attacks.
func specialAttack(s session.Session, w *ctx, label string, cost int, strike func(a, d *game.Empire) (string, error)) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	if cost > 0 {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — %d gold. Choose a target:")+"%s\n", ansi.FgBrightCyan, label, cost, ansi.Reset)
	} else {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — choose a target:")+"%s\n", ansi.FgBrightCyan, label, ansi.Reset)
	}
	printTargetRows(s, rows)
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	var report string
	var err error
	w.With(func() {
		if !stillTarget(w, target) {
			err = fmt.Errorf("that empire is no longer a valid target")
			return
		}
		report, err = strike(w.Player(), target)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

func nuclearAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Nuclear Attack", game.NukeCost, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Chemical Attack", game.ChemCost, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Biological Attack", game.BioCost, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
}

func sendSpy(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Send Spy", 0, func(a, d *game.Empire) (string, error) { return w.SendSpy(a, d) })
}

func supportDissensions(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Support Dissensions", 0, func(a, d *game.Empire) (string, error) { return w.SupportDissensions(a, d) })
}

func demoralizeForces(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Demoralize Forces", 0, func(a, d *game.Empire) (string, error) { return w.DemoralizeForces(a, d) })
}

func setUp(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Set Up", 0, func(a, d *game.Empire) (string, error) { return w.SetUp(a, d) })
}

func exposeEnemyOps(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Expose Enemy Ops", 0, func(a, d *game.Empire) (string, error) { return w.ExposeEnemyOps(a, d) })
}

func stirRevolts(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Stir Revolts", 0, func(a, d *game.Empire) (string, error) { return w.StirRevolts(a, d) })
}

// bombingAttack wraps specialAttack with BRE's Bomb Enemy Targets 500-Bomber
// payload requirement (BRE.OVR: "All missiles and bombs require 500 Bombers
// to deliver their payloads").
func bombingAttack(s session.Session, w *ctx, label string, cost int, strike func(a, d *game.Empire) (string, error)) Result {
	if w.Player().Bombers < game.BombingBombersRequired {
		fail(s, fmt.Errorf("you need at least %d Bombers to deliver a payload", game.BombingBombersRequired))
		return Stay
	}
	return specialAttack(s, w, label, cost, strike)
}

func bombFoodMarket(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Food Market", 0, func(a, d *game.Empire) (string, error) { return w.BombFood(a, d) })
}

func bombTradingMarket(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Trading Market", 0, func(a, d *game.Empire) (string, error) { return w.BombTradingMarket(a, d) })
}

func bombTradeRoutes(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Trade Routes", 0, func(a, d *game.Empire) (string, error) { return w.BombTradeRoutes(a, d) })
}

func undermineInvestments(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Undermine Investments", 0, func(a, d *game.Empire) (string, error) { return w.UndermineInvestments(a, d) })
}

func nuclearAssault(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Nuclear Assault", game.NukeCost, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalBombing(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Chemical Bombing", game.ChemCost, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func slappenheimerStrike(s session.Session, w *ctx) Result {
	var mode game.SlappenheimerMode
	w.With(func() { mode = w.Config.SlappenheimerHandling })
	if mode == game.SlappenheimerNone {
		ok(s, "The R5-Slappenheimer is disabled.")
		return Stay
	}
	// Under User Select handling the player dials the missile in (0-10). The
	// dial is BRE's bluff — it changes nothing about the outcome — but we still
	// prompt for it to keep the original's feel.
	if mode == game.SlappenheimerUserSelect {
		promptInt(s, "Set the R5-Slappenheimer dial (0-10)")
	}
	return bombingAttack(s, w, "R5-Slappenheimer", 0, func(a, d *game.Empire) (string, error) { return w.SlappenheimerStrike(a, d) })
}

// renderAdvisors prints the Advisors screen: contextual advice based on the
// empire's current state — the sort of nudges the original's advisors
// offered (military readiness, food, support/morale, taxes, treasury,
// technology). Split from visitAdvisors so tests can render without a pause.
func renderAdvisors(s session.Session, w *ctx) {
	// Snapshot so the report reflects one consistent moment even if another
	// session mutates the world mid-render (same reasoning as Empire Status).
	var p game.Empire
	w.With(func() { p = *w.Player() })

	titleBar(s, tr(s, "Visit Advisors"))
	var tips []string
	switch {
	case p.HQ == 0:
		tips = append(tips, tr(s, "We have no HeadQuarters, Sire. Building one would strengthen our tanks."))
	case p.HQ < 100:
		tips = append(tips, tr(s, "Our HeadQuarters is still under construction."))
	}
	if p.Carriers*100 < p.Jets {
		tips = append(tips, tr(s, "We have more jets than our carriers can carry into battle. Build more carriers."))
	}
	if p.Morale < 50 {
		tips = append(tips, tr(s, "Morale is low among our troops. Desertion is a real risk before our next battle."))
	}
	if p.Food < w.FoodNeededNextTurn(&p) {
		tips = append(tips, tr(s, "Our food will not last the turn. Buy or grow more."))
	}
	if p.Support < 50 {
		tips = append(tips, tr(s, "The people grow restless. Lower taxes or spend on their support."))
	}
	// Riots are possible above RiotTaxFloor(10), but the chance (tax^2/10000)
	// is trivial near the floor; only advise once the risk is worth acting on.
	if p.Tax > 20 {
		tips = append(tips, tr(s, "Taxes are set high enough to risk riots. Consider lowering them."))
	}
	if p.TechFactor() == 0 {
		tips = append(tips, tr(s, "We have no Technology infrastructure. Such regions would sharpen our efficiency."))
	}
	if p.Gold <= 0 && p.Bank <= 0 {
		tips = append(tips, tr(s, "Our treasury is empty, Sire. We should raise gold before it costs us dearly."))
	}
	if p.Debt > 0 {
		tips = append(tips, tr(s, "We carry debt that grows each turn. Repay it soon."))
	}
	if p.Agents == 0 {
		tips = append(tips, tr(s, "We have no covert agents. Recruit some for spying and sabotage."))
	}
	if len(tips) == 0 {
		tips = append(tips, tr(s, "The realm is in good order, Sire. Press the attack."))
	}
	for _, t := range tips {
		fmt.Fprintf(s, "  - %s\n", t)
	}
}

// visitAdvisors is the System menu's "Visit Advisors" action.
func visitAdvisors(s session.Session, w *ctx) Result {
	renderAdvisors(s, w)
	pause(s)
	return Stay
}

// about shows a short project panel: name, version, website, and the BRE
// heritage note (reachable from both the Game and System menus, #66).
func about(s session.Session, w *ctx) Result {
	titleBar(s, tr(s, "About"))
	fmt.Fprintf(s, "  %s\n", "Immortal Barons v"+game.Version)
	fmt.Fprintf(s, "  %s\n", "https://andy5995.github.io/immortal-barons/")
	fmt.Fprintf(s, "  %s\n", tr(s, "An independent tribute to Barren Realms Elite (BRE), created by Mehul Patel and later maintained by John Dailey. No original BRE code, text, or art is used."))
	pause(s)
	return Stay
}

// gameSetup shows the current game rules (read-only; the sysop edits them from
// the Coordinator menu's Configuration Editor).
func gameSetup(s session.Session, w *ctx) Result {
	c := w.Config
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Game Rules"), ansi.Reset)
	fmt.Fprintf(s, "  "+tr(s, "Turns per day:      %d")+"\n", c.TurnsPerDay)
	fmt.Fprintf(s, "  "+tr(s, "Protection turns:   %d")+"\n", c.ProtectionTurns)
	fmt.Fprintf(s, "  "+tr(s, "Game length (days): %d  (0 = endless)")+"\n", c.GameLength)
	fmt.Fprintf(s, "  "+tr(s, "Inter-BBS play:     %s")+"\n", onOffStr(c.IBBS))
	pause(s)
	return Stay
}

// playerList shows every living empire (Coordinator tool).
func playerList(s session.Session, w *ctx) Result {
	type row struct {
		name, owner string
		land, nw    int
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive {
				continue
			}
			rows = append(rows, row{e.Name, e.Owner, e.Land, w.NetWorth(e)})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightBlue, tr(s, "Player List"), ansi.Reset)
	fmt.Fprintf(s, "  %-16s %-14s %-8s %s\n", tr(s, "Empire"), tr(s, "Owner"), tr(s, "Land"), tr(s, "Net Worth"))
	for _, r := range rows {
		owner := r.owner
		if owner == "" {
			owner = tr(s, "(AI)")
		}
		fmt.Fprintf(s, "  %-16s %-14s %-8d %d\n", r.name, owner, r.land, r.nw)
	}
	pause(s)
	return Stay
}

func spyRelations(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Spy on Relations", 0, func(a, d *game.Empire) (string, error) { return w.SpyOnRelations(a, d) })
}

func briberyOp(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Bribery", 0, func(a, d *game.Empire) (string, error) { return w.Bribery(a, d) })
}

// allianceStrength shows the player's combined offense and defense with their
// Full Defense Alliance partners.
func allianceStrength(s session.Session, w *ctx) Result {
	p := w.Player()
	var off, def int
	var allies []string
	w.With(func() { off, def, allies = w.AllianceStrength(p) })
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Alliance Strength"), ansi.Reset)
	if len(allies) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "You have no defense allies."))
	} else {
		fmt.Fprintf(s, "  "+tr(s, "Allies: %s")+"\n", strings.Join(allies, ", "))
	}
	fmt.Fprintf(s, "  "+tr(s, "Combined offense: %d")+"\n  "+tr(s, "Combined defense: %d")+"\n", off, def)
	pause(s)
	return Stay
}

func attackPirates(s session.Session, w *ctx) Result {
	type pirateRow struct {
		name                              string
		forces, land, gold                int
		lootT, lootJ, lootU, lootK, lootA int
	}
	var rows []pirateRow
	w.With(func() {
		for _, p := range w.Pirates {
			rows = append(rows, pirateRow{
				p.Name, p.Forces, p.Land, p.Gold,
				p.LootTroopers, p.LootJets, p.LootTurrets, p.LootTanks, p.LootAgents,
			})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Pirate factions (strength is random; fat ones just raided someone):"), ansi.Reset)
	fmt.Fprintf(s, "  %-3s %-11s %-7s %-4s %-8s %s\n", "#", tr(s, "Faction"), tr(s, "Forces"), tr(s, "Rgn"), tr(s, "Gold"), tr(s, "Loot T/J/U/K/A"))
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %-11s %-7d %-4d %-8d %d/%d/%d/%d/%d\n",
			i+1, r.name, r.forces, r.land, r.gold,
			r.lootT, r.lootJ, r.lootU, r.lootK, r.lootA)
	}
	f := promptInt(s, "Raid which faction (0 to cancel)?")
	if f < 1 || f > len(rows) {
		return Stay
	}
	p := w.Player()
	troopers := promptSuggested(s, "Commit how many Troopers?", 0, p.Troopers)
	jets := promptSuggested(s, "Commit how many Jets?", 0, p.Jets)
	tanks := promptSuggested(s, "Commit how many Tanks?", 0, p.Tanks)

	// RaidFaction bounds-checks the faction index itself, so a faction that
	// vanished between the snapshot above and here just reports "no such
	// faction" instead of a stale read.
	var report string
	w.With(func() { report = w.World.RaidFaction(p, f-1, troopers, jets, tanks) })
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

func sdiProgram(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s"+tr(s, "SDI Program — current defense: %d%%")+"%s\n", ansi.FgBrightCyan, p.SDI, ansi.Reset)
	gold := promptInt(s, "Fund SDI — gold to spend (10000 per +1%%, max 75%%)?")
	if gold <= 0 {
		return Stay
	}
	var level int
	var err error
	w.With(func() { level, err = w.World.FundSDI(p, gold) })
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "SDI is now %d%%.", level)
	return Stay
}

func doomerKaboomer(s session.Session, w *ctx) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	answer := promptInt(s, fmt.Sprintf("A Doomer Kaboomer costs %d gold. Launch? (1 = yes)", game.DoomerCost))
	if answer != 1 {
		return Stay
	}
	var report string
	var err error
	w.With(func() { report, err = w.World.DoomerKaboomer(w.Player()) })
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

// renderDailyBulletin draws the boxed Daily Bulletin header: planet-wide
// totals with day-over-day change, colored green/red/neutral for +/-/0.
// title is Config.BoardID, or "" to show just "Daily Bulletin".
func renderDailyBulletin(s session.Session, b game.DailyBulletin, title string) {
	head := tr(s, "Daily Bulletin")
	if title != "" {
		head = title + " — " + head
	}
	titleBar(s, head)

	row := func(label string, total, change int, fmtNum func(int) string) {
		clr := ansi.FgWhite
		switch {
		case change > 0:
			clr = ansi.FgGreen
		case change < 0:
			clr = ansi.FgRed
		}
		sign := "+"
		abs := change
		if change < 0 {
			sign = "-"
			abs = -change
		}
		fmt.Fprintf(s, "  %s%s:%s %s    %s%s:%s %s%s%s%s\n",
			ansi.FgWhite, tr(s, label), ansi.Reset, fmtNum(total),
			ansi.FgWhite, tr(s, "Change"), ansi.Reset, clr, sign, fmtNum(abs), ansi.Reset)
	}

	row("Total Population", b.Totals.Population, b.Change.Population, comma)
	row("Total Regions", b.Totals.Regions, b.Change.Regions, comma)
	row("Total Net Worth", b.Totals.NetWorth, b.Change.NetWorth, abbrevMoney)
}

// titleBar prints a full-width white-on-blue panel header spanning the rule.
func titleBar(s session.Session, text string) {
	bar := " " + text + " "
	if pad := len(rule) - len([]rune(bar)); pad > 0 {
		bar += strings.Repeat(" ", pad)
	}
	fmt.Fprintf(s, "\n%s%s%s%s\n", ansi.BgBlue, ansi.FgBrightWhite, bar, ansi.Reset)
}

// empireStatus is the standalone status action (System menu): page through the
// Empire Status screens, pausing on each so a wide screen is not scrolled past.
func empireStatus(s session.Session, w *ctx) Result {
	renderEmpireStatus(s, w)
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

func seeScores(s session.Session, w *ctx) Result {
	printScores(s, w)
	pause(s)
	return Stay
}

func printScores(s session.Session, w *ctx) {
	// Snapshot every empire's rank inputs together so the board reflects one
	// consistent moment, even if another session mutates the world mid-render.
	type row struct {
		name     string
		alive    bool
		isPlayer bool
		land, nw int
	}
	var rows []row
	var lastMaster string
	w.With(func() {
		rows = make([]row, 0, len(w.Empires))
		for _, e := range w.Empires {
			rows = append(rows, row{e.Name, e.Alive, e == w.Player(), e.Land, w.NetWorth(e)})
		}
		lastMaster = w.LastMaster
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].nw > rows[j].nw })

	fmt.Fprintf(s, "\n%s%-4s %-18s %-8s %-10s%s\n",
		ansi.FgBrightCyan, tr(s, "Rank"), tr(s, "Empire"), tr(s, "Land"), tr(s, "Net Worth"), ansi.Reset)
	for i, r := range rows {
		name := r.name
		if !r.alive {
			name += " " + tr(s, "(dead)")
		}
		mark := "  "
		if r.isPlayer {
			mark = "->"
		}
		fmt.Fprintf(s, "%s%2d %-18s %-8d %-10d\n", mark, i+1, name, r.land, r.nw)
	}
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}

// interbbsScores displays scores imported from other boards via inter-BBS
// packets (see internal/ibbs). v1 covers only score/news sharing.
func interbbsScores(s session.Session, w *ctx) Result {
	if len(w.RemoteBoards) == 0 {
		ok(s, "No inter-BBS scores have been imported yet.")
		return Stay
	}
	for _, b := range w.RemoteBoards {
		fmt.Fprintf(s, "\n%s"+tr(s, "Board: %s (%s)")+"%s\n", ansi.FgBrightCyan, b.BoardID, b.Date, ansi.Reset)
		scores := append([]game.RemoteScore(nil), b.Scores...)
		sort.Slice(scores, func(i, j int) bool { return scores[i].NetWorth > scores[j].NetWorth })
		for _, sc := range scores {
			fmt.Fprintf(s, "  %-18s %-8d %-10d\n", sc.Empire, sc.Land, sc.NetWorth)
		}
	}
	pause(s)
	return Stay
}

// createGroupAttack assembles an interplanetary strike against an empire on
// another planet (chosen from imported scores). v1 commits a raw offense
// figure; it does not yet remove the units from the empire — a follow-up will
// make the forces actually depart.
func createGroupAttack(s session.Session, w *ctx) Result {
	p := w.Player()
	if len(w.RemoteBoards) == 0 {
		ok(s, "No other planets are known yet. Wait for inter-BBS scores to arrive.")
		return Stay
	}
	boards := make([]string, len(w.RemoteBoards))
	for i, b := range w.RemoteBoards {
		boards[i] = b.BoardID
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which planet?"), ansi.Reset)
	board := pickFromList(s, "Planet", boards)
	if board == "" {
		return Stay
	}
	var rb *game.RemoteBoard
	for i := range w.RemoteBoards {
		if w.RemoteBoards[i].BoardID == board {
			rb = &w.RemoteBoards[i]
		}
	}
	choices := []string{tr(s, "(the whole planet)")}
	for _, sc := range rb.Scores {
		choices = append(choices, sc.Empire)
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which baron?"), ansi.Reset)
	pick := pickFromList(s, "Baron", choices)
	if pick == "" {
		return Stay
	}
	target := pick
	if pick == choices[0] {
		target = "" // whole planet
	}
	offense := promptSuggested(s, "How much offense to commit?", p.Offense(), p.Offense())
	if offense <= 0 {
		return Stay
	}
	days := promptInt(s, "Leave in how many days?")
	if days < 1 {
		days = 1
	}
	var id, departDay int
	w.With(func() {
		ga := w.World.CreateGroupAttack(p, board, target, w.GameDay+days, offense)
		id, departDay = ga.ID, ga.DepartDay
	})
	ok(s, "Group attack #%d formed against %s on %s, leaving day %d.", id, pick, board, departDay)
	return Stay
}

// joinGroupAttack adds the player's offense to a group attack still forming.
func joinGroupAttack(s session.Session, w *ctx) Result {
	p := w.Player()
	var lines []string
	var ids []int
	for _, ga := range w.GroupAttacks {
		if w.GameDay >= ga.DepartDay {
			continue
		}
		tgt := ga.TargetEmpire
		if tgt == "" {
			tgt = tr(s, "the whole planet")
		}
		lines = append(lines, fmt.Sprintf("#%d -> %s on %s (leaves day %d, offense %s)",
			ga.ID, tgt, ga.TargetBoard, ga.DepartDay, comma(ga.Offense())))
		ids = append(ids, ga.ID)
	}
	if len(lines) == 0 {
		ok(s, "No group attacks are forming right now.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Join which attack?"), ansi.Reset)
	for i, x := range lines {
		fmt.Fprintf(s, "    %d) %s\n", i+1, x)
	}
	i := promptInt(s, "Attack (0 to cancel)?")
	if i < 1 || i > len(ids) {
		return Stay
	}
	offense := promptSuggested(s, "How much offense to add?", p.Offense(), p.Offense())
	if offense <= 0 {
		return Stay
	}
	var err error
	w.With(func() { err = w.World.JoinGroupAttack(p, ids[i-1], offense) })
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "You joined group attack #%d with %s offense.", ids[i-1], comma(offense))
	return Stay
}

// indivAttackForce is BRE's "Indiv. Attack Force" InterPlanetary Operations
// item. IB has no interplanetary individual-attack mechanic yet (unlike
// Create/Join Group Attack, which do); this is a recorded-but-inert stub so
// the menu's item set matches BRE's while the mechanic itself is unbuilt.
func indivAttackForce(s session.Session, w *ctx) Result {
	ok(s, "Individual attack forces are not yet available; use Create Group Attack.")
	return Stay
}

// travelTimes lists the approximate round-trip time to each known planet.
// Packets exchange on each PLANETARY maintenance run (about daily), so most
// interplanetary operations take roughly a day each way.
func travelTimes(s session.Session, w *ctx) Result {
	if len(w.LeagueNodes) == 0 && len(w.RemoteBoards) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Travel Times"), ansi.Reset)
	fmt.Fprintf(s, "%s\n", tr(s, "How long an operation takes to reach another planet and return\ndepends on how often your sysop exchanges inter-BBS packets."))

	// The league roster from ibnodes.dat, if the coordinator has distributed it.
	if len(w.LeagueNodes) > 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "League members:"), ansi.Reset)
		for _, n := range w.LeagueNodes {
			fmt.Fprintf(s, "  #%-3d %s\n", n.Number, n.Name)
		}
	}
	// Observed latency: how recently a packet actually arrived from each board.
	if len(w.RemoteBoards) > 0 {
		now := w.Today
		if now == "" {
			now = w.LastMaintDate
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Last packet received from:"), ansi.Reset)
		for _, b := range w.RemoteBoards {
			fmt.Fprintf(s, "  %-20s %s%s%s\n", b.BoardID, ansi.FgBrightCyan, daysAgoLocalized(s, b.Date, now), ansi.Reset)
		}
	}
	pause(s)
	return Stay
}

// daysAgoLocalized renders how long ago (in days) the ISO date `then` was
// relative to `now`, in the session's language, for inter-BBS packet latency.
func daysAgoLocalized(s session.Session, then, now string) string {
	t1, e1 := time.Parse("2006-01-02", then)
	t2, e2 := time.Parse("2006-01-02", now)
	if e1 != nil || e2 != nil {
		return then
	}
	switch d := int(t2.Sub(t1).Hours() / 24); {
	case d <= 0:
		return tr(s, "today")
	case d == 1:
		return tr(s, "1 day ago")
	default:
		return fmt.Sprintf(tr(s, "%d days ago"), d)
	}
}

// spyDatabase shows the planet-wide store of spy reports on remote empires.
func spyDatabase(s session.Session, w *ctx) Result {
	if len(w.SpyDatabase) == 0 {
		ok(s, "The spy database is empty. Spy on empires on other planets to fill it.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Spy Database:"), ansi.Reset)
	for _, r := range w.SpyDatabase {
		fmt.Fprintf(s, "  "+tr(s, "%s @ %s (%s): Land %s  Off %s  Def %s  Gold %s")+"\n",
			r.Empire, r.Board, r.Date, comma(r.Land), comma(r.Offense), comma(r.Defense), comma(r.Gold))
	}
	pause(s)
	return Stay
}

// terroristOps sends an agent to a remote planet to gather intel on a baron
// there; the report lands in the planet-wide Spy Database. v1: intel is drawn
// from the imported score data (land/net worth). A fuller model will queue an
// interplanetary covert strike into a packet like group attacks do.
func terroristOps(s session.Session, w *ctx) Result {
	p := w.Player()
	if len(w.RemoteBoards) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	if p.Agents < 1 {
		fail(s, game.ErrNoAgents)
		return Stay
	}
	boards := make([]string, len(w.RemoteBoards))
	for i, b := range w.RemoteBoards {
		boards[i] = b.BoardID
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Spy on which planet?"), ansi.Reset)
	board := pickFromList(s, "Planet", boards)
	if board == "" {
		return Stay
	}
	var rb *game.RemoteBoard
	for i := range w.RemoteBoards {
		if w.RemoteBoards[i].BoardID == board {
			rb = &w.RemoteBoards[i]
		}
	}
	if len(rb.Scores) == 0 {
		ok(s, "No barons are known on that planet yet.")
		return Stay
	}
	names := make([]string, len(rb.Scores))
	for i, sc := range rb.Scores {
		names[i] = sc.Empire
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Spy on which baron?"), ansi.Reset)
	pick := pickFromList(s, "Baron", names)
	if pick == "" {
		return Stay
	}
	var sc game.RemoteScore
	for _, x := range rb.Scores {
		if x.Empire == pick {
			sc = x
		}
	}
	w.With(func() {
		p.Agents--
		w.SpyDatabase = append(w.SpyDatabase, game.SpyReport{
			Board:  board,
			Empire: pick,
			Date:   w.LastMaintDate,
			Land:   sc.Land,
		})
	})
	ok(s, "Your agents infiltrated %s on %s; the report is in the Spy Database.", pick, board)
	return Stay
}

// voteCoordinator lets the player cast (or change) their vote for the BBS
// Coordinator — the elected player who gets the Coordinator menu. BRE: "Who do
// you feel should be the BBS Coordinator?"; the vote can change any time.
func voteCoordinator(s session.Session, w *ctx) Result {
	p := w.Player()
	var owners, names []string
	var coordinatorName string
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive || e.Owner == "" {
				continue
			}
			owners = append(owners, e.Owner)
			label := e.Name
			if e.Owner == p.CoordinatorVote {
				label += " " + tr(s, "(your current vote)")
			}
			names = append(names, label)
		}
		if co := w.BBSCoordinator(); co != nil {
			coordinatorName = co.Name
		}
	})
	if len(names) == 0 {
		ok(s, "There are no barons to vote for yet.")
		return Stay
	}
	if coordinatorName != "" {
		fmt.Fprintf(s, "\n%s"+tr(s, "The current BBS Coordinator is %s.")+"%s\n", ansi.FgBrightCyan, coordinatorName, ansi.Reset)
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Who should be the BBS Coordinator?"), ansi.Reset)
	for i, n := range names {
		fmt.Fprintf(s, "    %d) %s\n", i+1, n)
	}
	i := promptInt(s, "Vote for (0 to cancel)?")
	if i < 1 || i > len(owners) {
		return Stay
	}
	w.With(func() { w.World.VoteCoordinator(p, owners[i-1]) })
	ok(s, "Your vote is recorded. You may change it any time.")
	return Stay
}

// modifyLeagueDiplomacy lets a League Coordinator post a planet-wide diplomacy
// declaration, broadcast to the league on the next packet run. v1: a single
// free-text stance; a fuller model would track pairwise planet relations.
func modifyLeagueDiplomacy(s session.Session, w *ctx) Result {
	var isCoordinator bool
	var current string
	w.With(func() {
		isCoordinator = w.BBSCoordinator() == w.Player()
		current = w.LeagueDiplomacy
	})
	if !isCoordinator {
		ok(s, "Only the BBS Coordinator may set league diplomacy.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s %s\n", ansi.FgBrightCyan, tr(s, "Current league diplomacy:"), ansi.Reset, current)
	decl := prompt(s, "New league diplomacy declaration (blank to keep)")
	if strings.TrimSpace(decl) == "" {
		return Stay
	}
	w.With(func() { w.LeagueDiplomacy = decl })
	ok(s, "League diplomacy updated. It will be broadcast to the league.")
	return Stay
}

func readMessages(s session.Session, w *ctx) Result {
	p := w.Player()
	// Snapshot the mailbox and today's news under the world lock, clearing the
	// mailbox in the same critical section. An unlocked read of p.Mail would
	// race a concurrent sender, and reading-then-clearing separately could wipe
	// a message that arrived mid-display; a message that arrives after the
	// snapshot stays in p.Mail for next time. (issues #2, #5)
	var mail, news []string
	w.With(func() {
		mail = p.Mail
		p.Mail = nil
		news = append([]string(nil), w.NewsToday...)
	})
	if len(mail) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "You have no messages."))
	} else {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Your messages:"), ansi.Reset)
		for _, m := range mail {
			fmt.Fprintf(s, "  %s\n", m)
		}
	}
	if len(news) > 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Planetary Bulletin:"), ansi.Reset)
		for _, b := range news {
			fmt.Fprintf(s, "  %s\n", b)
		}
	}
	pause(s)
	return Stay
}

// recipients lists the living empires other than the player. Callers must
// already hold w's lock (w.Empires is shared, mutable state) — see
// pickRecipient and sendMessage.
func recipients(w *ctx) []*game.Empire {
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
func pickRecipient(s session.Session, w *ctx, prompt string, allowAll bool) (*game.Empire, bool) {
	type row struct {
		e               *game.Empire
		name            string
		land, land2, nw int
	}
	var rows []row
	w.With(func() {
		for _, e := range recipients(w) {
			rows = append(rows, row{e, e.Name, e.Land, e.Land, w.NetWorth(e)})
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil, false
	}
	fmt.Fprintf(s, "\n%s%-4s %-20s %-6s %-6s %s%s\n", ansi.FgBrightCyan, tr(s, "Id"), tr(s, "Empire"), tr(s, "Land"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	for i, r := range rows {
		if i >= 25 { // A..Y
			break
		}
		fmt.Fprintf(s, "(%c) %-20s %-6d %-6d %d\n", 'A'+i, r.name, r.land, r.land2, r.nw)
	}
	extra := ""
	if allowAll {
		extra = tr(s, ", Z=All")
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "(A-%c%s, 0=cancel) %s")+"%s ", ansi.FgBrightWhite, 'A'+min(len(rows), 25)-1, extra, tr(s, prompt), ansi.Reset)
	r, err := s.ReadKey()
	if err != nil {
		return nil, false
	}
	if allowAll && (r == 'z' || r == 'Z') {
		fmt.Fprintf(s, "%s\n", tr(s, "All"))
		return nil, true
	}
	idx := recipientIndex(r, len(rows))
	if idx < 0 {
		fmt.Fprint(s, "\n")
		return nil, false
	}
	fmt.Fprintf(s, "%s\n", rows[idx].name)
	return rows[idx].e, false
}

// msgMaxLines is how many lines a single message may hold (BRE: 20).
const msgMaxLines = 20

// composeMessage runs BRE's multi-line message editor: up to msgMaxLines lines
// under a column ruler. Entering "/" on a line opens the command prompt
// [A]bort / [S]ave / [C]lear. Returns the joined text and whether to send it
// (false = aborted).
func composeMessage(s session.Session) (string, bool) {
	// The banner and ruler are uniformly bright cyan in BRE (verified from a
	// live message-editor screenshot); the line-number prompts below are green.
	fmt.Fprintf(s, "\n    %s"+tr(s, "You have %d lines for your message.  /S=save /A=abort /C=clear")+"%s\n",
		ansi.FgBrightCyan, msgMaxLines, ansi.Reset)
	ruler := "[" + "---+----|" + strings.Repeat("----+----|", 6) + "]"
	fmt.Fprintf(s, "    %s%s%s\n", ansi.FgBrightCyan, ruler, ansi.Reset)

	var lines []string
	for len(lines) < msgMaxLines {
		fmt.Fprintf(s, "%s%2d>%s ", ansi.FgBrightGreen, len(lines)+1, ansi.Reset)

		// A '/' as the FIRST key of a line opens the command sub-menu right away
		// — BRE reads key-by-key and reacts on the bare '/', so we peek the first
		// key instead of reading a whole line. '/' anywhere else stays literal
		// text (e.g. "line s /s"), so only the leading key is special.
		first, err := s.ReadKey()
		if err != nil {
			return "", false
		}
		if first == '/' {
			fmt.Fprintf(s, tr(s, "/-Command?")+"  [%sA%s,%sS%s,%sC%s] ",
				ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset)
			r, err := s.ReadKey()
			if err != nil {
				return "", false
			}
			switch unicode.ToUpper(r) {
			case 'A':
				fmt.Fprintf(s, "%s\n", tr(s, "Abort"))
				return "", false
			case 'S':
				fmt.Fprintf(s, "%s\n", tr(s, "Save"))
				return trimTrailingBlank(lines), true
			case 'C':
				fmt.Fprintf(s, "%s\n", tr(s, "Clear"))
				lines = nil
			default:
				fmt.Fprint(s, "\n")
			}
			continue
		}
		if first == '\r' || first == '\n' {
			fmt.Fprint(s, "\n")
			lines = append(lines, "")
			continue
		}

		// Ordinary line: echo the first key, then read the rest until Enter.
		b := []rune{first}
		fmt.Fprintf(s, "%c", first)
		for {
			r, err := s.ReadKey()
			if err != nil {
				return "", false
			}
			if r == '\r' || r == '\n' {
				fmt.Fprint(s, "\n")
				break
			}
			if r == 127 || r == 8 { // backspace / DEL
				if len(b) > 0 {
					b = b[:len(b)-1]
					fmt.Fprint(s, "\b \b")
				}
				continue
			}
			if r >= 32 {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
		lines = append(lines, string(b))
	}
	fmt.Fprintf(s, "%s"+tr(s, "You have used all %d lines.")+"%s\n", ansi.FgYellow, msgMaxLines, ansi.Reset)
	return trimTrailingBlank(lines), true
}

// trimTrailingBlank joins message lines, dropping trailing empty ones.
func trimTrailingBlank(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func sendMessage(s session.Session, w *ctx) Result {
	p := w.Player()
	for {
		to, all := pickRecipient(s, w, "Send to:", true)
		if !all && to == nil {
			return Stay
		}
		text, send := composeMessage(s)
		if send && strings.TrimSpace(text) != "" {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
			w.With(func() {
				if all {
					for _, e := range recipients(w) {
						w.World.SendMail(p, e, text)
					}
				} else {
					w.World.SendMail(p, to, text)
				}
			})
		}
		if !askYesNo(s, "Do you wish to send another message?", false) {
			return Stay
		}
	}
}

func sendTradeDeal(s session.Session, w *ctx) Result {
	p := w.Player()
	to, _ := pickRecipient(s, w, "Trade with:", false)
	if to == nil {
		return Stay
	}
	amount := promptInt(s, "How much gold?")
	if amount <= 0 {
		return Stay
	}
	var err error
	w.With(func() { err = w.World.SendGold(p, to, amount) })
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Sent %d gold to %s.", amount, to.Name)
	}
	return Stay
}

func planetaryPost(s session.Session, w *ctx) Result {
	text := strings.TrimSpace(prompt(s, "Post to the planet:"))
	if text == "" {
		return Stay
	}
	w.With(func() { w.World.PostBulletin(w.Player(), text) })
	ok(s, "Posted.")
	return Stay
}

// negotiateTreaty returns a Diplomacy menu action for one BRE treaty type
// (#68): pick a target empire, then propose it, accept a matching offer from
// them, or break it if already held. Treaty types are now direct menu items
// instead of hiding behind a single "Modify Diplomacy" item.
func negotiateTreaty(ttype string) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		type row struct {
			e      *game.Empire
			name   string
			suffix string
		}
		var rows []row
		w.With(func() {
			for _, e := range w.Empires {
				if !e.Alive || e == p {
					continue
				}
				suffix := ""
				if w.World.HasTreaty(p, e, ttype) {
					suffix = " — " + tr(s, "held")
				} else {
					for _, o := range offersFrom(p, e.Name) {
						if o == ttype {
							suffix = " — " + tr(s, "offers this to you")
						}
					}
				}
				rows = append(rows, row{e, e.Name, suffix})
			}
		})
		if len(rows) == 0 {
			ok(s, "There is no one to negotiate with.")
			return Stay
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
		for i, r := range rows {
			fmt.Fprintf(s, "  %d) %s%s\n", i+1, r.name, r.suffix)
		}
		i := promptInt(s, "Negotiate with which empire (0 to cancel)?")
		if i < 1 || i > len(rows) {
			return Stay
		}
		negotiateWithType(s, w, p, rows[i-1].e, ttype)
		return Stay
	}
}

// negotiateWithType performs the one action that applies to ttype between p
// and e: propose it if neither side has it, accept it if e has already
// offered it, or break it (with confirmation) if p already holds it.
func negotiateWithType(s session.Session, w *ctx, p, e *game.Empire, ttype string) {
	var held, offered bool
	w.With(func() {
		held = w.World.HasTreaty(p, e, ttype)
		if !held {
			for _, o := range offersFrom(p, e.Name) {
				if o == ttype {
					offered = true
				}
			}
		}
	})
	switch {
	case held:
		fmt.Fprintf(s, "\n%s"+tr(s, "You hold a %s with %s.")+"%s\n", ansi.FgBrightCyan, ttype, e.Name, ansi.Reset)
		if !askYesNo(s, "Break this treaty?", false) {
			return
		}
		w.With(func() { w.World.BreakTreaty(p, e, ttype) })
		ok(s, "You broke the %s with %s.", ttype, e.Name)
	case offered:
		fmt.Fprintf(s, "\n%s"+tr(s, "%s offers you a %s.")+"%s\n", ansi.FgBrightCyan, e.Name, ttype, ansi.Reset)
		if !askYesNo(s, "Accept this treaty?", false) {
			return
		}
		w.With(func() { w.World.AcceptTreaty(p, e.Name, ttype) })
		ok(s, "You accepted the %s with %s.", ttype, e.Name)
	default:
		w.With(func() { w.World.ProposeTreaty(p, e, ttype) })
		ok(s, "Proposed a %s to %s.", ttype, e.Name)
	}
}

// declareWar is BRE's Declaration Of War: pick a target and, on confirmation,
// break every treaty currently held with them in one action. Per
// docs/mechanics-reference.md this is meant to skip the internal unrest a
// normal treaty break causes — but IB does not yet model unrest on treaty
// breaks at all, so there is no behavioral difference from breaking each
// treaty individually today; this is a placeholder for when that lands.
func declareWar(s session.Session, w *ctx) Result {
	p := w.Player()
	type row struct {
		e    *game.Empire
		name string
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive || e == p {
				continue
			}
			rows = append(rows, row{e, e.Name})
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to declare war on.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose an empire:"), ansi.Reset)
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %s\n", i+1, r.name)
	}
	i := promptInt(s, "Declare war on which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	if !askYesNo(s, "Declare war? This breaks all treaties with them.", false) {
		return Stay
	}
	var broke []string
	w.With(func() {
		for _, tt := range w.World.TreatiesBetween(p, target) {
			w.World.BreakTreaty(p, target, tt)
			broke = append(broke, tt)
		}
	})
	if len(broke) == 0 {
		ok(s, "You declared war on %s.", target.Name)
	} else {
		ok(s, "You declared war on %s. Treaties broken: %s", target.Name, strings.Join(broke, ", "))
	}
	return Stay
}

// offersFrom returns the treaty types `from` has offered to p.
func offersFrom(p *game.Empire, from string) []string {
	var out []string
	for _, o := range p.TreatyOffers {
		if o.From == from {
			out = append(out, o.Type)
		}
	}
	return out
}

// pickFromList shows a numbered list and returns the chosen entry, or "".
func pickFromList(s session.Session, msg string, list []string) string {
	for i, x := range list {
		fmt.Fprintf(s, "    %d) %s\n", i+1, x)
	}
	i := promptInt(s, msg+" (0 to cancel)?")
	if i < 1 || i > len(list) {
		return ""
	}
	return list[i-1]
}

func viewDiplomacy(s session.Session, w *ctx) Result {
	p := w.Player()
	type row struct{ name, treaties string }
	var rows []row
	var offers []game.TreatyOffer
	w.With(func() {
		for _, e := range w.Empires {
			if e == p || !e.Alive {
				continue
			}
			if held := w.TreatiesBetween(p, e); len(held) > 0 {
				rows = append(rows, row{e.Name, strings.Join(held, ", ")})
			}
		}
		offers = append([]game.TreatyOffer(nil), p.TreatyOffers...)
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Your treaties:"), ansi.Reset)
	if len(rows) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "(none)"))
	} else {
		for _, r := range rows {
			fmt.Fprintf(s, "  %s: %s\n", r.name, r.treaties)
		}
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Pending offers received:"), ansi.Reset)
	if len(offers) == 0 {
		fmt.Fprintf(s, "  %s\n", tr(s, "(none)"))
	} else {
		for _, o := range offers {
			fmt.Fprintf(s, "  %s — %s\n", o.From, o.Type)
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
func setIndustries(s session.Session, w *ctx) Result {
	p := w.Player()
	proj := w.ProjectedProduction(p)
	fmt.Fprintf(s, "\n%s\n", titleRule(ansi.FgBrightRed, tr(s, "Industrial Production")))
	for i, name := range prodTypeNames {
		fmt.Fprintf(s, "%-10s : %s%3d%%%s      %s"+tr(s, "(%d per year)")+"%s\n",
			tr(s, name), ansi.FgBrightYellow, *prodField(p, i), ansi.Reset, ansi.FgRed, proj[i], ansi.Reset)
	}
	if p.Specialized != "" {
		fmt.Fprintf(s, "\n%s"+tr(s, "Specialized in %s: more of it, less of everything else.")+"%s\n",
			ansi.FgBrightCyan, tr(s, p.Specialized), ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	if !askYesNo(s, "Change Production?", false) {
		return Stay
	}
	ns := make([]int, len(prodTypeNames))
	for i, name := range prodTypeNames {
		cur := *prodField(p, i)
		ns[i] = promptSuggested(s, name, cur, 100)
	}
	w.With(func() {
		for i, n := range ns {
			*prodField(p, i) = n
		}
	})
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
	w.With(func() { p.Specialized = prodTypeNames[t-1] })
	ok(s, "Your industry is now permanently specialized in %s.", p.Specialized)
	return Stay
}

func setTaxRate(s session.Session, w *ctx) Result {
	p := w.Player()
	maxRate := w.Config.MaxTaxRate
	fmt.Fprintf(s, "\n%s"+tr(s, "Current tax rate: %d%%")+"%s\n", ansi.FgBrightCyan, p.Tax, ansi.Reset)
	rate := promptInt(s, fmt.Sprintf("New tax rate (0-%d)?", maxRate))
	if rate < 0 || rate > maxRate {
		fail(s, fmt.Errorf("tax rate must be between 0 and %d", maxRate))
		return Stay
	}
	w.With(func() { p.Tax = rate })
	ok(s, "Tax rate set to %d%%.", rate)
	return Stay
}
