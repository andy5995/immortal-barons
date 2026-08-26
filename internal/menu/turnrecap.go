package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// turnrecap.go — what a player is shown when their turn opens and when it
// closes: the "since your last play" recap, the planetary bulletin, the mail
// waiting for them, and the income and end-of-turn reports.

// showBulletinToday is the Today's News menu action.
func showBulletinToday(s session.Session, w *ctx) Result { return showBulletin(s, w, false) }

// showBulletinYesterday is the Yesterday's News menu action.
func showBulletinYesterday(s session.Session, w *ctx) Result { return showBulletin(s, w, true) }

// showBulletin prints the Daily Bulletin header plus that day's planetary
// news lines, or a note if there is no news. yesterday selects
// BulletinYesterday/NewsYesterday instead of today's.
func showBulletin(s session.Session, w *ctx, yesterday bool) Result {
	var bulletin game.DailyBulletin
	var news []string
	var boardID, date string
	w.With(func() {
		if yesterday {
			bulletin, news = w.BulletinYesterday, w.NewsYesterday
		} else {
			bulletin, news = w.BulletinToday, w.NewsToday
			// Totals are recomputed live rather than trusted from the snapshot:
			// rollNews only takes one at daily maintenance, so on a board's first
			// day (or for any realm created since the last maintenance) the
			// snapshot is still zero-valued while living empires already exist
			// (#109). Change stays the frozen day-over-day delta rollNews took.
			bulletin.Totals = w.PlanetTotals()
		}
		// Only name the board in a league, where several planets file news into
		// one feed. On a stand-alone board the prefix is noise — and the default
		// name makes it read "local — Daily Bulletin" (#68).
		if w.Config.IBBS {
			boardID = w.Config.BoardID
		}
		date = w.LastMaintDate
	})
	if yesterday {
		date = prevISODate(date)
	}
	renderNewsMasthead(s, w.Term, newsDate(date))
	renderDailyBulletin(s, w.Term, bulletin, boardID)
	if len(news) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "No planetary bulletins."))
	} else {
		// Each item led by the red arrow, blank-line separated, with empire /
		// faction / number highlights over a white body (BRE layout).
		terms := newsHighlightTerms(w)
		for _, b := range news {
			// Wrapped continuation lines indent 5 spaces, as BRE draws them
			// (docs/dev/bre-screens.md). Wrap before colouring: hiNewsItem's
			// escapes are invisible on screen but count against the margin.
			fmt.Fprintf(s, "\n%s %s\n", newsItemArrow, hiNewsItem(wrapHanging(b, "", newsItemIndent), terms))
		}
	}
	pause(s)
	return Stay
}

// openTurnRecap runs the "since your last play" opening in BRE's order, read out
// of run_player_turn (BRE.EXE 0x36E1): it prints the header, then calls
// process_trade_offer (0x3855), then process_diplomatic_proposal (0x385A), then
// write_data_report (0x385F) for the numbered entries, then read_local_messages
// (0x3869). So a pending offer is answered BEFORE the recap entries it sits
// above, and the trade barter comes before the treaty. IB ran both the other way
// round until this was read.
//
// It is one function rather than four calls in playTurn so the order is
// testable: no capture can settle it, because no BRE screen anywhere shows an
// offer and numbered entries in the same recap.
func openTurnRecap(s session.Session, w *ctx) {
	reviewTradeDeals(s, w)   // pending trade-deal barters (accept/decline)
	reviewTreatyOffers(s, w) // pending treaty offers, with proposer stats
	showTurnEvents(s, w)
	spoilsStage(s, w) // a landed interplanetary strike still needs its region types
}

// showTurnEvents prints and clears the active empire's accumulated events, if
// any. The read and clear happen together under w's lock so a concurrent
// maintenance tick or another session's action can't append between the two,
// and the empire is re-resolved inside the lock so a reload can't rebind it.
func showTurnEvents(s session.Session, w *ctx) {
	var events []game.Event
	var realmNames []string
	withPlayer(w, func(p *game.Empire) {
		events = p.Events
		p.Events = nil
		// The recap consumed everything; re-baseline the mid-session notice so an
		// event appended right after this clear still shows (takeSessionNews's
		// shrink-reset alone couldn't tell it from part of the cleared backlog).
		w.seenEvents, w.seenEventsSet = 0, true
		for _, e := range w.World.Empires {
			if e.Name != "" {
				realmNames = append(realmNames, e.Name)
			}
		}
	})
	if len(events) == 0 {
		return
	}
	// Realm names and treaty types paint bright-cyan, BRE's color for both in
	// this recap (docs/dev/bre-screens.md, "Since your last play" capture).
	recapTokens := append(realmNames, game.TreatyTypes...)
	fmt.Fprintf(s, "\n%s%s%s\n\n", ansi.FgWhite, tr(s, "Since your last play, this has happened:"), ansi.Reset)
	rows := recapHeaderRows
	for i, ev := range countRepeats(events) {
		text := ev.Text
		if ev.Count > 1 {
			text = fmt.Sprintf("%s %s", text, fmt.Sprintf(tr(s, "(%d times)"), ev.Count))
		}
		body := hiTokens(wrapReport(text), recapTokens, ansi.FgBrightCyan)
		// Rule, the body's own lines, and the blank line after it.
		entryRows := 2 + strings.Count(body, "\n")
		if rows+entryRows > recapPageRows {
			pauseTight(s)
			rows = 0
		}
		rows += entryRows
		fmt.Fprintf(s, "%s\n%s\n\n", eventRule(i+1, ev.When), hiNums(body))
	}
	pause(s)
}

