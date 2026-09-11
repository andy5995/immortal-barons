package game

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Planet-wide news, in the spirit of BRE's news.dat feed (see
// docs/mechanics-reference.md, "News files"). Wording here is original — the
// original's news lines are not copied verbatim — but the events broadcast
// match: regular-attack wins/losses and total conquests go to every player's
// news, not just the victim.

// NewsLine is one entry in the planetary news feed: what happened, and when.
//
// The time is stored in UTC and shown on the reader's own clock (#267). The DATE
// is not on the line because the screen it is drawn on is headed by the day the
// feed belongs to. The original stamps no news line at all; IB does, because a
// feed a player meets once a day says more with the hour on it.
type NewsLine struct {
	At   string `json:",omitempty"`
	Text string
}

// UnmarshalJSON accepts a bare string as well as an object, so a world saved
// before news lines carried a time still loads. Such a line keeps its text and
// shows no time, rather than one invented for it — the same shape
// Message.UnmarshalJSON has, for the same reason.
func (n *NewsLine) UnmarshalJSON(b []byte) error {
	var text string
	if err := json.Unmarshal(b, &text); err == nil {
		*n = NewsLine{Text: text}
		return nil
	}
	type plain NewsLine // avoid recursing into this method
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*n = NewsLine(p)
	return nil
}

// NewsFeed is one day's news.
type NewsFeed []NewsLine

// Join is the feed's text, one line per entry — for a caller that wants the
// prose and not the times (tests, and any search over what was said).
func (f NewsFeed) Join(sep string) string {
	out := make([]string, len(f))
	for i, n := range f {
		out[i] = n.Text
	}
	return strings.Join(out, sep)
}

// News builds a feed from bare text — lines with no time of their own, which is
// what a test writes and what a world saved before news was stamped carries.
func News(lines ...string) NewsFeed {
	f := make(NewsFeed, len(lines))
	for i, l := range lines {
		f[i] = NewsLine{Text: l}
	}
	return f
}

// postNews appends a system news line to the planetary bulletin, keeping only
// the most recent entries (same cap as player bulletins).
func (w *World) postNews(line string) {
	w.NewsToday = append(w.NewsToday, NewsLine{At: StoredStamp(timeNow()), Text: line})
	if len(w.NewsToday) > 20 {
		w.NewsToday = w.NewsToday[len(w.NewsToday)-20:]
	}
}

// noteSysop records a transport fault for whoever runs the game. See
// World.SysopNotices for why these do not go to the planet's news.
func (w *World) noteSysop(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	// One fault, one line. A run can meet the same fault on several packets —
	// a board whose traffic is held sends a batch, and every file in it bounces
	// the same notice back — and repeating it once per file buries whatever else
	// the run has to say. Seen on the test rig: three identical held-ruleset
	// notices in one run.
	for _, have := range w.SysopNotices {
		if have == line {
			return
		}
	}
	w.SysopNotices = append(w.SysopNotices, line)
}

// BeginRun clears the once-per-run bookkeeping the hold notices keep. On a door
// the world is loaded fresh for every planetary run, so this was implicit and
// invisible — until two runs shared one World in a test and the second reported
// nothing, which is also what a caller doing two runs in one process would get.
// Per-run state that depends on being reloaded is per-process state.
func (w *World) BeginRun() { w.heldNoted = nil }

// NoteRulesetHold records that a board's packets are being set aside because
// the rules it says it is playing by are not the league's (#264). Once per
// board per run, for the reason NoteProtocolHold gives.
//
// The notice says which side has to move, because the two cases recover
// differently. A board sending packets that state rules the league never agreed
// has THOSE files expire at HeldMaxAge; what unblocks its traffic is that board
// taking the Coordinator's ruleset, which goes out on every planetary run. The
// reverse case — this board being the one behind after a ruleset change, so
// boards that adopted first look divergent — clears itself, because held packets
// are re-checked on every run.
// Returns whether this is the first hold from that board this run, which is
// what decides whether a bounce goes back to it (see World.BounceRuleset).
func (w *World) NoteRulesetHold(board string) bool {
	// Keyed by REASON as well as board: heldNoted is shared with
	// NoteProtocolHold, and a bare board key would have whichever hold fired
	// first silence the other for the rest of the run.
	key := "ruleset:" + board
	if w.heldNoted == nil {
		w.heldNoted = map[string]bool{}
	}
	if w.heldNoted[key] {
		return false
	}
	w.heldNoted[key] = true
	w.noteSysop("Packets from %s are being held: the rules it is playing by are not the league's. Its traffic flows again once it takes the Coordinator's ruleset, which goes out on every planetary run; the packets already held expire on the ordinary held-packet timer.", board)
	return true
}

