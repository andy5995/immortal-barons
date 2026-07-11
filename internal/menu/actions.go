package menu

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// errRealmChanged is the abort-on-conflict notice: a mutating action re-resolves
// the active empire inside its transaction and, if it has vanished (abdicated by
// another node between the prompt and the write), aborts cleanly with this rather
// than dereferencing a nil empire.
var errRealmChanged = errors.New("The realm has changed — try again.")

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
		w.With(func() {
			// Re-resolve the empire against the freshly-reloaded world and let
			// apply re-check gold atomically — the p/price gathered above (before
			// the prompt) may be stale after a concurrent node's transaction.
			p := w.Player()
			if p == nil {
				err = errRealmChanged
				return
			}
			err = apply(w.World, p, n)
		})
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
		var gold int
		w.With(func() {
			p := w.Player()
			if p == nil {
				err = errRealmChanged
				return
			}
			err = apply(w.World, p, n) // apply re-checks stock atomically
			gold = p.Gold
		})
		if err != nil {
			fail(s, err)
		} else {
			ok(s, "Sold %d. Gold: %d", n, gold)
		}
		return Stay
	}
}

// buildHQ starts HeadQuarters construction for the acting empire.
func buildHQ(s session.Session, w *ctx) Result {
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		err = w.World.StartHQ(p)
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "You have started work on your HeadQuarters.")
	}
	return Stay
}

// regionTypeNames lists the 8 region types in the stable order
// RegionMix.fields()/e.Regions' own field order uses.
var regionTypeNames = []string{
	"Coastal", "Mountain", "Desert", "River",
	"Agricultural", "Urban", "Industrial", "Technology",
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

// printRegionTable renders the BRE-style region picker: a Key / Name / Owned
// table, colored (magenta keys, yellow names) so buy and drop share one look.
func printRegionTable(s session.Session, p *game.Empire) {
	fmt.Fprintf(s, "%s%-5s%-15s%s%s\n", ansi.FgBrightWhite, tr(s, "Key"), tr(s, "Name"), tr(s, "Owned"), ansi.Reset)
	for i, name := range regionTypeNames {
		fmt.Fprintf(s, " %s(%c)%s %s%-14s%s %5d\n",
			ansi.FgBrightMagenta, regionTypeKeys[i], ansi.Reset,
			ansi.FgBrightYellow, name, ansi.Reset,
			*regionField(p, i))
	}
}

// promptRegionType reads a single-letter region choice (case-insensitive),
// returning its 0-based index or -1 to cancel.
func promptRegionType(s session.Session) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your choice? (0 to cancel)"), ansi.Reset)
	for {
		r, err := readKey(s)
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
		r, err := readKey(s)
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
				// Re-resolve inside the transaction: BuyRegions re-checks gold and
				// the per-turn region cap against fresh state, and the region field
				// pointer must index the reloaded empire, not the stale gather.
				fp := w.Player()
				if fp == nil {
					err = errRealmChanged
					return
				}
				err = w.World.BuyRegions(fp, regionField(fp, t), n)
				gold = fp.Gold
			})
			if err != nil {
				// No pause: the message stays above the next prompt; the player
				// keeps buying without the region list rescrolling each time.
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgRed, tr(s, err.Error()), ansi.Reset)
			} else {
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
					fmt.Sprintf(tr(s, "%d %s regions purchased. Gold: %d"), n, regionTypeNames[t], gold), ansi.Reset)
			}
			p = w.Player() // refresh the display pointer for the next iteration
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
	var land int
	w.With(func() {
		fp := w.Player()
		if fp == nil {
			err = errRealmChanged
			return
		}
		err = w.World.DropRegions(fp, regionField(fp, t), n)
		land = fp.Land
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "%d %s regions dropped. You now hold %d land.", n, regionTypeNames[t], land)
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
		var gold, bank, debt int
		w.With(func() {
			// apply (Deposit/Withdraw/Loan/Repay) re-checks the balance/debt and
			// the MoneyCap against the reloaded empire, so a concurrent node can't
			// let two sessions withdraw the same funds or overdraw.
			p := w.Player()
			if p == nil {
				err = errRealmChanged
				return
			}
			err = apply(w.World, p, n)
			gold, bank, debt = p.Gold, p.Bank, p.Debt
		})
		if err != nil {
			fail(s, err)
		} else {
			ok(s, "Gold: %d   Bank: %d   Debt: %d", gold, bank, debt)
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
	var ret, matureDay int
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		ret, err = w.World.Invest(p, amount, days) // re-checks affordability atomically
		matureDay = w.GameDay + days
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Invested %d for %d days; ~%d returns on day %d.", amount, days, ret, matureDay)
	}
	return Stay
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
	r, err := readKey(s)
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
		k, err := readKey(s)
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
	var gold int
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		err = w.World.BuyFoodMarket(p, n) // re-checks gold atomically
		gold = p.Gold
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Bought %d food. Gold: %d", n, gold)
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
	var gold int
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		err = w.World.SellFood(p, n) // re-clamps to fresh food stock atomically
		gold = p.Gold
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Sold %d food. Gold: %d", n, gold)
	}
	return Stay
}

// titleBar prints a full-width white-on-blue panel header spanning the rule.
func titleBar(s session.Session, text string) {
	bar := " " + text + " "
	if pad := len(rule) - len([]rune(bar)); pad > 0 {
		bar += strings.Repeat(" ", pad)
	}
	fmt.Fprintf(s, "\n%s%s%s%s\n", ansi.BgBlue, ansi.FgBrightWhite, bar, ansi.Reset)
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