// recapPageRows is how much of the recap goes by before it waits for a key, and
// recapHeaderRows is what its own heading has already spent. An 80x24 screen
// with the Paused bar on the last row is what these leave room for; the recap is
// the one screen a player can arrive at with a whole day of entries queued, and
// a terminal without scrollback loses everything above the fold.
const (
	recapPageRows   = 22
	recapHeaderRows = 2
)

// countedEvent is one recap line and how many times its exact text was filed
// since the last play. Repeats are counted rather than listed, which is what the
// original does with a batch of interplanetary terror operations: `ipreport.dat`
// carries a MULTI_ and a SINGLE_ template per outcome and the multi form counts
// the occurrences ("... %N times!"). BRE only does this for that one report —
// its local covert resolver writes a line per operation — so counting the local
// ones too is IB's own, recorded in docs/mechanics-reference.md.
type countedEvent struct {
	game.Event
	Count int
}

// countRepeats folds events sharing the same text into one entry apiece, in the
// order each text first appeared, keeping that first entry's timestamp. Repeats
// need not be adjacent: five covert operations resolving in one maintenance pass
// interleave their successes and failures, and the player wants two lines, not
// five.
func countRepeats(events []game.Event) []countedEvent {
	var out []countedEvent
	at := make(map[string]int, len(events))
	for _, ev := range events {
		if i, seen := at[ev.Text]; seen {
			out[i].Count++
			continue
		}
		at[ev.Text] = len(out)
		out = append(out, countedEvent{Event: ev, Count: 1})
	}
	return out
}

// Recap-rule geometry, measured off a live BRE capture: the rule runs 76 columns
// and the timestamp always starts at column 48.
const (
	eventRuleWidth   = 76
	eventStampColumn = 48
)

// eventRule is the numbered, timestamped rule BRE draws above each recap entry:
//
//	─────(1)────────────────────────────────────────07/31/2026  07:43:11────────
//
// The stamp sits at a fixed column so the times line up however many entries
// there are; a world saved before events carried one draws an unbroken rule.
// The capture only shows single-digit counters, so which of the two the original
// holds fixed past nine — the column or the 76-column width — is a guess.
func eventRule(n int, when time.Time) string {
	head := fmt.Sprintf("─────%s(%s%d%s)", dim(ansi.FgBrightYellow), ansi.FgBrightYellow, n, dim(ansi.FgBrightYellow))
	plain := len([]rune(fmt.Sprintf("─────(%d)", n)))
	stamp := ""
	if !when.IsZero() {
		stamp = when.Format(game.StampFormat)
	}
	fill := eventStampColumn - plain
	if fill < 1 {
		fill = 1
	}
	tail := eventRuleWidth - eventStampColumn - len([]rune(stamp))
	if tail < 0 {
		tail = 0
	}
	return ansi.FgBrightBlack + head + ansi.FgBrightBlack +
		strings.Repeat("─", fill) + stamp + strings.Repeat("─", tail) + ansi.Reset
}

// readTurnMail is the mail stop at the head of a turn. BRE asks nothing: the
// messages simply follow the recap, each read in its own box, and an empty inbox
// says so. IB used to state a count and gate the reader behind "Read them now?
// (Y/n)", which is not what the original does.
//
// announceEmpty is true only for the first turn of a session — the spot BRE
// prints "You have no messages." A later turn re-checks so mail arriving from
// another node mid-session is seen (#3), but stays quiet when there is none
// rather than repeating the line up to ten times a day.
func readTurnMail(s session.Session, w *ctx, announceEmpty bool) {
	var count int
	withPlayer(w, func(p *game.Empire) { count = len(p.Mail) })
	if count == 0 {
		if announceEmpty {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "You have no messages."), ansi.Reset)
		}
		return
	}
	mailReader(s, w)
}

