package menu

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

// abdicate ends the player's empire (BRE.DOC: "delete your empire from the game
// so you may start over the next day"). It is irreversible, so the player must
// retype their realm name to confirm. The realm is marked dead (not removed) so
// the same next-day rule as a battlefield death applies: the husk lingers until
// a LATER day, and only then does a login rebuild a fresh realm. Daily
// maintenance sweeps the husk once GameDay passes DiedDay.
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
	// Re-resolve inside the transaction: a p captured before the confirmation
	// prompt could rebind to the WRONG empire after a reload reshaped the empire
	// set. Mark the freshly-resolved empire dead instead of removing it, so the
	// husk survives to enforce the next-day rebuild rule.
	w.With(func() {
		if p := w.Player(); p != nil {
			p.Alive = false
			p.DiedDay = w.GameDay
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "Your empire is no more. Return on a later day to build a new realm."), ansi.Reset)
	pause(s)
	return Quit
}

// endProtection lets a player waive their remaining new-realm protection early
// (IB's own — BRE has no such option). It is one-way: the realm becomes open to
// attacks and pirate raids at once, so it takes a confirmation.
func endProtection(s session.Session, w *ctx) Result {
	p := w.Player()
	if p.Protection <= 0 {
		okNoPause(s, "Your realm is not under new-realm protection.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "Your realm has %d turns of new-realm protection left.")+"%s\n",
		ansi.FgBrightYellow, p.Protection, ansi.Reset)
	if !AskYesNo(s, "You will be out of protection at the end of this turn. Are you sure?", false) {
		return Stay
	}
	// Drop to a single turn, not 0: the realm stays protected for the rest of
	// this turn (still can't attack or be attacked), and the turn-end tick in
	// PlayTurn clears it — so ending protection can't double as a free same-turn
	// attack.
	w.With(func() {
		if fp := w.Player(); fp != nil && fp.Protection > 1 {
			fp.Protection = 1
		}
	})
	ok(s, "Your new-realm protection will end when this turn does.")
	return Stay
}

// advisorDomain is one of BRE's four advisors. The values match the submenu's
// 1..4 numbering (Civilian, Economic, Military, Technology).
type advisorDomain int

const (
	advisorCivilian advisorDomain = iota + 1
	advisorEconomic
	advisorMilitary
	advisorTechnology
)

// advisorData is a consistent snapshot of the figures the advisors report,
// gathered under one lock so the numbers agree (same discipline as Empire
// Status). The world totals feed the Economic advisor's share-of-world and
// world-average figures.
type advisorData struct {
	p           game.Empire
	foodGrown   int // this turn's food production (tech-boosted, incl. rivers)
	foodEaten   int // this turn's food consumption
	foodAtCap   int // consumption once population fills its carrying capacity
	income      int // this turn's gold income
	worldIncome int // Σ income over living empires
	worldLand   int // Σ Land over living empires
}

func gatherAdvisorData(w *ctx) advisorData {
	var d advisorData
	w.With(func() {
		d.p = *w.Player()
		d.foodGrown = w.FoodGrown(&d.p)
		d.foodEaten = w.FoodDue(&d.p)
		d.foodAtCap = d.p.FoodUpkeepAtCapacity()
		d.income = w.IncomeThisTurn(&d.p).Gold()
		for _, e := range w.Empires {
			if e.Alive {
				d.worldIncome += w.IncomeThisTurn(e).Gold()
				d.worldLand += e.Land
			}
		}
	})
	return d
}

// advisorGreeting is the advisor's first-person opening line. BRE's advisors are
// named and speak in first person ("Hi, I'm Joe, your military advisor"); IB's
// carry their own coined names and a light, dry touch of character.
func advisorGreeting(s session.Session, d advisorDomain) string {
	switch d {
	case advisorCivilian:
		return tr(s, "Odris, your civilian advisor, Sire. I keep an ear to the people and an eye on the food supply.")
	case advisorEconomic:
		return tr(s, "Vell, your treasurer, Sire. I count the coin twice — once for hope, once for the truth.")
	case advisorMilitary:
		return tr(s, "Krane, your war advisor, Sire. I will be brief; the enemy rarely is.")
	default:
		return tr(s, "Sable, your technology advisor, Sire. I tend the labs and the small miracles they leak.")
	}
}

// advisorLine is one line an advisor speaks, with the color its FIGURES take.
// BRE varies this: the Civilian advisor reports its tallies in bright-white but
// flags a food shortfall in bright-yellow; hi carries that per-line choice.
//
// A phrase wrapped in {braces} in Text is a KEY TERM, rendered in Emph — BRE
// pulls one phrase out of an advisor sentence and leaves the rest plain: the
// unit type in a piece of military advice ("...the strength of your Tanks",
// bright-yellow) and the aspect name in a Technology report ("Our military
// forces are functioning at...", bright-white). Translators keep the braces;
// they are stripped at render.
type advisorLine struct {
	Text string
	Hi   string // figure highlight color for this line
	Emph string // color for {braced} key terms ("" = none)
	// Note marks the Technology advisor's closing remark, which BRE sets apart
	// from the report above it: a blank line, a bright-cyan "NOTE:" and a cyan
	// body indented to hang under the label (docs/dev/bre-screens.md).
	Note bool
}

// hiTerms replaces each {braced} run in s with emph-colored text, returning to
// base afterwards. Runs after word-wrapping (a braced phrase has no spaces, so
// it can never straddle a wrap) and before hiNumsReset, which passes the escape
// sequences this inserts through untouched.
func hiTerms(s, emph, base string) string {
	if emph == "" || !strings.ContainsRune(s, '{') {
		return strings.NewReplacer("{", "", "}", "").Replace(s)
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i:], '}')
		if j < 0 {
			break
		}
		b.WriteString(s[:i])
		b.WriteString(emph)
		b.WriteString(s[i+1 : i+j])
		b.WriteString(base)
		s = s[i+j+1:]
	}
	b.WriteString(s)
	return strings.NewReplacer("{", "", "}", "").Replace(b.String())
}