// NoteProtocolHold records that a board's packets are being set aside because
// this build cannot read their format, once per board per run rather than once
// per packet — a mismatch affects every packet that board sends, and repeating
// it per file buries the one line that matters.
func (w *World) NoteProtocolHold(board string, protocol int) {
	key := "protocol:" + board // see NoteRulesetHold: one keyspace, several reasons
	if w.heldNoted == nil {
		w.heldNoted = map[string]bool{}
	}
	if w.heldNoted[key] {
		return
	}
	w.heldNoted[key] = true
	// Which way the mismatch runs decides whether waiting fixes it, and the
	// sysop's next move differs completely: upgrading this board releases a
	// newer board's packets, while an older board's are held by a format this
	// build has already moved past and no upgrade of theirs brings back. Saying
	// "when both boards run the same release" for both was true only of the
	// first (#228 review).
	if protocol > Protocol {
		w.noteSysop("Packets from %s are being held: they speak protocol %d and this board speaks %d. Upgrading this board applies them.",
			board, protocol, Protocol)
		return
	}
	w.noteSysop("Packets from %s are being held: they speak protocol %d, which this board (protocol %d) has moved past. They will NOT be applied on their own, even once that board upgrades — ask them to resend anything that mattered.",
		board, protocol, Protocol)
}

