package menu

import (
	"fmt"
	"sort"
	"strings"

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
		d.foodEaten = d.p.FoodUpkeep()
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

// advisorGreeting is the advisor's first-person opening line (BRE's advisors
// speak in first person, e.g. "Hi, I'm your military advisor").
func advisorGreeting(s session.Session, d advisorDomain) string {
	switch d {
	case advisorCivilian:
		return tr(s, "I am your Civilian advisor, Sire.")
	case advisorEconomic:
		return tr(s, "I am your Economic advisor, Sire.")
	case advisorMilitary:
		return tr(s, "I am your Military advisor, Sire.")
	default:
		return tr(s, "I am your Technology advisor, Sire.")
	}
}

// advisorReport builds the lines one advisor speaks: the figures for its domain
// (mirroring BRE's advisor reports — the populace and food, the treasury and
// income, the armed forces, and technology's effects) plus any advice that
// applies. Kept separate from rendering so tests can inspect the lines.
func advisorReport(s session.Session, d advisorData, dom advisorDomain) []string {
	num := func(n int) string { return formatGold(n, sessionLang(s)) }
	p := &d.p
	var out []string
	switch dom {
	case advisorCivilian:
		out = append(out, fmt.Sprintf(tr(s, "Our people number %s, and their support stands at %d%%."), num(p.People), p.Support))
		out = append(out, fmt.Sprintf(tr(s, "We grow %s food each turn and consume %s."), num(d.foodGrown), num(d.foodEaten)))
		net := d.foodGrown - d.foodEaten
		// Food is credited at turn start, so p.Food already includes this turn's
		// growth. The projections below are written against the pre-growth stock, so
		// recover it (p.Food - foodGrown) rather than counting the growth twice.
		stock := p.Food - d.foodGrown
		switch {
		case stock+net < 0:
			// Even with this turn's growth already in, stores can't cover this turn's
			// consumption, so the turn ends with negative food (turn.go starvation step).
			out = append(out, tr(s, "Our food will not last the turn. Buy or grow more."))
		case net < 0:
			out = append(out, fmt.Sprintf(tr(s, "We run a shortfall of %s; our stores will run out in about %d turns."), num(-net), stock/(-net)))
		case d.foodAtCap > d.foodGrown:
			// Fed now, but the populace is still growing toward a support-driven
			// capacity whose food need outruns production (see issue #35).
			out = append(out, fmt.Sprintf(tr(s, "We have a surplus now, but our people are still growing. At full size they will eat about %s food each turn, more than we grow. Add agricultural regions before then."), num(d.foodAtCap)))
		default:
			out = append(out, fmt.Sprintf(tr(s, "That leaves a surplus of %s. Our stores are secure."), num(net)))
		}
		if p.Support < 50 {
			out = append(out, tr(s, "The people grow restless. Lower taxes or spend on their support."))
		}
		if p.Tax > 20 {
			out = append(out, tr(s, "Taxes are high enough to risk riots. Consider lowering them."))
		}
	case advisorEconomic:
		out = append(out, fmt.Sprintf(tr(s, "Our treasury holds %s gold, with %s more in the bank."), num(p.Gold), num(p.Bank)))
		if p.Debt > 0 {
			out = append(out, fmt.Sprintf(tr(s, "We owe %s gold in debt, which grows each turn."), num(p.Debt)))
		}
		share := 0
		if d.worldIncome > 0 {
			share = d.income * 100 / d.worldIncome
		}
		out = append(out, fmt.Sprintf(tr(s, "We earn about %s gold each turn, %d%% of the world total."), num(d.income), share))
		perRegion, avg := 0, 0
		if p.Land > 0 {
			perRegion = d.income / p.Land
		}
		if d.worldLand > 0 {
			avg = d.worldIncome / d.worldLand
		}
		out = append(out, fmt.Sprintf(tr(s, "That is %s gold per region; the world average is %s."), num(perRegion), num(avg)))
		if p.Gold <= 0 && p.Bank <= 0 {
			out = append(out, tr(s, "Our treasury is empty, Sire. We should raise gold soon."))
		}
	case advisorMilitary:
		out = append(out, fmt.Sprintf(tr(s, "Our forces: %s troopers, %s jets, %s turrets, %s tanks, %s bombers, %s carriers."),
			num(p.Troopers), num(p.Jets), num(p.Turrets), num(p.Tanks), num(p.Bombers), num(p.Carriers)))
		switch {
		case p.HQ == 0:
			out = append(out, tr(s, "We have no HeadQuarters. Building one would strengthen our tanks."))
		case p.HQ < 100:
			out = append(out, fmt.Sprintf(tr(s, "Our HeadQuarters is %d%% built."), p.HQ))
		default:
			out = append(out, tr(s, "Our HeadQuarters is fully built."))
		}
		if p.Carriers*100 < p.Jets {
			out = append(out, tr(s, "We have more jets than our carriers can carry. Build more carriers."))
		}
		out = append(out, fmt.Sprintf(tr(s, "Troop morale stands at %d%%."), p.Morale))
		if p.Morale < 50 {
			out = append(out, tr(s, "Morale is low. Desertion is a real risk before our next battle."))
		}
		if p.Agents == 0 {
			out = append(out, tr(s, "We have no covert agents. Recruit some for spying and sabotage."))
		} else {
			out = append(out, fmt.Sprintf(tr(s, "We keep %s covert agents."), num(p.Agents)))
		}
	case advisorTechnology:
		tf := p.TechFactor()
		switch {
		case p.Regions.Technology == 0:
			out = append(out, tr(s, "We have no Technology regions."))
			out = append(out, tr(s, "Building some would raise our military strength, income, and food output, and lower our upkeep — a benefit that builds up over time."))
		case tf == 0:
			out = append(out, tr(s, "Our Technology regions are new. Their benefits will build up as we hold them."))
		default:
			out = append(out, fmt.Sprintf(tr(s, "Technology stands at %d%%."), tf))
			out = append(out, fmt.Sprintf(tr(s, "It raises our military strength, income, and food output by %d%%."), tf))
			out = append(out, fmt.Sprintf(tr(s, "It lowers unit and region upkeep, and food spoilage, to %d%% of normal."), 100-tf))
			out = append(out, tr(s, "The bonus builds up the longer we hold Technology regions."))
		}
	}
	return out
}

// renderAdvisor prints one advisor's greeting and its report. Split from the
// menu loop so tests can render an advisor without a pause.
func renderAdvisor(s session.Session, w *ctx, d advisorDomain) {
	data := gatherAdvisorData(w)
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, advisorGreeting(s, d), ansi.Reset)
	for _, line := range advisorReport(s, data, d) {
		// Word-wrap each report line to the screen width (78) less the 2-space
		// indent, so a long sentence breaks at spaces instead of mid-word at col 80.
		for _, wl := range strings.Split(help.Wrap(line, 76), "\n") {
			fmt.Fprintf(s, "  %s\n", wl)
		}
	}
}