// advisorReport builds the lines one advisor speaks: the figures for its domain
// (mirroring BRE's advisor reports — the populace and food, the treasury and
// income, the armed forces, and technology's effects) plus any advice that
// applies. Kept separate from rendering so tests can inspect the lines. The
// per-line figure color follows BRE (docs/dev/bre-screens.md): Economic and
// Technology figures are bright-yellow, the Civilian/Military tallies are
// bright-white, and a Civilian food shortfall is flagged bright-yellow.
func advisorReport(s session.Session, d advisorData, dom advisorDomain) []advisorLine {
	num := func(n int64) string { return formatGold(n, sessionLang(s)) }
	// count is num for figures held in count width (people, food, units).
	count := func(n int) string { return formatGold(n, sessionLang(s)) }
	p := &d.p
	fig := ansi.FgBrightWhite
	if dom == advisorEconomic || dom == advisorTechnology {
		fig = ansi.FgBrightYellow
	}
	// Key terms take bright-yellow in the Military advisor's advice (BRE colors
	// the unit type it names) and bright-white in the Technology report (BRE
	// colors the aspect name, leaving the percentage yellow). The other two
	// advisors mark none.
	emph := ""
	switch dom {
	case advisorMilitary:
		emph = ansi.FgBrightYellow
	case advisorTechnology:
		emph = ansi.FgBrightWhite
	}
	var out []advisorLine
	add := func(text string) { out = append(out, advisorLine{Text: text, Hi: fig, Emph: emph}) }
	warn := func(text string) { out = append(out, advisorLine{Text: text, Hi: ansi.FgBrightYellow, Emph: emph}) }
	note := func(text string) { out = append(out, advisorLine{Text: text, Hi: fig, Note: true}) }
	switch dom {
	case advisorCivilian:
		add(fmt.Sprintf(tr(s, "Our people number %s, and their support stands at %d%%."), count(p.People), p.Support))
		add(fmt.Sprintf(tr(s, "We grow %s food each turn and consume %s."), count(d.foodGrown), count(d.foodEaten)))
		net := d.foodGrown - d.foodEaten
		// Food is credited at turn start, so p.Food already includes this turn's
		// growth. The projections below are written against the pre-growth stock, so
		// recover it (p.Food - foodGrown) rather than counting the growth twice.
		stock := p.Food - d.foodGrown
		switch {
		case stock+net < 0:
			// Even with this turn's growth already in, stores can't cover this turn's
			// consumption, so the turn ends with negative food (turn.go starvation step).
			warn(tr(s, "Our food will not last the turn. Buy or grow more."))
		case net < 0:
			warn(fmt.Sprintf(tr(s, "We run a shortfall of %s; our stores will run out in about %d turns."), count(-net), stock/(-net)))
		case d.foodAtCap > d.foodGrown:
			// Fed now, but the populace is still growing toward a support-driven
			// capacity whose food need outruns production (see issue #35).
			warn(fmt.Sprintf(tr(s, "We have a surplus now, but our people are still growing. At full size they will eat about %s food each turn, more than we grow. Add agricultural regions before then."), count(d.foodAtCap)))
		default:
			// The food bottom line pops in yellow whether short or in surplus (BRE).
			warn(fmt.Sprintf(tr(s, "That leaves a surplus of %s. Our stores are secure."), count(net)))
		}
		if p.Support < 50 {
			warn(tr(s, "The people grow restless. Lower taxes or spend on their support."))
		}
		if p.Tax > 20 {
			warn(tr(s, "Taxes are high enough to risk riots. Consider lowering them."))
		}
	case advisorEconomic:
		add(fmt.Sprintf(tr(s, "Our treasury holds %s gold, with %s more in the bank."), num(p.Gold), num(p.Bank)))
		if p.Debt > 0 {
			add(fmt.Sprintf(tr(s, "We owe %s gold in debt, which grows each turn."), num(p.Debt)))
		}
		share := 0
		if d.worldIncome > 0 {
			share = d.income * 100 / d.worldIncome
		}
		add(fmt.Sprintf(tr(s, "We earn about %s gold each turn, %d%% of the world total."), count(d.income), share))
		perRegion, avg := 0, 0
		if p.Land > 0 {
			perRegion = d.income / p.Land
		}
		if d.worldLand > 0 {
			avg = d.worldIncome / d.worldLand
		}
		add(fmt.Sprintf(tr(s, "That is %s gold per region; the world average is %s."), count(perRegion), count(avg)))
		if p.Gold <= 0 && p.Bank <= 0 {
			add(tr(s, "Our treasury is empty, Sire. We should raise gold soon."))
		}
	case advisorMilitary:
		add(fmt.Sprintf(tr(s, "Our forces: %s troopers, %s jets, %s turrets, %s tanks, %s bombers, %s carriers."),
			count(p.Troopers), count(p.Jets), count(p.Turrets), count(p.Tanks), count(p.Bombers), count(p.Carriers)))
		switch {
		case p.HQ == 0:
			// The price climbs with every turn played (World.HQPrice), so "soon" is
			// the actionable half of this advice. The figure itself belongs to the
			// Spending Menu, which quotes the live price.
			add(tr(s, "We have no {HeadQuarters}. Building one would strengthen our {tanks}, and it costs more with every turn we wait."))
		case p.HQ < 100:
			add(fmt.Sprintf(tr(s, "Our {HeadQuarters} is %d%% built."), p.HQ))
		default:
			add(tr(s, "Our {HeadQuarters} is fully built."))
		}
		if p.Carriers*100 < p.Jets {
			add(tr(s, "We have more {jets} than our {carriers} can carry. Build more {carriers}."))
		}
		add(fmt.Sprintf(tr(s, "Troop morale stands at %d%%."), p.Morale))
		if p.Morale < 50 {
			warn(tr(s, "Morale is low. Desertion is a real risk before our next battle."))
		}
		if p.Agents == 0 {
			add(tr(s, "We have no {covert agents}. Recruit some for spying and sabotage."))
		} else {
			add(fmt.Sprintf(tr(s, "We keep %s covert agents."), count(p.Agents)))
		}
	case advisorTechnology:
		// One line per effect, as BRE's Technology advisor reports them: raised
		// effects as 100xfactor, lowered ones as 100/factor. Research never decays,
		// so a realm that has sold its Technology regions still reports what it
		// banked.
		gold := game.TechPercent(p.TechGoldFactor(), false)
		food := game.TechPercent(p.TechFoodFactor(), false)
		units := game.TechPercent(p.TechUnitFactor(), false)
		mil := game.TechPercent(p.TechMilitaryFactor(), false)
		maint := game.TechPercent(p.TechMaintFactor(), true)
		sdi := game.TechPercent(p.TechSDIFactor(), true)
		decay := game.TechPercent(p.TechDecayFactor(), true)
		researched := gold > 100 || food > 100 || units > 100 || mil > 100 ||
			maint < 100 || sdi < 100 || decay < 100
		switch {
		case !researched && p.Regions.Technology == 0:
			add(tr(s, "We have no Technology regions."))
			add(tr(s, "Building some would raise our military strength, income, and food output, and lower our upkeep — a benefit that builds up over time."))
		case !researched:
			add(tr(s, "Our Technology regions are new. Their benefits will build up as we hold them."))
		default:
			add(fmt.Sprintf(tr(s, "Our {military forces} are functioning at %d%% strength."), mil))
			add(fmt.Sprintf(tr(s, "Our {gold producing regions} are at %d%% of normal production."), gold))
			add(fmt.Sprintf(tr(s, "Our {food production techniques} increased efficiency to %d%%."), food))
			add(fmt.Sprintf(tr(s, "Our {industries} are running at %d%% efficiency."), units))
			add(fmt.Sprintf(tr(s, "Our {maintenance costs} have been reduced to %d%% of standard costs."), maint))
			add(fmt.Sprintf(tr(s, "Our {SDI yearly funding} needs have been lowered to %d%% normal expenses."), sdi))
			add(fmt.Sprintf(tr(s, "{Food decay} is at %d%% of standard levels."), decay))
			if p.Regions.Technology == 0 {
				add(tr(s, "We hold no Technology regions, so our research has halted — but what we have already learned is not lost."))
			}
			// BRE closes this advisor with a set-apart NOTE saying the same
			// thing, so it is the last line whether or not research has halted.
			note(tr(s, "Technology levels are relative to the number of regions in the empire. A larger realm needs more advanced technology to hold the same efficiency as a smaller one."))
		}
	}
	return out
}

