package menu

import (
	"fmt"
	"math"
	"sort"
	"strings"

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
// another planet (chosen from imported scores). Barons commit troopers (BRE's
// model — real forces, not gold); the pooled troopers become the strike's
// offense on departure.
func createGroupAttack(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
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
	board := pickPlanetNamed(s, w, boards)
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
	force := promptAttackForce(s, p)
	if force.Empty() {
		return Stay
	}
	days := promptInt(s, "Leave in how many days?")
	if days < 1 {
		days = 1
	}
	var id, departDay int
	err := w.mutatePlayer(func(p *game.Empire) error {
		ga, e := w.World.CreateGroupAttack(p, board, target, w.GameDay+days, force)
		if e != nil {
			return e
		}
		id, departDay = ga.ID, ga.DepartDay
		return nil
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
	if blockedByProtection(s, w) {
		return Stay
	}
	type gaRow struct {
		id   int
		line string
	}
	var rows []gaRow
	w.With(func() {
		if w.Player() == nil {
			return
		}
		for _, ga := range w.GroupAttacks {
			if w.GameDay >= ga.DepartDay {
				continue
			}
			tgt := ga.TargetEmpire
			if tgt == "" {
				tgt = tr(s, "the whole planet")
			}
			rows = append(rows, gaRow{ga.ID, fmt.Sprintf("#%d -> %s on %s (leaves day %d, %s offense)",
				ga.ID, tgt, ga.TargetBoard, ga.DepartDay, comma(ga.Offense()))})
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
	force := promptAttackForce(s, w.Player())
	if force.Empty() {
		return Stay
	}
	id := rows[i-1].id
	// JoinGroupAttack re-validates against fresh state: the attack must still exist
	// (ErrNoAttack), not yet have departed (ErrDeparted), and the baron must still
	// hold the committed units (ErrCantAfford).
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.JoinGroupAttack(p, id, force)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "You joined group attack #%d.", id)
	return Stay
}

// promptAttackForce asks how many of each unit type to commit to a group attack
// (BRE's "Send how many Troopers?/Jets?/Tanks?/Bombers?"), skipping types the
// baron has none of.
func promptAttackForce(s session.Session, p *game.Empire) game.AttackForce {
	var f game.AttackForce
	fmt.Fprint(s, "\n")
	if p.Troopers > 0 {
		f.Troopers = promptSuggestedTight(s, "Send how many Troopers?", p.Troopers, p.Troopers)
	}
	if p.Jets > 0 {
		f.Jets = promptSuggestedTight(s, "Send how many Jets?", 0, p.Jets)
	}
	if p.Tanks > 0 {
		f.Tanks = promptSuggestedTight(s, "Send how many Tanks?", 0, p.Tanks)
	}
	if p.Bombers > 0 {
		f.Bombers = promptSuggestedTight(s, "Send how many Bombers?", 0, p.Bombers)
	}
	return f
}

// indivAttackForce is BRE's "Indiv. Attack Force": one baron striking one named
// baron on another planet. It leaves at once rather than assembling like a group
// attack, and it spends one of the day's individual attacks (#62).
func indivAttackForce(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	board, target := pickRemoteBaron(s, w)
	if board == "" || target == "" {
		return Stay
	}
	force := promptAttackForce(s, w.Player())
	if force.Empty() {
		return Stay
	}
	var id int
	err := w.mutatePlayer(func(p *game.Empire) error {
		n, e := w.World.CreateIndividualAttack(p, board, target, force)
		id = n
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Attack force #%d is on its way to %s on %s.", id, target, board)
	return Stay
}

// pickRemoteBaron asks for a planet and then a named baron on it. Unlike the
// group-attack picker it offers no whole-planet choice, because an individual
// attack has to name its target. Empty strings mean the player backed out.
func pickRemoteBaron(s session.Session, w *ctx) (board, baron string) {
	var boards []string
	var scores map[string][]string
	w.With(func() {
		scores = map[string][]string{}
		for _, b := range w.RemoteBoards {
			boards = append(boards, b.BoardID)
			for _, sc := range b.Scores {
				scores[b.BoardID] = append(scores[b.BoardID], sc.Empire)
			}
		}
	})
	if len(boards) == 0 {
		ok(s, "No other planets are known yet. Wait for inter-BBS scores to arrive.")
		return "", ""
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which planet?"), ansi.Reset)
	board = pickPlanetNamed(s, w, boards)
	if board == "" {
		return "", ""
	}
	if len(scores[board]) == 0 {
		ok(s, "No barons are known on that planet yet.")
		return "", ""
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which baron?"), ansi.Reset)
	baron = pickFromList(s, "Baron", scores[board])
	return board, baron
}

// Travel Times geometry, measured off a live BRE capture: a 75-column inset
// rule above and below the list, and each planet's name in a 30-column field
// with the turnaround right after it.
const (
	travelRuleWidth  = 75
	travelRuleDouble = 15
	travelNameWidth  = 30
)

// travelTimes is BRE's "Average Turn Around Times to All BBSes": every other
// planet in the league with the average round trip a packet makes to it and
// back — the figure that says whether a strike aimed there lands tonight or
// next week. A board that has not yet answered a probe reads "No Data".
//
// The averaging is game.recordTravelTime's; this screen only renders it, in
// BRE's units and colors — hours in green under two days, days in cyan at or
// above it, "No Data" in red.
func travelTimes(s session.Session, w *ctx) Result {
	type planet struct {
		name string
		days float64
	}
	var planets []planet
	w.With(func() {
		for _, name := range w.KnownBoards() {
			planets = append(planets, planet{name, w.TravelTimes[name]})
		}
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	rule := ansi.FgBrightBlack + insetRule(travelRuleWidth, travelRuleDouble) + ansi.Reset
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "Average Turn Around Times to All BBSes"), ansi.Reset)
	fmt.Fprintf(s, "%s\n", rule)
	for _, p := range planets {
		label, col := turnaroundLabel(s, p.days)
		fmt.Fprintf(s, "%s%-*s%s%s%s\n", ansi.FgWhite, travelNameWidth, p.name, col, label, ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	pause(s)
	return Stay
}

// turnaroundLabel renders one average round trip and its color. BRE quantizes
// before printing — hours to a tenth, days to a hundredth — so the figures land
// on the same values the original shows.
func turnaroundLabel(s session.Session, days float64) (string, string) {
	if days <= 0 {
		return tr(s, "No Data"), ansi.FgRed
	}
	if days < game.TravelHoursCutoff {
		hours := math.Round(days*24*10) / 10
		return fmt.Sprintf(tr(s, "%.2f hours"), hours), ansi.FgBrightGreen
	}
	return fmt.Sprintf(tr(s, "%.2f days"), math.Round(days*100)/100), ansi.FgCyan
}

// planetaryTreaties is the InterPlanetary Ops "Diplomacy List": BRE's
// "Planetary Treaties" screen lists every other BBS with the planet-to-planet
// treaty in force. IB has no inter-BBS treaty mechanic yet, so each shows
// "None" — which is what the observed BRE board displayed as well.
func planetaryTreaties(s session.Session, w *ctx) Result {
	rows := knownPlanets(w)
	if len(rows) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "Planetary Treaties"), ansi.Reset)
	for _, r := range rows {
		fmt.Fprintf(s, "  %s(%2d)%s %-24s %s\n", ansi.FgBrightRed, r.Number, ansi.Reset, r.Name, planetRelation(s))
	}
	pause(s)
	return Stay
}

// pickRemoteTarget prompts for a planet then a baron on it, returning the
// board, the baron's name, and its imported score. found is false if the caller
// cancels or no planets/barons are known.
func pickRemoteTarget(s session.Session, w *ctx, planetPrompt, baronPrompt string) (board, baron string, sc game.RemoteScore, found bool) {
	if len(w.RemoteBoards) == 0 {
		ok(s, "No other planets are known yet.")
		return
	}
	boards := make([]string, len(w.RemoteBoards))
	for i, b := range w.RemoteBoards {
		boards[i] = b.BoardID
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, planetPrompt), ansi.Reset)
	board = pickPlanetNamed(s, w, boards)
	if board == "" {
		return "", "", sc, false
	}
	var rb *game.RemoteBoard
	for i := range w.RemoteBoards {
		if w.RemoteBoards[i].BoardID == board {
			rb = &w.RemoteBoards[i]
		}
	}
	if rb == nil || len(rb.Scores) == 0 {
		ok(s, "No barons are known on that planet yet.")
		return "", "", sc, false
	}
	names := make([]string, len(rb.Scores))
	for i, x := range rb.Scores {
		names[i] = x.Empire
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, baronPrompt), ansi.Reset)
	baron = pickFromList(s, "Baron", names)
	if baron == "" {
		return "", "", sc, false
	}
	for _, x := range rb.Scores {
		if x.Empire == baron {
			sc = x
		}
	}
	return board, baron, sc, true
}

// sendSpyGuy is the Special Operations "Send SpyGuy" item: infiltrate a baron on
// another planet. BRE puts sending on Special Operations and keeps the Spy
// Database as the read-only viewer.
func sendSpyGuy(s session.Session, w *ctx) Result {
	if w.Player().Agents < 1 {
		fail(s, game.ErrNoAgents)
		return Stay
	}
	sendRemoteSpy(s, w)
	return Stay
}

// ipSpecialStub is a recorded-but-inert interplanetary Special Operations item:
// the cross-planet bombing/WMD variants aren't built yet, so the menu matches
// BRE while the mechanic stays a stub.
func ipSpecialStub(s session.Session, w *ctx) Result {
	ok(s, "That interplanetary operation is not yet available.")
	return Stay
}

// spyDatabase is the read-only Spy Database viewer (sending is Special
// Operations → Send SpyGuy, matching BRE).
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

// sendRemoteSpy scouts a baron on another planet. The request travels in the
// outbound packet and the answer comes back with real figures read on the target
// board, which is the point of the exchange — a shared score table already gives
// everyone land and net worth (#61). It costs an agent, and the report lands in
// the planet-wide Spy Database where every baron here can read it.
func sendRemoteSpy(s session.Session, w *ctx) {
	board, baron, _, found := pickRemoteTarget(s, w, "Spy on which planet?", "Spy on which baron?")
	if !found {
		return
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendRecon(p, board, baron)
	})
	if err != nil {
		fail(s, err)
		return
	}
	ok(s, "Your agents are on their way to %s on %s. Their report will reach the Spy Database when word comes back.", baron, board)
}

// terroristOps sends agents to destroy an enemy baron's forces on another
// planet — BRE's Terrorist Ops. The strike is queued and resolves on the target
// board's next packet run; New Realm Protection blocks it.
func terroristOps(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	if w.Player().Agents < 1 {
		fail(s, game.ErrNoAgents)
		return Stay
	}
	board, baron, _, found := pickRemoteTarget(s, w, "Terrorize which planet?", "Terrorize which baron?")
	if !found {
		return Stay
	}
	agents := promptSuggested(s, "How many agents to send?", w.Player().Agents, w.Player().Agents)
	if agents <= 0 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendTerror(p, board, baron, agents)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your terrorists depart for %s on %s.", baron, board)
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
			// Realms still under new-realm protection are not eligible candidates
			// (BRE). Voting for yourself is allowed once your own protection ends.
			if !e.Alive || e.Owner == "" || e.Protection > 0 {
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
	err := w.mutatePlayer(func(p *game.Empire) error {
		w.World.VoteCoordinator(p, owners[i-1])
		return nil
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
	err := w.mutatePlayer(func(p *game.Empire) error {
		// Re-check the coordinator role against fresh state: a vote elsewhere may
		// have unseated the player between the check and here.
		if w.BBSCoordinator() != p {
			return errRealmChanged
		}
		w.LeagueDiplomacy = decl
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "League diplomacy updated. It will be broadcast to the league.")
	return Stay
}