// manufacturedUnits lists what the Industrial Zones built this turn, one unit
// type per line under a heading, and nothing at all on a turn that built
// nothing. A type that produced none is left out, as every other line of this
// report is.
//
// This is a DELIBERATE DIVERGENCE from the original, which ends every one of
// those lines with "were manufactured by Industrial Zones." — five words
// repeated six times for the sake of six figures. Recorded in
// docs/dev/bre-screens.md and docs/mechanics-reference.md; do not "fix" it back
// to match a capture.
func manufacturedUnits(s session.Session, made []int) {
	width := 0
	for _, n := range made {
		if w := len(comma(n)); n > 0 && w > width {
			width = w
		}
	}
	if width == 0 {
		return
	}
	fmt.Fprintf(s, "\n  %s\n", tr(s, "Your Industrial Zones built:"))
	for i, g := range game.MilitaryGoods {
		if made[i] == 0 {
			continue
		}
		name := g.Plural
		if made[i] == 1 {
			name = g.Singular
		}
		fmt.Fprintf(s, "    %s%*s%s  %s\n", ansi.FgBrightCyan, width, comma(made[i]), ansi.Reset, tr(s, name))
	}
}

// incomeReport itemizes p's per-turn income by source. It shows exactly the
// values CollectIncome credits: both derive from World.IncomeThisTurn.
func incomeReport(s session.Session, w *ctx) {
	var b game.IncomeBreakdown
	var raids []game.PirateHit
	var interest, invested int64
	made := make([]int, len(game.MilitaryGoods))
	if !withPlayer(w, func(p *game.Empire) {
		b = w.IncomeThisTurn(p)
		raids = p.PirateHits
		p.PirateHits = nil
		p.RaidersThisTurn = raiderSlots(raids)
		interest = p.LastInterest
		invested = p.InvestReturnsToday
		for i, g := range game.MilitaryGoods {
			made[i] = *g.Made(p)
		}
	}) {
		return
	}

	golds := []struct {
		amount int
		text   string
	}{
		{b.Taxes, "gold was earned in taxes."},
		{b.Ore, "gold was produced from the Ore Mines."},
		{b.Tourism, "gold was earned in Tourism."},
		{b.Solar, "gold was earned by Solar Power Generators."},
		{b.Rivers, "gold was earned from Rivers."},
		{b.Industrial, "gold was earned from Industrial Zones."},
		{b.Trade, "gold was earned from Trade."},
	}
	// Right-align every amount to one column width so the report scans cleanly.
	total := 0
	width := len(comma(b.Food))
	for _, l := range golds {
		if l.amount > 0 {
			total += l.amount
			if w := len(comma(l.amount)); w > width {
				width = w
			}
		}
	}
	for _, n := range []int64{int64(total), interest, invested} {
		if w := len(comma(n)); w > width {
			width = w
		}
	}

	// BRE opens the income lines under its 75-column blue rule and gives them no
	// heading at all (docs/dev/bre-screens.md, "Turn income, status block,
	// maintenance"). IB drew a blue-backed "Income Report" bar of its own here
	// until 2026-08-25.
	fmt.Fprintf(s, "\n%s\n", rule75(ansi.FgBlue))
	amt := func(color string, n int64, text string) {
		fmt.Fprintf(s, "  %s%*s%s  %s\n", color, width, comma(n), ansi.Reset, i18n.T(sessionLang(s), text))
	}
	for _, l := range golds {
		if l.amount > 0 {
			amt(ansi.FgBrightCyan, int64(l.amount), l.text)
		}
	}
	if total > 0 {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBlue, strings.Repeat("─", width), ansi.Reset)
		amt(ansi.FgBrightYellow, int64(total), "gold earned this turn.")
	}
	if b.Food > 0 {
		amt(ansi.FgBrightCyan, int64(b.Food), "Food units were grown.")
	}
	if b.RiverFood > 0 {
		amt(ansi.FgBrightCyan, int64(b.RiverFood), "Food units were fished from the rivers.")
	}
	manufacturedUnits(s, made)
	// The bank's two returns close the report, interest first (#216) and then the
	// matured investments — which is where BRE puts that line, after the
	// manufacturing figures (cap/eots-ibbs-01.cap). BRE prints no interest line at
	// all; the wording is IB's own, in the style of the lines above it.
	if interest > 0 || invested > 0 {
		// A blank line ahead of them, as with the raid notices below: what the
		// realm PRODUCED and what the bank paid are two different things, and the
		// unit list right above ends in an indented column of its own. BRE runs
		// its investment line straight on from the production lines.
		fmt.Fprintln(s)
		if interest > 0 {
			amt(ansi.FgBrightCyan, interest, "gold was earned in bank interest.")
		}
		if invested > 0 {
			amt(ansi.FgBrightCyan, invested, "gold was earned from investment returns.")
		}
	}
	if len(raids) > 0 {
		// A blank line before the raid notices. BRE runs them straight on from
		// the production lines; separating them is IB's own readability choice
		// (docs/dev/bre-screens.md).
		fmt.Fprintln(s)
		for _, r := range raids {
			pirateHitLine(s, r)
		}
	}
	pause(s)
}