// renderAdvisor prints one advisor's greeting and its report. Split from the
// menu loop so tests can render an advisor without a pause.
func renderAdvisor(s session.Session, w *ctx, d advisorDomain) {
	data := gatherAdvisorData(w)
	fmt.Fprint(s, "\n")
	// Wrap the greeting too (it now carries a name + a line of character, so it can
	// run past 80 columns).
	for _, gl := range strings.Split(help.Wrap(advisorGreeting(s, d), 78), "\n") {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightCyan, gl, ansi.Reset)
	}
	// Body text is regular/off-white (37); the figures are the only bright things
	// on the line — bright-white (97) or yellow (93) per the line's Hi — so they
	// pop, the way BRE's advisors read (docs/dev/bre-screens.md). Without the dim
	// base, bright-white figures would blend into a terminal's default-white text.
	base := ansi.FgWhite
	for _, line := range advisorReport(s, data, d) {
		// Word-wrap each report line to the screen width (78) less the 2-space
		// indent (wrap the plain text, then color), so a long sentence breaks at
		// spaces instead of mid-word at col 80. A figure returns to the off-white
		// base after its highlight, not the terminal default.
		if line.Note {
			// BRE's own shape: a blank line, "NOTE:" in bright cyan, and the body
			// in cyan hanging under the label at six columns.
			fmt.Fprint(s, "\n")
			for i, wl := range strings.Split(help.Wrap(line.Text, 66), "\n") {
				if i == 0 {
					fmt.Fprintf(s, "%sNOTE:%s %s%s%s\n", ansi.FgBrightCyan, ansi.Reset, ansi.FgCyan, wl, ansi.Reset)
					continue
				}
				fmt.Fprintf(s, "      %s%s%s\n", ansi.FgCyan, wl, ansi.Reset)
			}
			continue
		}
		for _, wl := range strings.Split(help.Wrap(line.Text, 76), "\n") {
			fmt.Fprintf(s, "  %s%s%s\n", base, hiNumsReset(hiTerms(wl, line.Emph, base), line.Hi, base), ansi.Reset)
		}
	}
}