// advisorsMenu is BRE's four-advisor submenu: pick an advisor to hear that
// domain's counsel, or 0 to leave. Shared by the System menu's "Visit Advisors"
// action and the Buy Regions "(*) Advisors" entry.
func advisorsMenu(s session.Session, w *ctx) {
	for {
		titleBar(s, tr(s, "Visit Advisors"))
		fmt.Fprintf(s, "  1) %s\n", tr(s, "Civilian"))
		fmt.Fprintf(s, "  2) %s\n", tr(s, "Economic"))
		fmt.Fprintf(s, "  3) %s\n", tr(s, "Military"))
		fmt.Fprintf(s, "  4) %s\n", tr(s, "Technology"))
		fmt.Fprintf(s, "  0) %s\n", tr(s, "Quit"))
		n := choiceQuit(s, 4)
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
	titleBar(s, tr(s, "About"))
	fmt.Fprintf(s, "  %s\n", "Immortal Barons v"+game.VersionString())
	fmt.Fprintf(s, "  %s\n", "https://andy5995.github.io/immortal-barons/")
	fmt.Fprintf(s, "  %s\n", tr(s, "An independent tribute to Barren Realms Elite (BRE), created by Mehul Patel and later maintained by John Dailey. No original BRE code, text, or art is used."))
	fmt.Fprintf(s, "  %s\n", "Free software under the MIT License. Copyright (c) 2026 Andy Alt.")
	pause(s)
	return Stay
}

// gameSetup shows the current game rules (read-only; the sysop edits them with
// the -reset Configuration Editor).
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

	locale := func(n int) string { return formatGold(n, sessionLang(s)) }
	row("Total Population", b.Totals.Population, b.Change.Population, locale)
	row("Total Regions", b.Totals.Regions, b.Change.Regions, locale)
	row("Total Net Worth", b.Totals.NetWorth, b.Change.NetWorth, abbrevMoney)
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
		name            string
		alive, isPlayer bool
		land, score, nw int
	}
	var rows []row
	var lastMaster string
	w.With(func() {
		rows = make([]row, 0, len(w.Empires))
		for _, e := range w.Empires {
			nw := w.NetWorth(e)
			// Net Worth is the asset value (land + military). Score is BRE's
			// cumulative metric (Empire.Score): the day-start net worth awarded per
			// turn played, minus small riot/spoilage dings — separate from wealth.
			rows = append(rows, row{e.Name, e.Alive, e == w.Player(), e.Land, e.Score, nw})
		}
		lastMaster = w.LastMaster
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].score > rows[j].score })

	// BRE-style scores screen (matches a live BRE scores screen): a game-name
	// banner, lettered (A)/(B) ids, Id / Empire Name / Territory / Score /
	// Net Worth columns, magenta header/footer rules. IB-branded.
	rule := strings.Repeat("─", 72)
	fmt.Fprintf(s, "\n%s-*%s%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.Reset, ansi.FgBrightMagenta, ansi.Reset)
	fmt.Fprintf(s, "%s%-4s %-26s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, tr(s, "Id"), tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
	for i, r := range rows {
		name := r.name
		if !r.alive {
			name += " " + tr(s, "(dead)")
		}
		nameColor := ansi.FgBrightWhite
		if r.isPlayer {
			nameColor = ansi.FgBrightYellow // highlight the caller's own realm
		}
		fmt.Fprintf(s, "%s%-4s%s %s%-26s%s %s%10d%s %s%11d%s %s%11d%s\n",
			ansi.FgBrightMagenta, scoreID(i), ansi.Reset,
			nameColor, name, ansi.Reset,
			ansi.FgBrightMagenta, r.land, ansi.Reset,
			ansi.FgBrightWhite, r.score, ansi.Reset,
			ansi.FgWhite, r.nw, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}

// scoreID is the lettered id for a scores row — (A), (B), … (Z), then (27)+ for
// the rare game with more than 26 realms.
func scoreID(i int) string {
	if i < 26 {
		return fmt.Sprintf("(%c)", 'A'+i)
	}
	return fmt.Sprintf("(%d)", i+1)
}