// pirateColor returns a faction's color by its slot — pirateColors, the palette
// the Attack Pirates menu paints. Keyed on the slot rather than the name so a
// world whose factions carry other names still colors them.
func pirateColor(slot int) string {
	if slot >= 0 && slot < len(pirateColors) {
		return pirateColors[slot].Color
	}
	return ansi.FgWhite
}

// raiderSlots is the distinct set of faction slots that hit in raids, in the
// order first seen — feeds Empire.RaidersThisTurn (nil when raids is empty,
// which is what leaves an old turn's mark cleared).
func raiderSlots(raids []game.PirateHit) []int {
	var slots []int
	for _, h := range raids {
		seen := false
		for _, s := range slots {
			if s == h.Slot {
				seen = true
				break
			}
		}
		if !seen {
			slots = append(slots, h.Slot)
		}
	}
	return slots
}

// pirateHitLine reports one raid the way BRE does: the faction in its own
// color, then the single thing it carried off, its figure highlighted. BRE
// writes "has captured" for every faction; IB uses "have", since the names are
// plural.
func pirateHitLine(s session.Session, h game.PirateHit) {
	unit := h.Spoil.Label()
	faction := pirateColor(h.Slot) + h.Faction + ansi.Reset
	amount := ansi.FgBrightCyan + comma(h.Amount) + ansi.Reset
	fmt.Fprintf(s, "  %s\n", fmt.Sprintf(tr(s, "%s have captured %s %s"), faction, amount, tr(s, unit)))
}

// peopleMood returns an end-of-turn flavor line keyed to popular support, in
// the spirit of BRE's tiered "how your people feel" message (original wording).
func peopleMood(support int) string {
	switch {
	case support < 10:
		return "The mob is at your gates — your people would be rid of you by any means."
	case support < 20:
		return "Your people seethe with open hatred for your rule."
	case support < 30:
		return "Riots flare through the streets almost daily."
	case support < 40:
		return "Unrest simmers; angry crowds gather against your decrees."
	case support < 50:
		return "Discontent runs deep — your people grumble at every order."
	case support < 60:
		return "Your people endure your rule, but take little joy in it."
	case support < 70:
		return "Your people go about their business, content enough."
	case support < 80:
		return "Your people are glad to live under your banner."
	case support < 90:
		return "Your people admire your leadership and prosper gladly."
	default:
		return "Your people revere you — faith in your rule has never been higher."
	}
}

// endOfTurnStats prints a short flavor line and the remaining turns. It does
// not pause — the caller (runTurn) immediately follows it with the "Continue
// to your next turn?" prompt, and a pause here plus that prompt were two
// consecutive single-key reads that could cross a fast typist's input.
// endOfTurnStats snapshots p under the lock first, since the daily-
// maintenance ticker (or another session) can mutate these same fields
// concurrently.
func endOfTurnStats(s session.Session, w *ctx) {
	var snap game.Empire
	if !withPlayer(w, func(p *game.Empire) { snap = *p }) {
		return
	}
	p := &snap
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "End of Turn Statistics:"), ansi.Reset)
	fmt.Fprintf(s, "%s\n", rule75(ansi.FgBlue))
	fmt.Fprintf(s, "  %s\n", tr(s, peopleMood(p.Support)))
	// A flat turn prints "gained 0" rather than nothing: BRE does (cap/kd3-01.cap,
	// twice, both on riot turns), and IB used to skip the line entirely, so a
	// realm whose growth was suppressed — most often by an empty granary — was
	// told nothing at all.
	if p.LastPopGrowth >= 0 {
		fmt.Fprintf(s, "  "+tr(s, "Your dominion gained %s%s%s people.")+"\n", ansi.FgBrightCyan, comma(p.LastPopGrowth), ansi.Reset)
	} else {
		fmt.Fprintf(s, "  "+tr(s, "Your dominion lost %s%s%s people.")+"\n", ansi.FgBrightRed, comma(-p.LastPopGrowth), ansi.Reset)
	}
	statLine(s, p.LastSpoiled, "units of food spoiled.")
	if p.LastRiot {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBrightRed, tr(s, "Riots have broken out due to high tax rates!"), ansi.Reset)
	}
	if p.LastCivilWar > 0 {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBrightRed, hiNums(fmt.Sprintf(tr(s, "Civil war! Famine cost you %d%% of your realm and its forces."), p.LastCivilWar)), ansi.Reset)
	}
	statLine(s, p.LastMoraleDesertion, "troops deserted due to low morale.")
	fmt.Fprintf(s, "%s\n", rule75(ansi.FgBlue))
}