// advisorsMenu is BRE's four-advisor submenu: pick an advisor to hear that
// domain's counsel, or 0 to leave. Shared by the System menu's "Visit Advisors"
// action and the Buy Regions "(*) Advisors" entry.
func advisorsMenu(s session.Session, w *ctx) {
	// item colors match BRE's Advisors menu (docs/dev/bre-screens.md): magenta
	// parens, a bright-magenta key, a white label.
	item := func(n int, label string) {
		fmt.Fprintf(s, "  %s(%s%d%s)%s %s%s%s\n",
			ansi.FgMagenta, ansi.FgBrightMagenta, n, ansi.FgMagenta, ansi.Reset,
			ansi.FgWhite, tr(s, label), ansi.Reset)
	}
	for {
		// BRE frames this menu with a magenta bracketed rule ("──[Advisors]──"),
		// not IB's lightbar (docs/dev/bre-screens.md).
		fmt.Fprintf(s, "\n%s\n", titleRule(ansi.FgMagenta, tr(s, "Advisors"), len([]rune(rule))))
		item(1, "Civilian")
		item(2, "Economic")
		item(3, "Military")
		item(4, "Technology")
		item(0, "Quit")
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
		n := ChoiceQuit(s, 4)
		if n < 1 {
			return
		}
		renderAdvisor(s, w, advisorDomain(n))
		pause(s)
	}
}

// visitAdvisors is the System menu's "Visit Advisors" action.
func visitAdvisors(s session.Session, w *ctx) Result {
	advisorsMenu(s, w)
	return Stay
}

// about shows a short project panel: name, version, website, and the BRE
// heritage note (reachable from both the Game and System menus, #66).
func about(s session.Session, w *ctx) Result {
	// BRE frames its status blocks with a dashes-and-double-line separator
	// (─────═════…, docs/dev/bre-screens.md) rather than a title panel; the About
	// screen follows that look — no blue title bar, the name/version below is the
	// only headline.
	fmt.Fprintf(s, "\n%s%s%s\n", dim(ansi.FgBrightRed), InsetRule, ansi.Reset)
	centered(s, ansi.FgBrightWhite, "Immortal Barons  v"+game.VersionString())
	centered(s, ansi.FgBrightCyan, "https://andy5995.github.io/immortal-barons/")
	fmt.Fprint(s, "\n")
	// Body prose is off-white so the name/version headline above stays the only
	// bright thing on the panel, per docs/dev/bre-screens.md. Wrap to the rule's
	// width less the 2-space indent.
	for _, wl := range strings.Split(help.Wrap(tr(s, "An independent tribute to Barren Realms Elite (BRE), created by Mehul Patel and now owned by John Dailey Software. It is maintained by Andy Alt, with contributions and advice from the BBS and Barren Realms Elite community. This project is not affiliated with, nor endorsed by, the current or past owner(s) of Barren Realms Elite."), len([]rune(rule))-2), "\n") {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, wl, ansi.Reset)
	}
	fmt.Fprint(s, "\n")
	fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, tr(s, "Free software under the MIT License."), ansi.Reset)
	fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, "Copyright (c) 2026 Andy Alt", ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgBrightRed), rule, ansi.Reset)
	pause(s)
	return Stay
}

