package menu

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// interbbsScores displays scores imported from other boards via inter-BBS
// packets (see internal/ibbs). v1 covers only score/news sharing.
func interbbsScores(s session.Session, w *ctx) Result {
	// BRE's View IPScores is one cross-planet board (header
	// "Id Empire Name Territory Score Net Worth", lettered ids, "No Scores"
	// when empty), not a per-board split. Reuse the local scores layout.
	type row struct {
		name            string
		land, score, nw int
	}
	var rows []row
	w.With(func() {
		for _, b := range w.RemoteBoards {
			for _, sc := range b.Scores {
				score := sc.Score
				if score == 0 {
					score = sc.NetWorth // pre-Score packets carried only net worth
				}
				rows = append(rows, row{sc.Empire, sc.Land, score, sc.NetWorth})
			}
		}
	})
	if len(rows) == 0 {
		ok(s, "No inter-BBS scores have been imported yet.")
		return Stay
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	rule := strings.Repeat("─", 72)
	fmt.Fprintf(s, "\n%s-*%s%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "InterBBS Scores"), ansi.Reset, ansi.FgBrightMagenta, ansi.Reset)
	fmt.Fprintf(s, "%s%-4s %-26s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, tr(s, "Id"), tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
	for i, r := range rows {
		fmt.Fprintf(s, "%s%-4s%s %s%-26s%s %s%10d%s %s%11d%s %s%11d%s\n",
			ansi.FgBrightMagenta, scoreID(i), ansi.Reset,
			ansi.FgBrightWhite, r.name, ansi.Reset,
			ansi.FgBrightMagenta, r.land, ansi.Reset,
			ansi.FgBrightWhite, r.score, ansi.Reset,
			ansi.FgWhite, r.nw, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
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
	gold := promptSuggested(s, "Add how much gold for funding?", p.Gold, p.Gold)
	if gold <= 0 {
		return Stay
	}
	days := promptInt(s, "Leave in how many days?")
	if days < 1 {
		days = 1
	}
	var id, departDay int
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		var ga *game.GroupAttack
		ga, err = w.World.CreateGroupAttack(p, board, target, w.GameDay+days, gold)
		if err != nil {
			return
		}
		id, departDay = ga.ID, ga.DepartDay
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Group attack #%d formed against %s on %s, leaving day %d.", id, pick, board, departDay)
	return Stay
}

// joinGroupAttack adds the player's offense to a group attack still forming.
func joinGroupAttack(s session.Session, w *ctx) Result {
	type gaRow struct {
		id   int
		line string
	}
	var rows []gaRow
	var suggested int
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		suggested = p.Gold
		for _, ga := range w.GroupAttacks {
			if w.GameDay >= ga.DepartDay {
				continue
			}
			tgt := ga.TargetEmpire
			if tgt == "" {
				tgt = tr(s, "the whole planet")
			}
			rows = append(rows, gaRow{ga.ID, fmt.Sprintf("#%d -> %s on %s (leaves day %d, funding %s gold)",
				ga.ID, tgt, ga.TargetBoard, ga.DepartDay, comma(ga.Gold()))})
		}
	})
	if len(rows) == 0 {
		ok(s, "No group attacks are forming right now.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Join which attack?"), ansi.Reset)
	for i, r := range rows {
		fmt.Fprintf(s, "    %d) %s\n", i+1, r.line)
	}
	i := promptInt(s, "Attack (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	gold := promptSuggested(s, "Add how much gold for funding?", suggested, suggested)
	if gold <= 0 {
		return Stay
	}
	id := rows[i-1].id
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		// JoinGroupAttack re-validates against fresh state: the attack must still
		// exist (ErrNoAttack), not yet have departed (ErrDeparted), and the funder
		// must still hold the gold (ErrCantAfford).
		err = w.World.JoinGroupAttack(p, id, gold)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "You joined group attack #%d with %s gold.", id, comma(gold))
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

// travelTimes shows the round-trip time to each known planet, matching BRE's
// "Average Turn Around Times to All BBSes" screen: each BBS with its turnaround
// coloured green (sub-day), cyan (days), or red ("No Data"). BRE averages many
// exchanges; IB has only the last packet's date per board, so the figure is the
// latency of the most recent exchange rather than a running average.
func travelTimes(s session.Session, w *ctx) Result {
	type planet struct{ name, date string }
	var planets []planet
	var now string
	w.With(func() {
		now = w.Today
		if now == "" {
			now = w.LastMaintDate
		}
		at := map[string]int{}
		add := func(name, date string) {
			if i, ok := at[name]; ok {
				if date != "" {
					planets[i].date = date
				}
				return
			}
			at[name] = len(planets)
			planets = append(planets, planet{name, date})
		}
		for _, n := range w.LeagueNodes {
			add(n.Name, "")
		}
		for _, b := range w.RemoteBoards {
			add(b.BoardID, b.Date)
		}
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Average Turn Around Times to All BBSes"), ansi.Reset)
	for _, p := range planets {
		label, col := turnaroundLabel(s, p.date, now)
		fmt.Fprintf(s, "  %-24s %s%s%s\n", p.name, col, label, ansi.Reset)
	}
	pause(s)
	return Stay
}

// turnaroundLabel renders a planet's round-trip time and its colour from the
// last packet date, matching BRE's Travel Times colouring: red "No Data" when
// no packet has arrived, green under a day, cyan for whole days.
func turnaroundLabel(s session.Session, then, now string) (string, string) {
	if then == "" {
		return tr(s, "No Data"), ansi.FgBrightRed
	}
	t1, e1 := time.Parse("2006-01-02", then)
	t2, e2 := time.Parse("2006-01-02", now)
	if e1 != nil || e2 != nil {
		return tr(s, "No Data"), ansi.FgBrightRed
	}
	if days := t2.Sub(t1).Hours() / 24; days >= 1 {
		return fmt.Sprintf(tr(s, "%.1f days"), days), ansi.FgBrightCyan
	}
	return tr(s, "< 1 day"), ansi.FgBrightGreen
}

// planetaryTreaties is the InterPlanetary Ops "Diplomacy List": BRE's
// "Planetary Treaties" screen lists every other BBS with the planet-to-planet
// treaty in force. IB has no inter-BBS treaty mechanic yet, so each shows
// "None" — which is what the observed BRE board displayed as well.
func planetaryTreaties(s session.Session, w *ctx) Result {
	type row struct {
		num  int
		name string
	}
	var rows []row
	w.With(func() {
		seen := map[string]bool{}
		maxNum := 1
		for _, n := range w.LeagueNodes {
			if seen[n.Name] {
				continue
			}
			seen[n.Name] = true
			rows = append(rows, row{n.Number, n.Name})
			if n.Number > maxNum {
				maxNum = n.Number
			}
		}
		for _, b := range w.RemoteBoards {
			if seen[b.BoardID] {
				continue
			}
			seen[b.BoardID] = true
			maxNum++
			rows = append(rows, row{maxNum, b.BoardID})
		}
	})
	if len(rows) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "Planetary Treaties"), ansi.Reset)
	for _, r := range rows {
		fmt.Fprintf(s, "  %s(%2d)%s %-24s %s\n", ansi.FgBrightRed, r.num, ansi.Reset, r.name, tr(s, "None"))
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
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		if p.Agents < 1 { // re-check the agent against fresh state
			err = game.ErrNoAgents
			return
		}
		p.Agents--
		w.SpyDatabase = append(w.SpyDatabase, game.SpyReport{
			Board:  board,
			Empire: pick,
			Date:   w.LastMaintDate,
			Land:   sc.Land,
		})
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
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
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		w.World.VoteCoordinator(p, owners[i-1])
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
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
	var err error
	w.With(func() {
		// Re-check the coordinator role against fresh state: a vote elsewhere may
		// have unseated the player between the check and here.
		if w.BBSCoordinator() != w.Player() {
			err = errRealmChanged
			return
		}
		w.LeagueDiplomacy = decl
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "League diplomacy updated. It will be broadcast to the league.")
	return Stay
}
