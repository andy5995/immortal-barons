package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
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
	// away is this realm's forces committed to group attacks, whether still
	// waiting to leave or in flight. They have left the army, and the Military
	// advisor counts them in its totals as the original does (#239).
	away game.AttackForce
}

func gatherAdvisorData(w *ctx) advisorData {
	var d advisorData
	w.Read(func() {
		d.p = *w.Player()
		d.foodGrown = w.FoodGrown(&d.p)
		d.foodEaten = w.FoodDue(&d.p)
		d.foodAtCap = d.p.FoodUpkeepAtCapacity()
		d.income = w.IncomeThisTurn(&d.p).Gold()
		d.away = w.ForcesAway(d.p.Owner)
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
	// Advice marks a line that suggests something rather than reports it. The
	// advice is gathered below the figures, each line marked with a », so a
	// reader can find what to do without reading the report again.
	Advice bool
	// Raw is laid out already (the Military advisor's force rows) and is
	// printed as it stands, never wrapped.
	Raw bool
	// Break starts a new group of figures: a blank line goes before it, so a
	// report reads as a few short blocks rather than one run of lines.
	Break bool
}

// hiTerms replaces each {braced} run in s with emph-colored text, returning to
// base afterward. Runs after word-wrapping (wrapTerms keeps a braced phrase on
// one line) and before hiNumsReset, which passes the escape sequences this
// inserts through untouched.
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
	// brk is add, starting a new group of figures.
	brk := func(text string) { out = append(out, advisorLine{Text: text, Hi: fig, Emph: emph, Break: true}) }
	// warn is a figure line whose figure is flagged bright-yellow.
	warn := func(text string) { out = append(out, advisorLine{Text: text, Hi: ansi.FgBrightYellow, Emph: emph}) }
	advise := func(text string) {
		out = append(out, advisorLine{Text: text, Hi: ansi.FgBrightYellow, Emph: emph, Advice: true})
	}
	note := func(text string) { out = append(out, advisorLine{Text: text, Hi: fig, Note: true}) }
	switch dom {
	case advisorCivilian:
		add(fmt.Sprintf(tr(s, "Our people number %s, and their support stands at %d%%."), count(p.People), p.Support))
		brk(fmt.Sprintf(tr(s, "We grow %s units of food each turn, and our people eat %s."), count(d.foodGrown), count(d.foodEaten)))
		add(fmt.Sprintf(tr(s, "Our stores hold %s units of food."), count(p.Food)))
		net := d.foodGrown - d.foodEaten
		// Food is credited at turn start, so p.Food already includes this turn's
		// growth. The projections below are written against the pre-growth stock, so
		// recover it (p.Food - foodGrown) rather than counting the growth twice.
		stock := p.Food - d.foodGrown
		switch {
		case stock+net < 0:
			// Even with this turn's growth already in, stores can't cover this turn's
			// consumption, so the turn ends with negative food (turn.go starvation step).
			advise(tr(s, "Our food will not last the turn. Buy or grow more."))
		case net < 0:
			warn(fmt.Sprintf(tr(s, "We run a shortfall of %s; our stores will run out in about %d turns."), count(-net), stock/(-net)))
		case d.foodAtCap > d.foodGrown:
			// Fed now, but the populace is still growing toward a support-driven
			// capacity whose food need outruns production (see issue #35).
			advise(fmt.Sprintf(tr(s, "We have a surplus now, but our people are still growing. At full size they will eat about %s food each turn, more than we grow. Add agricultural regions before then."), count(d.foodAtCap)))
		default:
			// The food bottom line pops in yellow whether short or in surplus (BRE).
			warn(fmt.Sprintf(tr(s, "That leaves %s to spare each turn."), count(net)))
		}
		if p.Support < 50 {
			advise(tr(s, "Popular support is low. It cuts our coastal income and the number of people our land can hold. Lower taxes or spend on their support."))
		}
		if pct := game.RiotChancePct(p.Tax); pct > 0 {
			advise(fmt.Sprintf(tr(s, "At a %d%% tax rate, a riot breaks out in about %d%% of turns. Each riot costs us people and support."), p.Tax, pct))
		}
	case advisorEconomic:
		add(fmt.Sprintf(tr(s, "We have %s gold in hand and %s in the bank."), num(p.Gold), num(p.Bank)))
		if p.Debt > 0 {
			add(fmt.Sprintf(tr(s, "We owe %s gold on loans. The bank takes a payment each turn, and what is unpaid at the end of the day grows."), num(p.Debt)))
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
			advise(tr(s, "Our treasury is empty, Sire. We should raise gold soon."))
		}
	case advisorMilitary:
		for _, row := range forceTable(s, p, d.away) {
			out = append(out, advisorLine{Text: row, Raw: true})
		}
		if away := d.away.Units(); away > 0 {
			add(fmt.Sprintf(tr(s, "Of these, %s units are away on attacks."), count(away)))
		}
		switch {
		case p.HQ == 0:
			// The price climbs with every turn played (World.HQPrice), so "soon" is
			// the actionable half of this advice. The figure itself belongs to the
			// Spending Menu, which quotes the live price.
			advise(tr(s, "We have no {HeadQuarters}. Building one would strengthen our {tanks}, and it costs more with every turn we wait."))
		case p.HQ < 100:
			brk(fmt.Sprintf(tr(s, "Our {HeadQuarters} is %d%% built."), p.HQ))
		default:
			brk(tr(s, "Our {HeadQuarters} is fully built."))
		}
		if int64(p.Carriers)*game.JetsPerCarrier*100 < int64(p.Jets)*game.AdvisorCarrierWarnPct {
			advise(tr(s, "We have more {jets} than our {carriers} can carry. Build more {carriers}."))
		}
		mtn := game.MountainIndustryPercent(p.Regions)
		switch {
		case p.Regions.Mountain == 0:
			advise(tr(s, "We hold no {mountain} regions. Their ore would make our factories build units faster."))
		case mtn >= game.MountainIndustryCapPct:
			brk(fmt.Sprintf(tr(s, "Our {mountain} regions have the foundries at their limit, %d%% of normal unit output."), mtn))
		default:
			brk(fmt.Sprintf(tr(s, "With our {mountain} regions, our factories build units at %d%% of normal. The figure depends on their share of our land, so buying other regions lowers it."), mtn))
		}
		brk(fmt.Sprintf(tr(s, "Troop morale stands at %d%%."), p.Morale))
		if p.Morale < game.MoraleDesertBandTop {
			advise(tr(s, "Morale is low, so some of our {troopers}, {jets} and {tanks} may desert each turn."))
		}
		if p.Agents == 0 {
			advise(tr(s, "We have no {covert agents}. Recruit some for spying and sabotage."))
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
			advise(tr(s, "Building some would raise our military strength, income, and food output, and lower our upkeep — a benefit that builds up over time."))
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

// advisorWidth is the advisor box: the title and closing rules, and the width
// every line inside them is wrapped to. BRE draws its advisors as bare prose with
// no box; IB frames them like its menus so a report reads as one screen.
const advisorWidth = 76

// advisorTitle is the advisor's label on the Advisors menu, reused as the box
// title so the two always agree.
func advisorTitle(d advisorDomain) string {
	return [...]string{advisorCivilian: "Civilian", advisorEconomic: "Economic",
		advisorMilitary: "Military", advisorTechnology: "Technology"}[d]
}

// forceTable lays the Military advisor's unit counts out the way Empire Status
// does, with its own row code: a Military label, then [count Unit] cells,
// three to a line. The counts include forces away on attacks and are shortened
// to k and m, both as the original's report has them (#239).
func forceTable(s session.Session, p *game.Empire, away game.AttackForce) []string {
	label, indent := statusRowPrefix(s, "Military")
	var cells []statusItem
	for _, g := range []*game.Good{game.Trooper, game.Jet, game.Turret, game.Tank, game.Bomber, game.Carrier} {
		n := *g.Count(p)
		if g.Force != nil {
			n += *g.Force(&away)
		}
		if n > 0 {
			cells = append(cells, statusCell(numfmt.Short(n), tr(s, g.Plural)))
		}
	}
	const lead = "    "
	if len(cells) == 0 {
		return []string{lead + label + tr(s, "None")}
	}
	return statusRows(lead+label, lead+indent, cells, statusMilitaryPerLine)
}

// wrapTerms word-wraps an advisor line to width and hangs it: first leads the
// first line, cont every line after, and paint colors the text between. Only
// what shows is counted: the {braces} that mark a key term take no column, and a
// braced term is never split across two lines, since hiTerms can only color a
// term whose braces share a line.
func wrapTerms(text string, width int, first, cont string, paint func(string) string) string {
	var words []string
	depth := 0
	start := 0
	for i, r := range text {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		case ' ':
			if depth == 0 {
				words = append(words, text[start:i])
				start = i + 1
			}
		}
	}
	words = append(words, text[start:])
	shown := func(w string) int { return len([]rune(w)) - strings.Count(w, "{") - strings.Count(w, "}") }
	var lines []string
	var cur string
	n := 0
	for _, w := range words {
		if w == "" {
			continue
		}
		if n > 0 && n+1+shown(w) > width {
			lines = append(lines, cur)
			cur, n = "", 0
		}
		if n > 0 {
			cur += " "
			n++
		}
		cur += w
		n += shown(w)
	}
	lines = append(lines, cur)
	for i, l := range lines {
		lead := cont
		if i == 0 {
			lead = first
		}
		lines[i] = lead + paint(l)
	}
	return strings.Join(lines, "\n")
}

// renderAdvisor prints one advisor's report in a box: the greeting, the figures,
// then any advice, each part set apart by a blank line. Split from the menu loop
// so tests can render an advisor without a pause.
func renderAdvisor(s session.Session, w *ctx, d advisorDomain) {
	data := gatherAdvisorData(w)
	fmt.Fprintf(s, "\n%s\n", titleRule(ansi.FgMagenta, tr(s, advisorTitle(d)), advisorWidth))
	greet := func(t string) string { return ansi.FgBrightCyan + t + ansi.Reset }
	fmt.Fprintln(s, wrapTerms(advisorGreeting(s, d), advisorWidth-2, "  ", "  ", greet))
	// Body text is regular/off-white (37); the figures are the only bright things
	// on the line — bright-white (97) or yellow (93) per the line's Hi — so they
	// pop, the way BRE's advisors read (docs/dev/bre-screens.md). Without the dim
	// base, bright-white figures would blend into a terminal's default-white text.
	base := ansi.FgWhite
	body := func(line advisorLine, text string) string {
		return base + hiNumsReset(hiTerms(text, line.Emph, base), line.Hi, base) + ansi.Reset
	}
	// Every wrapped line hangs its continuation under the text, so a sentence
	// that runs onto a second line is not read as the next one.
	hang := func(line advisorLine, first string) {
		fmt.Fprintln(s, wrapTerms(line.Text, advisorWidth-4, first, "    ", func(t string) string { return body(line, t) }))
	}
	lines := advisorReport(s, data, d)
	fmt.Fprint(s, "\n")
	firstFact := true
	for _, line := range lines {
		if line.Advice || line.Note {
			continue
		}
		if line.Break && !firstFact {
			fmt.Fprint(s, "\n")
		}
		firstFact = false
		if line.Raw {
			// Empire Status's own cells, colored as they are there.
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, line.Text, ansi.Reset)
			continue
		}
		hang(line, "    ")
	}
	marker := ansi.FgBrightMagenta + "»" + ansi.Reset + " "
	first := true
	for _, line := range lines {
		if !line.Advice {
			continue
		}
		if first {
			fmt.Fprint(s, "\n")
			first = false
		}
		// "  » " is four columns, the same as the hang, so every line of the
		// advice starts at one column.
		hang(line, "  "+marker)
	}
	for _, line := range lines {
		if !line.Note {
			continue
		}
		// BRE's own shape: a blank line, "NOTE:" in bright cyan, and the body
		// in cyan hanging under the label.
		fmt.Fprint(s, "\n")
		label := "  " + ansi.FgBrightCyan + "NOTE:" + ansi.Reset + " "
		cyan := func(t string) string { return ansi.FgCyan + t + ansi.Reset }
		fmt.Fprintln(s, wrapTerms(line.Text, advisorWidth-10, label, "        ", cyan))
	}
	fmt.Fprintln(s, closingRule(ansi.FgMagenta, advisorWidth))
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
		for d := advisorCivilian; d <= advisorTechnology; d++ {
			item(int(d), advisorTitle(d))
		}
		item(0, "Quit")
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
		n := ChoiceQuit(s, int(advisorTechnology))
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