// gameSetup shows the current game rules (read-only; the sysop edits them with
// the -reset Configuration Editor). Two screens with a pause between them: the
// full ruleset does not fit an 80x25 terminal, and a player reading it wants
// the turn/economy rules and the war/trade rules apart anyway.
func gameSetup(s session.Session, w *ctx) Result {
	c := w.Config
	// Label in body white, figure in bright white — the highlight convention from
	// docs/dev/bre-screens.md, so the numbers a player is scanning for stand out.
	row := func(label, value string) {
		fmt.Fprintf(s, "  %s%-24s%s %s%s%s\n",
			ansi.FgWhite, tr(s, label), ansi.Reset, ansi.FgBrightWhite, value, ansi.Reset)
	}
	// countOr renders a limit, naming what its zero means rather than printing a
	// bare 0 the player has to interpret.
	countOr := func(n int, unlimited string) string {
		if n <= 0 {
			return tr(s, unlimited)
		}
		return comma(n)
	}
	daysOr := func(n int, forever string) string {
		if n <= 0 {
			return tr(s, forever)
		}
		return fmt.Sprintf(tr(s, "%s days"), comma(n))
	}
	// group heads a run of related settings with a captioned divider. BRE's own
	// Configuration Editor is one long ungrouped list; these dividers are IB's,
	// and they are what makes a screen this dense scannable. Dim, so the figures
	// stay the brightest thing on the panel.
	group := func(caption string) {
		text := tr(s, caption)
		fill := len([]rune(rule)) - len([]rune(text)) - 6
		if fill < 0 {
			fill = 0
		}
		// The leading blank line is what separates one group from the rows above
		// it — and it is also what puts the first group of a panel clear of the
		// pause prompt the previous panel ended on.
		fmt.Fprintf(s, "\n%s── %s %s%s\n",
			dim(ansi.FgBrightCyan), text, strings.Repeat("─", fill), ansi.Reset)
	}

	group("The day and the game")
	row("Turns per day", comma(c.TurnsPerDay))
	row("New realm protection", fmt.Sprintf(tr(s, "%s turns"), comma(c.ProtectionTurns)))
	row("Game length", daysOr(c.GameLength, "Endless"))
	row("Removed if unplayed", daysOr(c.IdleDaysRemove, "Never"))
	group("Land")
	row("Maximum regions", countOr(c.MaxRegions, "Unlimited"))
	row("Land at game start", comma(c.InitialMarketLand))
	row("New land per realm/day", comma(c.LandPerDay))
	row("Region costs", tr(s, c.RegionCosts.String()))
	group("Money")
	row("Maximum tax rate", fmt.Sprintf("%d%%", c.MaxTaxRate))
	row("Crown tax on income", fmt.Sprintf("%d%%", c.PlanetaryTaxRate))
	// Worth a line of its own: gold earned past this is destroyed, and a player
	// who does not know the figure only finds out by losing some.
	// Whole billions, not the 4-decimal display form: the cap is SET in billions,
	// so the fraction is always zeros.
	row("Most gold you can hold", fmt.Sprintf(tr(s, "%sB in hand, and again in the bank"),
		comma(w.MoneyCapBillions())))
	row("Bank interest", fmt.Sprintf(tr(s, "%d%% over 10 days"), c.InterestRate))
	row("Investment rate", investRateStr(s, c))
	row("Food market", foodMarketStr(s, c))
	pause(s)

	group("Forces and trade")
	row("Buy military", tr(s, c.BuyMilitary.String()))
	row("Maintenance costs", tr(s, c.MaintCosts.String()))
	row("Trade costs", tr(s, c.TradeCosts.String()))
	group("Attacking")
	row("Attack damage", tr(s, c.AttackDamage.String()))
	row("Attack rewards", tr(s, c.AttackRewards.String()))
	row("Attacks per day", countOr(c.MaxIndividualAttacks, "Unlimited"))
	row("Attack costs", tr(s, c.AttackCosts.String()))
	row("R5-Slappenheimer", tr(s, c.SlappenheimerHandling.String()))
	if !c.MissileOps {
		row("Missile ops", tr(s, "Disabled"))
	}
	if !c.BombingOps {
		row("Bombing ops", tr(s, "Disabled"))
	}
	row("Pirates", onOffStr(c.Pirates))

	group("This board")
	row("Players per board", countOr(c.MaxPlayers, "Unlimited"))
	row("Inter-BBS play", onOffStr(c.IBBS))
	if c.GameStartDate != "" {
		row("Game starts", c.GameStartDate)
	}
	if c.JoinDate != "" {
		row("Joining closes", c.JoinDate)
	}
	if c.IBBS {
		var boards int
		var declaration string
		w.With(func() {
			boards = len(w.LeagueNodes)
			declaration = w.LeagueDiplomacy
		})
		group("The league")
		// The name its Coordinator gave the league, when there is one. Nothing
		// routes by it, so a league that never set one simply has no row rather
		// than a blank.
		if c.LeagueName != "" {
			row("League", c.LeagueName)
		}
		row("This planet", c.BoardID)
		row("Planets in the league", countOr(boards, "Roster not loaded"))
		// The rules above are not this sysop's to set once a board joins a league:
		// the Coordinator broadcasts the whole ruleset (see LeagueConfig), so a
		// player asking their own sysop to change one is asking the wrong person.
		row("Rules come from", tr(s, "The league Coordinator"))
		if declaration != "" {
			row("League declaration", declaration)
		}
		// The interplanetary rules only mean anything once this board is in a
		// league, so they stay hidden on a stand-alone board.
		group("Interplanetary")
		row("Group attacks per day", countOr(c.MaxGroupAttacks, "Unlimited"))
		row("Terrorist ops per day", countOr(c.MaxTerrorOps, "Unlimited"))
		row("Bombing ops per day", countOr(c.MaxBombingOps, "Unlimited"))
		row("Terrorism costs", tr(s, c.TerrorCosts.String()))
		row("Lost forces return after", lostForcesStr(s, c))
		row("Clingy Annihilator", onOffStr(c.ClingyAnnihilator))
		row("Local attacks", onOffStr(c.LocalAttacks))
		row("Local attack scoring", onOffStr(c.LocalAttackScoring))
		row("Allied market trading", onOffStr(c.IPTrading))
	}
	pause(s)
	return Stay
}

// lostForcesStr says how long a strike waits for a result before its forces are
// given back, or that they never are.
func lostForcesStr(s session.Session, c game.Config) string {
	if c.LostForcesDays <= 0 {
		return tr(s, "Never")
	}
	return fmt.Sprintf(tr(s, "%d days"), c.LostForcesDays)
}

// investRateStr names the investment rate and whether it is pinned there.
func investRateStr(s session.Session, c game.Config) string {
	rate := fmt.Sprintf(tr(s, "%d%% over 10 days"), c.StdInvestRate)
	if c.SteadyInvest {
		return rate + tr(s, " (steady)")
	}
	return rate + tr(s, " (floating)")
}

// foodMarketStr says whether the market's daily supply runs out.
func foodMarketStr(s session.Session, c game.Config) string {
	if c.FoodUnlimited {
		return tr(s, "Unlimited")
	}
	return tr(s, "Limited daily supply")
}