// postCombatNews broadcasts the outcome of a regular attack to the planet.
func (w *World) postCombatNews(a, d *Empire, won, conquered bool) {
	// Every conventional battle funnels through here, so this is the honest place
	// to tally them for the -spectate balance probe. Counting them by matching the
	// news prose below would break the moment the wording changes.
	w.BattlesTotal++
	if conquered {
		w.ConquestsTotal++
	}
	// Recorded here for the same reason it is counted here: every conventional
	// battle passes through, and the alternative is reading it back out of the
	// prose below.
	w.logBattle(BattleLogEntry{Attacker: a.Name, Defender: d.Name, Won: won, Crushed: conquered})
	var lines []string
	switch {
	case conquered:
		lines = []string{
			fmt.Sprintf("NEWS! The empire of %s has been wiped from the map by %s!", d.Name, a.Name),
			fmt.Sprintf("%s conquered %s outright — the realm is no more.", a.Name, d.Name),
			fmt.Sprintf("Historians will remember %s's destruction of %s.", a.Name, d.Name),
		}
	case won:
		lines = []string{
			fmt.Sprintf("The wars grind on: %s overran %s in battle.", a.Name, d.Name),
			fmt.Sprintf("%s broke through the defenses of %s today.", a.Name, d.Name),
			fmt.Sprintf("%s claimed a hard-won victory over %s.", a.Name, d.Name),
			fmt.Sprintf("%s got thrashed by %s.", d.Name, a.Name),
		}
	default:
		lines = []string{
			fmt.Sprintf("%s threw itself at %s and was thrown back.", a.Name, d.Name),
			fmt.Sprintf("%s repelled an assault from %s.", d.Name, a.Name),
			fmt.Sprintf("%s retreated in disarray from the walls of %s.", a.Name, d.Name),
		}
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postStrikeNews broadcasts a WMD strike (weapon = "nuclear"/"chemical"/
// "biological"), matching BRE's NUKE/CHEM/BIO news categories.
func (w *World) postStrikeNews(a, d *Empire, weapon string) {
	// Logged for the world report alongside conventional battles (#233). The
	// report used to carry attacks only, which left it blank in a league whose
	// fighting is mostly missiles.
	w.logBattle(BattleLogEntry{Attacker: a.Name, Defender: d.Name, Won: true, Weapon: weapon})
	lines := []string{
		fmt.Sprintf("ALERT: %s struck %s with %s weapons!", a.Name, d.Name, weapon),
		fmt.Sprintf("%s unleashed %s fire upon %s.", a.Name, weapon, d.Name),
		fmt.Sprintf("The planet recoils as %s hits %s with %s missiles.", a.Name, d.Name, weapon),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postPirateNews broadcasts a pirate-raid outcome (BRE PIRATEWIN/PIRATELOSS).
func (w *World) postPirateNews(a *Empire, faction string, won bool) {
	// The sysop's PirateNews switch, which the original asks per game mode. Only
	// the news line is suppressed: the raid, its loot and its losses all stand,
	// and the raider still gets the full report on screen.
	if !w.Config.PirateNews {
		return
	}
	var lines []string
	if won {
		lines = []string{
			fmt.Sprintf("%s drove off the %s in a daring raid.", a.Name, faction),
			fmt.Sprintf("The %s were bloodied by %s today.", faction, a.Name),
		}
	} else {
		lines = []string{
			fmt.Sprintf("%s's raid on the %s ended in humiliation.", a.Name, faction),
			fmt.Sprintf("The %s repelled %s with ease.", faction, a.Name),
		}
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postRiotNews broadcasts tax riots in an empire (BRE RIOTS).
func (w *World) postRiotNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Riots erupt in %s over crushing taxes!", e.Name),
		fmt.Sprintf("The people of %s take to the streets against high taxes.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postCivilWarNews broadcasts a realm collapsing into civil war after a famine.
func (w *World) postCivilWarNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Civil war breaks out in %s as the hungry turn on the crown.", e.Name),
		fmt.Sprintf("The realm of %s tears itself apart over empty food stores.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postStarvationNews broadcasts an empire's food shortfall.
func (w *World) postStarvationNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Famine grips %s as food stocks run out.", e.Name),
		fmt.Sprintf("Reports of starvation reach the planet from %s.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postInvestRateNews broadcasts a change in the planetary investment rate
// (BRE's daily bank-rate float). No line posts when the rate did not move.
func (w *World) postInvestRateNews(before int) {
	switch {
	case w.InvestRate > before:
		w.postNews(fmt.Sprintf("The planetary investment rate rose to %s%%.", PctTenths(w.InvestRate)))
	case w.InvestRate < before:
		w.postNews(fmt.Sprintf("The planetary investment rate fell to %s%%.", PctTenths(w.InvestRate)))
	}
}

// postMasterNews broadcasts the planet's political standing: the empire
// with the highest net worth among the living either keeps or claims the
// title of Planetary Master, and CurrentMaster is kept in sync with it. This
// runs every maintenance day (matching BRE, which shows the title daily),
// separate from endGame's one-time crowning of LastMaster at a league's end.
func (w *World) postMasterNews() {
	var master *Empire
	bestNW := 0
	for _, e := range w.Empires {
		if e.Alive {
			if nw := w.NetWorth(e); master == nil || nw > bestNW {
				bestNW = nw
				master = e
			}
		}
	}
	if master == nil {
		return
	}
	best := master.Name
	switch {
	case best == w.CurrentMaster:
		w.postNews(fmt.Sprintf("%s retains the title of Planetary Master.", best))
	case w.CurrentMaster == "":
		w.postNews(fmt.Sprintf("%s claims the title of Planetary Master!", best))
	default:
		w.postNews(fmt.Sprintf("%s has seized the title of Planetary Master from %s!", best, w.CurrentMaster))
	}
	w.CurrentMaster = best
	w.payMaster(master)
}

// payMaster hands the day's Planetary Master a share of the Queen's purse, the
// same purse the tax refund is drawn from. BINARY-VERIFIED (BRE.OVR 0x007aeb,
// update_planet_title): straight after filing the title news it credits the
// holder's gold with pool/100 and takes the same amount back out of the pool
// (config record +0x24), then files a personal notice on the holder's recap.
func (w *World) payMaster(e *Empire) {
	if w.RefundPool <= 0 {
		return
	}
	pay := pctOf(w.RefundPool, MasterAwardPct)
	if pay <= 0 {
		return
	}
	w.RefundPool -= pay
	w.creditGold(e, pay, "the Planetary Master's share")
	// A computer baron takes the gold but gets no notice: the daily sweep in
	// DailyMaintenance clears its recap anyway, and this runs after that sweep.
	if e.Owner != "" {
		e.addEvent(fmt.Sprintf("The Queen Royale sends you %s gold for holding the title of Planetary Master.",
			numfmt.Comma(pay)))
	}
}

// BattleLog is the world report's raw material: one line per attack fought
// anywhere in the league (#233, asked for by a sysop who wanted to see the
// wars rather than each board's own scoreboard). Deliberately ATTACKS only --
// no nuclear, chemical or biological strike, and no terror op -- because the
// report is about armies meeting, and a weapon landing on a city is a
// different story the news already tells.
//
// Structured rather than parsed back out of the news: the news wording is
// picked at random from three phrasings and then translated, so anything
// reading it would be reading a sentence that changes under it.
type BattleLogEntry struct {
	Date     string
	Planet   string // the board it was fought on; empty means this one
	Attacker string
	Defender string
	Won      bool
	Land     int  // regions taken, 0 on a defeat
	Crushed  bool // the defender was wiped out
	// Remote marks a strike that crossed planets, which reads differently from
	// two neighbours fighting and is worth telling apart in the report.
	Remote bool
	// Weapon names the warhead when the entry is a WMD strike rather than an
	// attack -- "nuclear", "chemical", "biological". Empty for a conventional
	// battle, which is every entry written before strikes were logged, so an
	// older board's packets read as attacks and are right to.
	//
	// NO PROTOCOL BUMP for this field, deliberately. Battles are stripped before
	// signing (see StampOutbox), so the signed bytes do not change; the tag is
	// omitempty, so an older board never sees it; and SpeaksOurProtocol is an
	// exact match, so bumping would HOLD every packet from a board still on the
	// old build for a purely additive field. See the golden shape test.
	Weapon string `json:",omitempty"`
}

// MaxBattleLog bounds the log. A league that fights hard produces a few dozen
// entries a day and every one of them rides in a packet, so this is a real
// bound rather than a formality: without it a busy season grows the world file
// and the wire without limit.
const MaxBattleLog = 200

// logBattle records one attack, newest last, discarding the oldest once the log
// is full.
func (w *World) logBattle(e BattleLogEntry) {
	if e.Date == "" {
		e.Date = w.LastMaintDate
	}
	w.Battles = append(w.Battles, e)
	if len(w.Battles) > MaxBattleLog {
		w.Battles = w.Battles[len(w.Battles)-MaxBattleLog:]
	}
}

// mergeBattles takes in another board's log, ignoring any entry that names no
// planet -- a board stamps its own name on the way out, so a blank one is
// either malformed or would masquerade as a battle fought here.
//
// A resend repeats entries the board already holds, so they are matched on
// their content rather than trusted to arrive once: a packet can be replayed,
// and a world report that counted the same battle twice would be worse than
// one a day out of date.
func (w *World) mergeBattles(in []BattleLogEntry) {
	if len(in) == 0 {
		return
	}
	seen := make(map[BattleLogEntry]bool, len(w.Battles))
	for _, b := range w.Battles {
		seen[b] = true
	}
	for _, b := range in {
		if b.Planet == "" || seen[b] {
			continue
		}
		seen[b] = true
		w.logBattle(b)
	}
}

// CountFaults adds this run's new transport faults to the tally Game Setup
// shows, starting the count's clock the first time there is anything to count
// (#187). A scheduled planetary run's output goes to a mailbox nobody reads, so
// this is the one place an operator meets the fact that something is wrong
// without having been told to go looking.
func (w *World) CountFaults(n int) {
	if n <= 0 {
		return
	}
	if w.FaultsSeen == 0 {
		// The game clock, not the wall clock: it is the date every other thing
		// on that screen is measured in, and a board pinned to a -date would
		// otherwise report a span it did not play.
		w.FaultsSince = w.LastMaintDate
	}
	w.FaultsSeen += n
}
