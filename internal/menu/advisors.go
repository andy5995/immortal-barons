package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

// advisors.go — the four named advisors and the report each gives. They read
// the realm and say what they make of it; none of them changes anything.

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
		mtn := game.MountainIndustryPercent(p.Regions)
		switch {
		case p.Regions.Mountain == 0:
			add(tr(s, "We hold no {mountain} regions. Their ore would speed the foundries; without it our factories build at plain rate."))
		case mtn >= game.MountainIndustryCapPct:
			add(fmt.Sprintf(tr(s, "Our {mountain} regions have the foundries at their limit, %d%% of normal unit output."), mtn))
		default:
			add(fmt.Sprintf(tr(s, "Our {mountain} regions build our units at %d%% of normal output. It is their share of the realm that sets this, so buying land elsewhere thins the gain."), mtn))
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