// playerList shows every living empire (Coordinator tool).
// playerListNameWidth holds a realm name AND the online suffix. The column was
// 16 before the suffix existed; keeping it there would have clipped ordinary
// names down to seven characters to make room.
const playerListNameWidth = 25

func playerList(s session.Session, w *ctx) Result {
	type row struct {
		name, owner string
		online      bool
		land, nw    int
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive {
				continue
			}
			rows = append(rows, row{e.Name, e.Owner, e != w.Player() && e.Online(), e.Land, w.NetWorth(e)})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightBlue, tr(s, "Player List"), ansi.Reset)
	fmt.Fprintf(s, "  %-*s %-14s %-8s %s\n", playerListNameWidth, tr(s, "Empire"), tr(s, "Owner"), tr(s, "Land"), tr(s, "Net Worth"))
	for _, r := range rows {
		owner := r.owner
		if owner == "" {
			owner = tr(s, "(AI)")
		}
		// The suffix rides in the name column, so the roster that names who owns
		// each realm also says who is on without moving the columns beside it.
		fmt.Fprintf(s, "  %s %-14s %-8d %d\n", nameCell(s, r.name, "", r.online, playerListNameWidth), owner, r.land, r.nw)
	}
	pause(s)
	return Stay
}

// News-screen chrome, matched to BRE's captured layout (docs/dev/bre-screens.md).
const (
	// newsBannerName is the masthead over the planetary news — IB's own flavor
	// name where BRE has "The Queen's Quadrant". Tunable; a single string.
	newsBannerName = "The Oligarchy Express"
	// newsItemArrow leads each planetary-news item: red "──" + bright-red "─".
	newsItemArrow = ansi.FgRed + "──" + ansi.FgBrightRed + "─" + ansi.Reset
	// newsItemIndent is the hanging indent under that arrow, 5 spaces as BRE
	// wraps it — a column wider than the arrow itself.
	newsItemIndent = "     "
	newsBoxIndent  = "    "
	newsBoxInner   = 54 // printable columns between the box's ║ borders
)

// renderDailyBulletin draws the boxed blue Daily Bulletin: planet-wide totals
// with day-over-day change, colored green/red/neutral for +/-/0. title is the
// board name in a league, or "" for a stand-alone board, which shows just
// "Daily Bulletin".
func renderDailyBulletin(s session.Session, b game.DailyBulletin, title string) {
	head := tr(s, "Daily Bulletin")
	if title != "" {
		head = title + " — " + head
	}
	boxTop(s, head)

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
		lab := fmt.Sprintf("%-18s", tr(s, label)+":")
		val := fmt.Sprintf("%-16s", fmtNum(total))
		chg := sign + fmtNum(abs)
		inner := "  " + ansi.FgWhite + lab + ansi.FgCyan + val + ansi.FgWhite + tr(s, "Change") + ": " + clr + chg + ansi.Reset
		vis := 2 + len([]rune(lab)) + len([]rune(val)) + len([]rune(tr(s, "Change")+": ")) + len([]rune(chg))
		boxRow(s, inner, vis)
	}

	locale := func(n int) string { return formatGold(n, sessionLang(s)) }
	row("Total Population", b.Totals.Population, b.Change.Population, locale)
	row("Total Regions", b.Totals.Regions, b.Change.Regions, locale)
	row("Total Net Worth", b.Totals.NetWorth, b.Change.NetWorth, abbrevMoney)
	boxBottom(s)
}

// boxTop draws the box's top border with head embedded (bright-white) in the ═
// line; boxRow draws one ║…║ content line padded to newsBoxInner (innerVis is
// the printable width of inner, excluding ANSI); boxBottom closes it.
func boxTop(s session.Session, head string) {
	tw := len([]rune(head))
	if tw > newsBoxInner {
		tw = newsBoxInner
	}
	left := (newsBoxInner - tw) / 2
	right := newsBoxInner - tw - left
	fmt.Fprintf(s, "\n%s%s╔%s%s%s%s%s╗%s\n",
		newsBoxIndent, ansi.FgBlue,
		strings.Repeat("═", left),
		ansi.FgBrightWhite, head, ansi.FgBlue,
		strings.Repeat("═", right), ansi.Reset)
}

func boxRow(s session.Session, inner string, innerVis int) {
	pad := newsBoxInner - innerVis
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(s, "%s%s║%s%s%s%s║%s\n",
		newsBoxIndent, ansi.FgBlue, ansi.Reset,
		inner, strings.Repeat(" ", pad),
		ansi.FgBlue, ansi.Reset)
}

func boxBottom(s session.Session) {
	fmt.Fprintf(s, "%s%s╚%s╝%s\n",
		newsBoxIndent, ansi.FgBlue, strings.Repeat("═", newsBoxInner), ansi.Reset)
}

// renderNewsMasthead prints the news screen's header: the program/version
// "News File" line with the date right-aligned, the centered banner, and a
// yellow rule — matched to BRE's captured layout.
func renderNewsMasthead(s session.Session, date string) {
	headVis := len([]rune(tr(s, "Immortal Barons") + " v" + game.Version + tr(s, ": News File")))
	fmt.Fprintf(s, "\n%s%s %sv%s%s%s",
		ansi.FgBrightYellow, tr(s, "Immortal Barons"),
		ansi.FgBrightWhite, game.Version,
		ansi.FgWhite, tr(s, ": News File"))
	if date != "" {
		pad := len([]rune(rule)) - headVis - len([]rune(date))
		if pad < 1 {
			pad = 1
		}
		fmt.Fprintf(s, "%s%s", strings.Repeat(" ", pad), date)
	}
	fmt.Fprintf(s, "%s\n", ansi.Reset)

	banner := ansi.FgRed + "──" + ansi.FgBrightRed + "»" + ansi.FgBrightWhite + tr(s, newsBannerName) + ansi.FgBrightRed + "«" + ansi.FgRed + "──" + ansi.Reset
	bannerVis := 2 + 1 + len([]rune(tr(s, newsBannerName))) + 1 + 2
	indent := (len([]rune(rule)) - bannerVis) / 2
	if indent < 0 {
		indent = 0
	}
	fmt.Fprintf(s, "\n%s%s\n", strings.Repeat(" ", indent), banner)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, rule, ansi.Reset)
}

// hiTerm is a name the news highlighter should color where it appears.
type hiTerm struct{ text, color string }

// hiNewsItem colors one planetary-news line the way BRE does: known names
// (empires, pirate factions, the Planetary Master title) in their accent color
// and digit runs in bright-white, over a white body. Single left-to-right pass
// matching the LONGEST term at each position, so "Iron Dominion" is not split by
// a shorter "Iron"; ANSI codes it inserts never contain a term, so there is no
// re-match. Replaces the numbers-only hiNums for news text.
func hiNewsItem(line string, terms []hiTerm) string {
	sort.SliceStable(terms, func(i, j int) bool { return len(terms[i].text) > len(terms[j].text) })
	runes := []rune(line)
	var b strings.Builder
	b.WriteString(ansi.FgWhite)
	for i := 0; i < len(runes); {
		matched := false
		for _, t := range terms {
			tr := []rune(t.text)
			if t.text != "" && i+len(tr) <= len(runes) && string(runes[i:i+len(tr)]) == t.text {
				b.WriteString(t.color + t.text + ansi.FgWhite)
				i += len(tr)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if isDigit(runes[i]) {
			j := i
			for j < len(runes) && (isDigit(runes[j]) || (runes[j] == ',' && j+1 < len(runes) && isDigit(runes[j+1]))) {
				j++
			}
			b.WriteString(ansi.FgBrightWhite + string(runes[i:j]) + ansi.FgWhite)
			i = j
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	b.WriteString(ansi.Reset)
	return b.String()
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// newsHighlightTerms is the set of names to color in planetary-news lines:
// every empire in bright-yellow, the pirate factions in bright-red, and the
// Planetary Master title in bright-white. Read off the live captures in `cap/`,
// where three different realms in one news screen all render `1;33` — BRE gives
// the reader's own realm no distinct color here, unlike the recap (bright-cyan)
// or the status screens. Factions are the one deliberate divergence: BRE uses plain
// `31`, which is 2.71:1 on black and under the legibility floor, so IB brightens
// to `1;31` within the same hue (docs/dev/bre-screens.md records it).
func newsHighlightTerms(w *ctx) []hiTerm {
	var terms []hiTerm
	w.With(func() {
		for _, e := range w.Empires {
			terms = append(terms, hiTerm{e.Name, ansi.FgBrightYellow})
		}
	})
	for _, f := range game.PirateFactions {
		terms = append(terms, hiTerm{f, ansi.FgBrightRed})
	}
	terms = append(terms, hiTerm{"Planetary Master", ansi.FgBrightWhite})
	return terms
}

// newsDate formats an ISO date (2006-01-02) as BRE's M-D-YYYY, or "" when empty
// or unparseable (the masthead then omits the date).
func newsDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d-%d", int(t.Month()), t.Day(), t.Year())
}

// prevISODate returns the ISO day before iso (for the Yesterday's News header),
// or "" if unparseable.
func prevISODate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

// empireStatus is the standalone status action (System menu): draw the Empire
// Status block, then pause so it is read before the menu redraws over it.
func empireStatus(s session.Session, w *ctx) Result {
	renderEmpireStatus(s, w)
	pause(s)
	return Stay
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
		name                    string
		alive, isPlayer, online bool
		land, score, nw         int
	}
	var rows []row
	var lastMaster string
	w.With(func() {
		rows = make([]row, 0, len(w.Empires))
		for _, e := range w.Empires {
			nw := w.NetWorth(e)
			if !e.Alive {
				nw = 0 // BRE's scores screen values a dead realm at nothing
			}
			// Net Worth is the asset value (land + military). Score is BRE's
			// cumulative metric (Empire.Score): the day-start net worth awarded per
			// turn played, minus small riot/spoilage dings — separate from wealth.
			// Never mark your own realm: you know you are here, and the row is
			// already singled out by its bright-yellow name. Marking it would
			// also be the one row whose stamp is guaranteed fresh, so it would
			// read as the most reliable entry on a screen where it carries no
			// information at all.
			self := e == w.Player()
			rows = append(rows, row{e.Name, e.Alive, self, !self && e.Online(), e.Land, e.Score, nw})
		}
		lastMaster = w.LastMaster
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].score > rows[j].score })

	// BRE-style scores screen (matches a live BRE scores screen): a game-name
	// banner, lettered (A)/(B) ids, Id / Empire Name / Territory / Score /
	// Net Worth columns, magenta header/footer rules. IB-branded.
	fmt.Fprintf(s, "\n%s-*%s%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.Reset, ansi.FgBrightMagenta, ansi.Reset)
	scoreTableHead(s)
	for i, r := range rows {
		name := r.name
		if !r.alive {
			name += " " + tr(s, "(dead)")
		}
		nameColor := ansi.FgBrightWhite
		if r.isPlayer {
			nameColor = ansi.FgBrightYellow // highlight the caller's own realm
		}
		scoreTableRow(s, scoreID(i), name, nameColor, r.online, r.land, r.score, r.nw)
	}
	scoreTableRule(s)
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}

const (
	scoreRuleWidth = 72
	// scoreIDCellWidth is the id column. Three columns is the whole id: a realm
	// is addressed by a letter, and Max Players Per BBS is capped at 26 for that
	// reason (game.MaxPlayersPerBoard), so `(A)`..`(Z)` is every id there is. The
	// attack picker leaves it EMPTY for a realm that cannot be attacked, which
	// this still holds in place. A board configured "unlimited" can hand out
	// `(27)`, which widens its own row and moves nothing else.
	scoreIDCellWidth = 3
	// scoreNameWidth is the name column. It carries the online suffix too, so
	// the figures beside it do not move when a baron comes or goes.
	scoreNameWidth  = 26
	scoreOnlineMark = "O"
)

// onlineMark renders the indicator, or a blank of the same width. Both the
// score tables and the Relations roster call it, so the two cannot drift into
// showing the same thing two ways.
//
// It is drawn as an inverse-video cell — a lit key against the dim table —
// rather than a colored glyph. Reverse plus a bright FOREGROUND is what gets a
// bright background here: the `10x` bright-background codes are an aixterm
// extension, and on the classic CP437 clients a BBS door actually meets, that
// high-intensity background bit means blink. Black on bright yellow measures
// 19.7:1 against the VGA palette and 19.6:1 against xterm's; plain `43m` yellow
// would be 4.0:1 on VGA and fail. The `O` still carries the meaning on its own
// for a monochrome or ANSI-less session.
// onlineMark marks a baron who is on the board: "(O)" set immediately to the
// LEFT of their name, or a blank of the same width so the name column holds in
// both states.
//
// The LETTER takes the lighter gray and the parens the darker one, not the other
// way round. Measured against a black background, gray (37) is 9.04:1 on the
// VGA/CP437 palette and 11.54:1 on xterm's, while dark gray (90) is 2.82:1 and
// 5.32:1 — so on the CP437 clients a BBS door actually meets, dark gray misses
// the 4.5:1 target by a wide margin. The parens are decoration and can carry
// that; the letter cannot, because it is the whole message.
// markWidth is the mark's column cost — "(O)" and whatever a translation makes
// of the letter — so headings and blanks can be sized from one place.
func markWidth(s session.Session) int { return len([]rune(tr(s, scoreOnlineMark))) + 2 }

func onlineMark(s session.Session, online bool) string {
	if !online {
		return strings.Repeat(" ", markWidth(s))
	}
	return ansi.FgBrightBlack + "(" + ansi.FgWhite + tr(s, scoreOnlineMark) +
		ansi.FgBrightBlack + ")" + ansi.Reset
}

// nameCell renders the online mark hugging a realm's name on the left, then
// pads the pair to width. It measures the PLAIN text: the mark carries escapes
// that a width verb counts as characters, which is what would otherwise ragged
// the column beside it.
//
// It also CLIPS a name too long for the cell. Nothing caps a realm's name at
// onboarding — only a three-character minimum is enforced — so a long one used
// to push the figures beside it out of true. The mark survives the clip: it is
// the part carrying information the row cannot show twice.
func nameCell(s session.Session, name, nameColor string, online bool, width int) string {
	mark := markWidth(s)
	if room := width - mark; len([]rune(name)) > room {
		name = string([]rune(name)[:max(room, 0)])
	}
	return onlineMark(s, online) + nameColor + name + ansi.Reset +
		strings.Repeat(" ", max(width-mark-len([]rune(name)), 0))
}

func idCell(id string, idColor string) string {
	return idColor + id + ansi.Reset + strings.Repeat(" ", max(scoreIDCellWidth-len(id), 0))
}

// The scores table is drawn on three screens — the Scores display, the attack
// target picker, and the recipient picker — so its geometry and colors live
// here once. They previously carried a copy each of the same format strings,
// which is how a change to one silently leaves the others behind.
func scoreTableRule(s session.Session) {
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, strings.Repeat("─", scoreRuleWidth), ansi.Reset)
}

func scoreTableHead(s session.Session) {
	// The heading sits over the names, which the online mark's gutter indents.
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, scoreIDCellWidth, tr(s, "Id"),
		scoreNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	scoreTableRule(s)
}

func scoreTableRow(s session.Session, id, name, nameColor string, online bool, land, score, nw int) {
	fmt.Fprintf(s, "%s%s %s%10d%s %s%11d%s %s%11d%s\n",
		idCell(id, ansi.FgBrightMagenta),
		nameCell(s, name, nameColor, online, scoreNameWidth),
		ansi.FgBrightMagenta, land, ansi.Reset,
		ansi.FgBrightWhite, score, ansi.Reset,
		ansi.FgWhite, nw, ansi.Reset)
}

// scoreID is the lettered id for a scores row — (A), (B), … (Z), then (27)+ for
// the rare game with more than 26 realms.
func scoreID(i int) string {
	if i < 26 {
		return fmt.Sprintf("(%c)", 'A'+i)
	}
	return fmt.Sprintf("(%d)", i+1)
}
