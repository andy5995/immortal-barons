package menu

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

type ipRankKind int

const (
	ipRankPlanetScore ipRankKind = iota + 1
	ipRankPlanetNW
	ipRankPlanetLand
	ipRankPlanetDensity
	ipRankPlayerScore
	ipRankPlayerNW
	ipRankPlayerLand
	ipRankPlayerDensity
)

type ipScoreRow struct {
	name            string
	planet          string // empty for planet views
	score, nw, land int
}

// interbbsScores opens BRE's IP Scores submenu: eight ranking views (four
// planet-level, four player-level) each showing a ranked table. Captured from
// BRE (docs/dev/bre-screens.md, "InterBBS Scores").
func interbbsScores(s session.Session, w *ctx) Result {
	var rows []ipScoreRow
	w.With(func() {
		for _, b := range w.RemoteBoards {
			for _, sc := range b.Scores {
				score := sc.Score
				if score == 0 {
					score = sc.NetWorth
				}
				rows = append(rows, ipScoreRow{
					name:   sc.Empire,
					planet: b.BoardID,
					score:  score,
					nw:     sc.NetWorth,
					land:   sc.Land,
				})
			}
		}
		for _, e := range w.Empires {
			if e.Alive && e.Owner != "" {
				rows = append(rows, ipScoreRow{
					name:   e.Name,
					planet: w.Config.BoardID,
					score:  e.Score,
					nw:     w.NetWorth(e),
					land:   e.Land,
				})
			}
		}
	})
	if len(rows) == 0 {
		ok(s, "No inter-BBS scores have been imported yet.")
		return Stay
	}
	boxW := 42
	boxPad := (boxW - len("InterBBS Scores")) / 2
	for {
		fmt.Fprintf(s, "\n%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat("─", boxW), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat(" ", boxPad),
			tr(s, "InterBBS Scores"), ansi.Reset,
			strings.Repeat(" ", boxW-boxPad-len("InterBBS Scores")))
		for i, item := range []struct {
			key   byte
			label string
		}{
			{'1', "Top Planets by Score"},
			{'2', "Top Planets by Net Worth"},
			{'3', "Top Planets by Land"},
			{'4', "Top Planets by Net Worth Density"},
			{'5', "Top Players by Score"},
			{'6', "Top Players by Net Worth"},
			{'7', "Top Players by Land"},
			{'8', "Top Players by Net Worth Density"},
		} {
			fmt.Fprintf(s, "%s(%s%d%s) %s%s%s\n",
				ansi.FgBrightBlack, ansi.FgBrightWhite, i+1,
				ansi.FgBrightBlack, ansi.FgWhite, tr(s, item.label), ansi.Reset)
		}
		fmt.Fprintf(s, "%s(%s%c%s) %s%s%s\n",
			ansi.FgBrightBlack, ansi.FgBrightWhite, '0',
			ansi.FgBrightBlack, ansi.FgWhite, tr(s, "Quit"), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat("─", boxW), ansi.Reset)

		fmt.Fprintf(s, "\n%s>%s ", ansi.FgBrightWhite, ansi.Reset)
		r, err := readKey(s)
		if err != nil {
			return Stay
		}
		drainInput(s)
		if r == '\r' || r == '\n' || r == '0' {
			fmt.Fprintln(s, tr(s, "Quit"))
			return Stay
		}
		n := int(r - '0')
		if r < '1' || r > '8' {
			fmt.Fprintln(s)
			continue
		}
		fmt.Fprintf(s, "%d\n", n)
		ipScoreRank(s, w, rows, ipRankKind(n))
	}
}

// ipScoreViewName returns the BRE label for each ranking view.
func ipScoreViewName(s session.Session, kind ipRankKind) string {
	switch kind {
	case ipRankPlanetScore:
		return tr(s, "Top Planets by Score")
	case ipRankPlanetNW:
		return tr(s, "Top Planets by Net Worth")
	case ipRankPlanetLand:
		return tr(s, "Top Planets by Land")
	case ipRankPlanetDensity:
		return tr(s, "Top Planets by Net Worth Density")
	case ipRankPlayerScore:
		return tr(s, "Top Players by Score")
	case ipRankPlayerNW:
		return tr(s, "Top Players by Net Worth")
	case ipRankPlayerLand:
		return tr(s, "Top Players by Land")
	case ipRankPlayerDensity:
		return tr(s, "Top Players by Net Worth Density")
	}
	return ""
}

type planetAgg struct {
	name            string
	score, nw, land int
}

// ipScoreRank sorts and renders one BRE ranking view. Planet views aggregate
// scores per board; player views show individual empires with a Planet column.
// Captured from BRE (docs/dev/bre-screens.md, "InterBBS Scores").
func ipScoreRank(s session.Session, w *ctx, rows []ipScoreRow, kind ipRankKind) {
	metric := tr(s, "Score")
	switch kind {
	case ipRankPlanetScore, ipRankPlayerScore:
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].score != rows[j].score {
				return rows[i].score > rows[j].score
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetNW, ipRankPlayerNW:
		metric = tr(s, "Net Worth")
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].nw != rows[j].nw {
				return rows[i].nw > rows[j].nw
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetLand, ipRankPlayerLand:
		metric = tr(s, "Land")
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].land != rows[j].land {
				return rows[i].land > rows[j].land
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetDensity, ipRankPlayerDensity:
		metric = tr(s, "Net Worth / Region")
		sort.Slice(rows, func(i, j int) bool {
			di, dj := 0, 0
			if rows[i].land > 0 {
				di = rows[i].nw / rows[i].land
			}
			if rows[j].land > 0 {
				dj = rows[j].nw / rows[j].land
			}
			if di != dj {
				return di > dj
			}
			return rows[i].name < rows[j].name
		})
	}
	isPlanet := kind <= ipRankPlanetDensity
	var viewRows []struct {
		name, planet string
		val          int
	}
	if isPlanet {
		agg := map[string]*planetAgg{}
		for _, r := range rows {
			a, ok := agg[r.planet]
			if !ok {
				a = &planetAgg{name: r.planet}
				agg[r.planet] = a
			}
			a.score += r.score
			a.nw += r.nw
			a.land += r.land
		}
		var list []planetAgg
		for _, a := range agg {
			list = append(list, *a)
		}
		// Re-sort the aggregated list using the same comparator.
		switch kind {
		case ipRankPlanetScore:
			sort.Slice(list, func(i, j int) bool {
				if list[i].score != list[j].score {
					return list[i].score > list[j].score
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetNW:
			sort.Slice(list, func(i, j int) bool {
				if list[i].nw != list[j].nw {
					return list[i].nw > list[j].nw
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetLand:
			sort.Slice(list, func(i, j int) bool {
				if list[i].land != list[j].land {
					return list[i].land > list[j].land
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetDensity:
			sort.Slice(list, func(i, j int) bool {
				di, dj := 0, 0
				if list[i].land > 0 {
					di = list[i].nw / list[i].land
				}
				if list[j].land > 0 {
					dj = list[j].nw / list[j].land
				}
				if di != dj {
					return di > dj
				}
				return list[i].name < list[j].name
			})
		}
		for _, a := range list {
			val := metricValue(kind, a.score, a.nw, a.land)
			viewRows = append(viewRows, struct {
				name, planet string
				val          int
			}{a.name, "", val})
		}
	} else {
		for _, r := range rows {
			val := metricValue(kind, r.score, r.nw, r.land)
			viewRows = append(viewRows, struct {
				name, planet string
				val          int
			}{r.name, r.planet, val})
		}
	}
	viewName := ipScoreViewName(s, kind)
	rule := strings.Repeat("─", 72)
	// Title: "<game name>: <view name>" (bright-white + gray ": " + bright-red name).
	//
	// DELIBERATE DIVERGENCE, and one that must never be "corrected" back: the
	// original heads this screen with its OWN product name, so copying the layout
	// faithfully means substituting ours. A title is branding, not a mechanic —
	// see the licence note in the bre-gather skill. IB printed the original's name
	// here until 2026-08-23.
	fmt.Fprintf(s, "\n%s%s%s: %s%s%s\n", ansi.FgBrightWhite, tr(s, "Immortal Barons"),
		ansi.FgBrightBlack, ansi.FgBrightRed, viewName, ansi.Reset)
	// Planetary Post banner: ──═Planetary Post═── (red ──, bright-red ═, bright-white title).
	fmt.Fprintf(s, "%s                          %s──%s═%s%s%s═%s──%s\n",
		ansi.FgRed, ansi.FgRed, ansi.FgBrightRed,
		ansi.FgBrightWhite, tr(s, "Planetary Post"), ansi.FgBrightRed,
		ansi.FgRed, ansi.Reset)
	fmt.Fprintf(s, "\n")
	// Column header.
	if isPlanet {
		fmt.Fprintf(s, "%s      %-34s %12s%s\n", ansi.FgBrightWhite, tr(s, "Name"), metric, ansi.Reset)
	} else {
		fmt.Fprintf(s, "%s      %-26s %-20s %12s%s\n", ansi.FgBrightWhite, tr(s, "Name"), tr(s, "Planet"), metric, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack, rule, ansi.Reset)
	for i, r := range viewRows {
		if isPlanet {
			fmt.Fprintf(s, "%s(%s%3d%s) %s%-34s%s %s%12d%s\n",
				ansi.FgRed, ansi.FgBrightRed, i+1, ansi.FgRed,
				ansi.FgWhite, game.FitColumn(r.name, 33), ansi.Reset,
				ansi.FgBrightWhite, r.val, ansi.Reset)
		} else {
			fmt.Fprintf(s, "%s(%s%3d%s) %s%-26s%s %s%-20s%s %s%12d%s\n",
				ansi.FgRed, ansi.FgBrightRed, i+1, ansi.FgRed,
				ansi.FgWhite, game.FitColumn(r.name, 25), ansi.Reset,
				ansi.FgWhite, game.FitColumn(r.planet, 19), ansi.Reset,
				ansi.FgBrightWhite, r.val, ansi.Reset)
		}
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack, rule, ansi.Reset)
	pause(s)
}

func metricValue(kind ipRankKind, score, nw, land int) int {
	switch kind {
	case ipRankPlanetScore, ipRankPlayerScore:
		return score
	case ipRankPlanetNW, ipRankPlayerNW:
		return nw
	case ipRankPlanetLand, ipRankPlayerLand:
		return land
	case ipRankPlanetDensity, ipRankPlayerDensity:
		if land > 0 {
			return nw / land
		}
	}
	return 0
}

// createGroupAttack assembles an interplanetary strike against an empire on
// another planet (chosen from imported scores). Barons commit troopers (BRE's
// model — real forces, not gold); the pooled troopers become the strike's
// offense on departure.
// askOneOrAll asks BRE's whole-planet question and takes one key for it. The
// capture (docs/dev/bre-screens.md, "Create Group Attack") shows the "A" answer
// echoing "Entire Planet"; the "O" wording is IB's, since no capture has it. The
// prompt offers no default, so it waits for one of the two letters — a player
// who arrives here by mistake leaves BRE's own way, by sending no forces.
// Reports false, false when the session ends under it.
func askOneOrAll(s session.Session) (all, answered bool) {
	fmt.Fprintf(s, "\n%s ", tr(s, "Do you wish to target (O)ne Dominion or (A)ll?"))
	for {
		r, err := readKey(s)
		if err != nil {
			return false, false
		}
		drainInput(s) // drop a trailing Enter typed with the single-key answer
		switch r {
		case 'a', 'A':
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, tr(s, "Entire Planet"), ansi.Reset)
			return true, true
		case 'o', 'O':
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, tr(s, "One Dominion"), ansi.Reset)
			return false, true
		}
	}
}

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
	// Whole planet or one baron is asked BEFORE the roster, and answered with one
	// key, as BRE does (#125). Taking it first is the point: a planet-wide strike
	// names no target, so it never fetches or draws the baron list at all.
	all, answered := askOneOrAll(s)
	if !answered {
		return Stay
	}
	pick := tr(s, "the whole planet")
	var target string
	if !all {
		names, hidden := attackableBarons(rb.Scores, hostile)
		noteProtectedHidden(s, hidden)
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which baron?"), ansi.Reset)
		if pick = pickFromList(s, "Baron", names); pick == "" {
			return Stay
		}
		target = pick
	}
	// BRE asks for the wait in HOURS, floor 12 and ceiling 120, before the force
	// prompts (docs/dev/bre-screens.md, "Create Group Attack"). The window is what
	// makes the timing a decision: a strike can be aimed to land before the
	// target's next turn, which a whole-day delay cannot express (#124).
	hours := promptSuggestedTight(s,
		fmt.Sprintf(tr(s, "Wait how many Hours (%d-%d)?"), game.GroupAttackHoursMin, game.GroupAttackHoursMax),
		game.GroupAttackHoursMin, game.GroupAttackHoursMax)
	force := promptAttackForce(s, p)
	if force.Empty() {
		return Stay
	}
	var id int
	var departAt time.Time
	err := w.mutatePlayer(func(p *game.Empire) error {
		ga, e := w.World.CreateGroupAttack(p, board, target, hours, force)
		if e != nil {
			return e
		}
		id, departAt = ga.ID, ga.DepartAt
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Group attack #%d formed against %s on %s, leaving at %s.", id, pick, board, departAt.Format(departFormat))
	return Stay
}

// departFormat is how a group attack's departure is shown. It is a wall-clock
// instant rather than a game day, so the hour has to be on it — that is the
// whole point of asking in hours.
const departFormat = "01/02 15:04"

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
	now := time.Now()
	w.With(func() {
		if w.Player() == nil {
			return
		}
		for _, ga := range w.GroupAttacks {
			if ga.Due(now, w.GameDay) {
				continue
			}
			tgt := ga.TargetEmpire
			if tgt == "" {
				tgt = tr(s, "the whole planet")
			}
			rows = append(rows, gaRow{ga.ID, fmt.Sprintf("#%d -> %s on %s (leaves %s, %s offense)",
				ga.ID, tgt, ga.TargetBoard, ga.DepartAt.Format(departFormat), comma(ga.Offense()))})
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

// promptAttackForce asks how many of each unit type to commit. Captured live
// from BRE (docs/dev/bre-screens.md): both the individual and the group picker
// ask about every type, including ones held at zero — "Send how many Jets?
// (0; 0)" — and every default is 0, never "send everything".
func promptAttackForce(s session.Session, p *game.Empire) game.AttackForce {
	var f game.AttackForce
	fmt.Fprint(s, "\n")
	f.Troopers = promptSuggestedTight(s, "Send how many Troopers?", 0, p.Troopers)
	f.Jets = promptSuggestedTight(s, "Send how many Jets?", 0, p.Jets)
	f.Tanks = promptSuggestedTight(s, "Send how many Tanks?", 0, p.Tanks)
	f.Bombers = promptSuggestedTight(s, "Send how many Bombers?", 0, p.Bombers)
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
	kind, chose := promptAttackKind(s, w)
	if !chose {
		fail(s, errAttackAborted)
		return Stay
	}
	force := promptAttackForce(s, w.Player())
	if force.Empty() {
		return Stay
	}
	okNoPause(s, "This attack will cost %s gold.", comma(w.AttackGoldCost(force)))
	if !askYesNoHere(s, "Send this Attack?", true) {
		return Stay
	}
	var id int
	err := w.mutatePlayer(func(p *game.Empire) error {
		n, e := w.World.CreateIndividualAttack(p, board, target, kind, force)
		id = n
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "%s force #%d is on its way to %s on %s.", kind, id, target, board)
	return Stay
}

// errAttackAborted is BRE's wording when the attack-type menu is quit out of.
var errAttackAborted = errors.New("Attack aborted.")

// showAttackTypeHelp renders the Attack Types topic, which is where the three
// variants' figures live. BRE's own menu carries the same (?) Help item, and
// its help shows the same numbers, so the topic is the single source rather
// than a blurb duplicated onto the menu.
func showAttackTypeHelp(s session.Session, w *ctx) {
	lang := "en"
	if p := w.Player(); p != nil && p.Language != "" {
		lang = p.Language
	}
	t, found := help.TopicByPath("interbbs/attack-types.md", lang)
	if !found {
		return
	}
	fmt.Fprintf(s, "\n%s\n", t.RenderANSI(78))
	pause(s)
}

// promptAttackKind is BRE's attack-type menu, the choice an individual strike
// makes and a group attack does not. Captured live from the original
// (docs/dev/bre-screens.md): a 21-column box in red, `────[Attack Type]────`
// over the items, a Help item that shows the same figures the help topic
// carries, and Enter taking Quit — the ordinary menu default, NOT Normal
// Attack. chose is false when the baron quit out.
func promptAttackKind(s session.Session, w *ctx) (kind game.AttackKind, chose bool) {
	rows := []struct {
		kind  game.AttackKind
		label string
	}{
		{game.NormalAttack, "Normal Attack"},
		{game.QuickStrike, "Quick Strike"},
		{game.ExtendedBattle, "Extended Battle"},
	}
	for {
		fmt.Fprintf(s, "\n%s────%s[%s%s%s]%s────%s\n",
			ansi.FgRed, ansi.FgBrightRed, ansi.FgBrightWhite, tr(s, "Attack Type"),
			ansi.FgBrightRed, ansi.FgRed, ansi.Reset)
		for i, r := range rows {
			fmt.Fprintf(s, "%s(%s%d%s) %s%s%s\n",
				ansi.FgRed, ansi.FgBrightRed, i+1, ansi.FgRed,
				ansi.FgWhite, tr(s, r.label), ansi.Reset)
		}
		fmt.Fprintf(s, "%s(%s?%s) %s%s%s\n", ansi.FgRed, ansi.FgBrightRed, ansi.FgRed, ansi.FgWhite, tr(s, "Help"), ansi.Reset)
		fmt.Fprintf(s, "%s(%s0%s) %s%s%s\n", ansi.FgRed, ansi.FgBrightRed, ansi.FgRed, ansi.FgWhite, tr(s, "Quit"), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgRed, strings.Repeat("─", 21), ansi.Reset)

		n, helpWanted := choiceQuitOrHelp(s, len(rows))
		if helpWanted {
			showAttackTypeHelp(s, w)
			continue
		}
		if n < 1 {
			return 0, false
		}
		return rows[n-1].kind, true
	}
}

// hostile says whether a target list is being drawn for something New Realm
// Protection stops. Spying is not — a protected realm can still be looked at —
// so an observing caller sees every realm.
const (
	hostile   = true
	observing = false
)

// attackableBarons lists the barons a planet's last scores packet leaves open to
// `hostile` work, and how many it held back. A local attack list hides protected
// realms the same way, and offering one costs the attacker forces for a strike
// the target board will refuse on arrival. What we know can be stale, so this is
// a courtesy, not the enforcement — that stays with the target board
// (game.resolveRemoteAttack).
func attackableBarons(scores []game.RemoteScore, hostile bool) (names []string, hidden int) {
	for _, sc := range scores {
		if hostile && sc.Protected {
			hidden++
			continue
		}
		names = append(names, sc.Empire)
	}
	return names, hidden
}

// noteProtectedHidden says why a planet's list is shorter than its score board,
// so a shrinking list does not read as missing data. It does not pause: a target
// prompt follows immediately.
func noteProtectedHidden(s session.Session, hidden int) {
	if hidden == 1 {
		okNoPause(s, "One realm there is under New Realm Protection and cannot be attacked.")
	} else if hidden > 1 {
		okNoPause(s, "%d realms there are under New Realm Protection and cannot be attacked.", hidden)
	}
}

// pickRemoteBaron asks for a planet and then a named baron on it. Unlike the
// group-attack picker it offers no whole-planet choice, because an individual
// attack has to name its target. Empty strings mean the player backed out.
func pickRemoteBaron(s session.Session, w *ctx) (board, baron string) {
	var boards []string
	var scores map[string][]string
	hidden := map[string]int{}
	w.With(func() {
		scores = map[string][]string{}
		for _, b := range w.RemoteBoards {
			boards = append(boards, b.BoardID)
			names, n := attackableBarons(b.Scores, hostile)
			scores[b.BoardID], hidden[b.BoardID] = names, n
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
		if hidden[board] > 0 {
			ok(s, "Every realm known on that planet is under New Realm Protection.")
		} else {
			ok(s, "No barons are known on that planet yet.")
		}
		return "", ""
	}
	noteProtectedHidden(s, hidden[board])
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
		fmt.Fprintf(s, "%s%-*s%s%s%s\n", ansi.FgWhite, travelNameWidth, game.FitColumn(p.name, travelNameWidth-1), col, label, ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	pause(s)
	return Stay
}

// turnaroundLabel renders one average round trip and its color. BRE quantizes
// before printing — hours to a tenth, days to a hundredth — so the figures land
// on the same values the original shows. The seconds and minutes tiers are IB's
// own: BRE never needed them, but a link that answers in under a minute would
// otherwise print 0.00 hours and read as no measurement at all.
func turnaroundLabel(s session.Session, days float64) (string, string) {
	switch {
	case days <= 0:
		return tr(s, "No Data"), ansi.FgRed
	case days < game.TravelSecondsCutoff:
		// Never round a real measurement down to zero — that is the reading this
		// tier exists to avoid.
		return plural(s, math.Max(1, math.Round(days*24*60*60)), "1 second", "%.0f seconds"), ansi.FgBrightGreen
	case days < game.TravelMinutesCutoff:
		return plural(s, math.Round(days*24*60), "1 minute", "%.0f minutes"), ansi.FgBrightGreen
	case days < game.TravelHoursCutoff:
		hours := math.Round(days*24*10) / 10
		return fmt.Sprintf(tr(s, "%.2f hours"), hours), ansi.FgBrightGreen
	}
	return fmt.Sprintf(tr(s, "%.2f days"), math.Round(days*100)/100), ansi.FgCyan
}

// plural renders a whole-number count, picking the singular wording at one. The
// two forms are separate translatable strings because a PO catalogue cannot
// derive one from the other, and languages do not agree on where the plural
// starts.
func plural(s session.Session, n float64, one, many string) string {
	if n == 1 {
		return tr(s, one)
	}
	return fmt.Sprintf(tr(s, many), n)
}

// Planetary Treaties geometry, measured off a live BRE capture: a 50-column
// inset rule with a 10-column double run, "( N) " ahead of each row, and the
// planet name in 35 columns before its status.
const (
	treatyRuleWidth  = 50
	treatyRuleDouble = 10
	treatyNameWidth  = 35
)

// planetaryTreaties is the InterPlanetary Ops "Diplomacy List": BRE's
// "Planetary Treaties" chart of where this planet stands with each of the
// others. The planet the caller is on is not in it — a board has no standing
// with itself.
func planetaryTreaties(s session.Session, w *ctx) Result {
	type row struct {
		number   int
		name     string
		relation game.PlanetRelation
	}
	var rows []row
	w.With(func() {
		for _, p := range w.LeaguePlanets() {
			if p.Name == w.Config.BoardID {
				continue
			}
			rows = append(rows, row{p.Number, p.Name, w.PlanetRelationWith(p.Name)})
		}
	})
	if len(rows) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	rule := ansi.FgBrightBlack + insetRule(treatyRuleWidth, treatyRuleDouble) + ansi.Reset
	fmt.Fprintf(s, "\n%s──%s═%s%s%s═%s──%s\n",
		ansi.FgRed, ansi.FgBrightRed, ansi.FgBrightWhite, tr(s, "Planetary Treaties"),
		ansi.FgBrightRed, ansi.FgRed, ansi.Reset)
	fmt.Fprintf(s, "%s\n", rule)
	for _, r := range rows {
		fmt.Fprintf(s, "%s(%s%2d%s) %s%-*s%s%s\n",
			ansi.FgBrightBlack, ansi.FgBrightWhite, r.number, ansi.FgBrightBlack,
			ansi.FgBrightWhite, treatyNameWidth, r.name,
			relationColored(s, r.relation), ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	pause(s)
	return Stay
}

// pickRemoteTarget prompts for a planet then a baron on it, returning the
// board, the baron's name, and its imported score. found is false if the caller
// cancels or no planets/barons are known.
func pickRemoteTarget(s session.Session, w *ctx, planetPrompt, baronPrompt string, hostile bool) (board, baron string, sc game.RemoteScore, found bool) {
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
	names, protected := attackableBarons(rb.Scores, hostile)
	if len(names) == 0 {
		ok(s, "Every realm known on that planet is under New Realm Protection.")
		return "", "", sc, false
	}
	noteProtectedHidden(s, protected)
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

// sendSpyGuy is the Special Operations "Send SpyGuy" item: post a watcher on
// another PLANET for a paid number of days. He is not a covert agent — no agent
// is spent, he cannot be caught, and he brings back no intelligence. What he
// does is send word home the moment his hosts aim a group attack or a Clingy
// Annihilator at his own planet, and that word arrives as planet news there.
// The model is BRE's, read out of the binary; see internal/game/spyguy.go.
func sendSpyGuy(s session.Session, w *ctx) Result {
	// The original gates the whole Special Operations node on the caller's own
	// protection — the InterPlanetary menu tests it when '8' is pressed, with no
	// exemption for any item inside (`BRE.OVR 0x020F88`). IB gates the items
	// instead, and this one had been missed.
	if blockedByCovertProtection(s, w) {
		return Stay
	}
	var planets []string
	var perDay int64
	var maxDays int
	w.With(func() {
		planets = w.KnownBoards()
		perDay = w.SpyGuyCostPerDay()
		maxDays = w.SpyGuyDaysAffordable(w.Player())
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	// The price is quoted before the target is picked, as BRE quotes it: it is
	// the same on every planet, being drawn from the sender's own size.
	fmt.Fprintf(s, "\n%s"+tr(s, "A SpyGuy costs %s%s%s gold per day.")+"%s\n",
		ansi.FgWhite, ansi.FgBrightCyan, comma(perDay), ansi.FgWhite, ansi.Reset)
	if maxDays < 1 {
		fail(s, game.ErrCantAfford)
		return Stay
	}
	board := pickPlanetNamed(s, w, planets)
	if board == "" {
		return Stay
	}
	suggested := game.SpyGuyDefaultDays
	if suggested > maxDays {
		suggested = maxDays
	}
	days := promptSuggested(s, "How many days would you like him to remain?", suggested, maxDays)
	if days < 1 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendSpyGuy(p, board, days)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your SpyGuy leaves for %s, and will watch it for %d days.", board, days)
	return Stay
}

// ipSpecialOp drives every item on the interplanetary Special Operations menu
// (#49): choose a target, quote the price, confirm, and send. What "a target"
// means depends on the op — a planet for the four bombing ops, a named baron for
// the three missiles — which is why the choice branches below.
//
// The strike itself happens on the target's board when the packet lands, so the
// screen promises a report rather than printing an outcome — the same shape as
// Terrorist Ops above, and for the same reason.
func ipSpecialOp(op game.SpecialOp) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		if blockedByProtection(s, w) {
			return Stay
		}
		// Checked before a planet is picked so a baron who cannot deliver a
		// payload is not walked through choosing a target first.
		if w.Player().Bombers < game.BombingBombersRequired {
			fail(s, game.ErrNeedBombers)
			return Stay
		}
		// The R5-Slappenheimer's handling mode is the sysop's, and this is the
		// only menu that fires one: BRE's S3-Sabre is an interplanetary Special
		// Operation, so the local Covert menu never had it.
		if op == game.OpSlappenheimer {
			var mode game.SlappenheimerMode
			w.With(func() { mode = w.Config.SlappenheimerHandling })
			if mode == game.SlappenheimerNone {
				ok(s, "The R5-Slappenheimer is disabled.")
				return Stay
			}
			// Under User Select handling the player dials the missile in (0-10).
			// The dial is BRE's bluff — it changes nothing about the outcome —
			// but we still prompt for it to keep the original's feel.
			if mode == game.SlappenheimerUserSelect {
				promptInt(s, "Set the R5-Slappenheimer dial (0-10)")
			}
		}
		label := game.SpecialOpLabel(op)
		var board, baron string
		if op.TargetsPlanet() {
			// The four bombing ops wreck what the whole planet shares — its food
			// market, its trading market, its bank — so there is no baron to
			// name and asking for one would be asking a question with no answer.
			var boards []string
			w.With(func() {
				for _, b := range w.World.RemoteBoards {
					boards = append(boards, b.BoardID)
				}
			})
			if len(boards) == 0 {
				ok(s, "No other planets are known yet.")
				return Stay
			}
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightYellow, fmt.Sprintf(tr(s, "%s against which planet?"), label), ansi.Reset)
			if board = pickPlanetNamed(s, w, boards); board == "" {
				return Stay
			}
		} else {
			var found bool
			board, baron, _, found = pickRemoteTarget(s, w,
				fmt.Sprintf(tr(s, "%s against which planet?"), label),
				fmt.Sprintf(tr(s, "%s against which baron?"), label), hostile)
			if !found {
				return Stay
			}
		}
		cost := w.World.SpecialOpGoldCost(w.Player(), op)
		okNoPause(s, "This operation will cost %s gold.", comma(cost))
		if !askYesNoHere(s, "Send this Operation?", true) {
			return Stay
		}
		err := w.mutatePlayer(func(p *game.Empire) error {
			return w.World.SendSpecialOp(p, board, baron, op)
		})
		if err != nil {
			fail(s, err)
			return Stay
		}
		if baron == "" {
			ok(s, "Your %s is away against %s. Word will come back with the next packet.", label, board)
			return Stay
		}
		ok(s, "Your %s is away against %s of %s. Word will come back with the next packet.", label, baron, board)
		return Stay
	}
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

// terrorOp returns a handler that sends agents to perform a specific terror
// sub-operation on an enemy baron on another planet. All nine sub-ops share the
// same mechanical effect (each agent destroys 1/7 of a random unit type) but BRE
// carries the op type in the packet so the result report can name it.
func terrorOp(op game.TerrorOpType) Action {
	return func(s session.Session, w *ctx) Result {
		return doTerrorOp(s, w, op)
	}
}

// doTerrorOp is the shared implementation behind the Terrorist Ops submenu.
// The strike is queued and resolves on the target board's next packet run; New
// Realm Protection blocks it.
func doTerrorOp(s session.Session, w *ctx, op game.TerrorOpType) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	if w.Player().Agents < 1 {
		fail(s, game.ErrNoAgents)
		return Stay
	}
	board, baron, _, found := pickRemoteTarget(s, w, "Terrorize which planet?", "Terrorize which baron?", hostile)
	if !found {
		return Stay
	}
	agents := promptSuggested(s, "How many agents to send?", w.Player().Agents, w.Player().Agents)
	if agents <= 0 {
		return Stay
	}
	// BRE prices the op on the menu itself; quote it here too, since the price
	// climbs with the launcher's own region count and is easy to be surprised by.
	okNoPause(s, "This operation will cost %s gold.", comma(w.TerrorOpGoldCost(w.Player())))
	if !askYesNoHere(s, "Send this Operation?", true) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendTerror(p, board, baron, agents, op)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your %s agents depart for %s on %s.", op, baron, board)
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
			// Every realm holding a slot is a candidate, protected ones included,
			// and so are you (#149). BRE builds the picker's key set in
			// choose_target_empire (BRE.OVR 0x01aa99) from one test — the slot's
			// player id > 0 — and its two extra filters are both switched OFF for
			// this call: the net-worth and protection checks ride an argument the
			// vote passes as 0, and the exclude-yourself branch rides another it
			// passes as 1. IB filtered out protected realms, which in a young game
			// is every realm but the voter's own.
			//
			// The voter's OWN protection is a separate gate, and BRE does have it:
			// show_game_settings draws the Coordinator Vote item only when the
			// protection predicate says the caller is clear (see tree.go).
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

// dismantleAnnihilator is BRE's Dismantle Gooie, item 1 of the Coordinator Ops
// menu: the elected Coordinator calls off the planet's doomsday weapon before
// it flies (#45). The builder keeps a dismantle of their own on the weapon's
// own desk for now; #114 covers taking that away, along with launching the
// weapon by hand.
func dismantleAnnihilator(s session.Session, w *ctx) Result {
	var isCoordinator, enabled bool
	var d *game.Annihilator
	w.With(func() {
		isCoordinator = w.BBSCoordinator() == w.Player()
		enabled = w.Config.ClingyAnnihilator
		if w.Annihilator != nil {
			c := *w.Annihilator
			d = &c
		}
	})
	if !isCoordinator {
		ok(s, "Only the BBS Coordinator may stand the weapon down.")
		return Stay
	}
	if !enabled {
		ok(s, "Clingy Annihilator operations are switched off in this game.")
		return Stay
	}
	if d == nil {
		ok(s, "This planet is not building a Clingy Annihilator.")
		return Stay
	}
	if d.Launched {
		ok(s, "The Clingy Annihilator has already launched. Nothing can call it back.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "The Clingy Annihilator is aimed at %s.")+"%s\n", ansi.FgWhite, d.TargetBoard, ansi.Reset)
	okNoPause(s, "Dismantling it refunds nothing — the gold is spent.")
	if !AskYesNo(s, "Stand it down?", false) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		// Re-checked inside the transaction: a vote on another node can unseat
		// the caller between the gate above and here.
		return w.World.DismantleAnnihilatorByCoordinator(p)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "The Clingy Annihilator has been dismantled.")
	return Stay
}

// diplomacyModification is the BBS Coordinator's Diplomacy Modification screen:
// pick a planet, file where this one stands with it. What that does and does
// not mean is BRE's own note, reproduced below — the chart tells this planet's
// own players something and binds nothing.
func diplomacyModification(s session.Session, w *ctx) Result {
	var isCoordinator bool
	w.With(func() { isCoordinator = w.BBSCoordinator() == w.Player() })
	if !isCoordinator {
		ok(s, "Only the BBS Coordinator may set planetary diplomacy.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Diplomacy Modification"), ansi.Reset)
	for _, line := range []string{
		"NOTE: Planetary Diplomacy is your own record. It tells your barons",
		"      where this planet stands with the others, and it is not sent",
		"      to them, so each planet keeps its own view.",
		"      One thing does turn on it: your barons may only trade at a",
		"      planet's market while you have it marked Allied.",
	} {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, tr(s, line), ansi.Reset)
	}
	var planets []string
	w.With(func() { planets = w.KnownBoards() })
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	board := pickPlanetNamed(s, w, planets)
	if board == "" {
		return Stay
	}
	r, ok2 := pickRelation(s)
	if !ok2 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		// Re-check the role against fresh state: a vote on another node may have
		// unseated the player between the check above and here.
		if w.BBSCoordinator() != p {
			return errRealmChanged
		}
		w.SetPlanetRelationWith(board, r)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s %s%s%s: %s%s\n", ansi.FgWhite, tr(s, "Our current relations with"),
		ansi.FgBrightWhite, board, ansi.FgWhite, relationColored(s, r), ansi.Reset)
	return Stay
}

// pickRelation is BRE's "Change status to War, None, Peace, or Ally?" prompt.
// The four keys are the original's; the words they file are the display words
// from its status table, which is why War files Enemy and Ally files Allied.
func pickRelation(s session.Session) (game.PlanetRelation, bool) {
	choices := []struct {
		key   byte
		label string
		rel   game.PlanetRelation
	}{
		{'W', "War", game.PlanetEnemy},
		{'N', "None", game.PlanetNone},
		{'P', "Peace", game.PlanetPeace},
		{'A', "Ally", game.PlanetAllied},
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s%s ", ansi.FgWhite, tr(s, "Change status to"))
	for i, c := range choices {
		if i > 0 {
			fmt.Fprintf(&b, "%s%s", ansi.FgWhite, tr(s, ", "))
		}
		if i == len(choices)-1 {
			fmt.Fprintf(&b, "%s%s", ansi.FgWhite, tr(s, "or "))
		}
		// BRE brightens the key character and prints the rest of the word after
		// it, so the answer key is legible without a bracket around it.
		first, rest := splitFirstRune(tr(s, c.label))
		fmt.Fprintf(&b, "%s%s%s%s", ansi.FgBrightWhite, first, ansi.FgWhite, rest)
	}
	fmt.Fprintf(&b, "%s? %s", ansi.FgWhite, ansi.Reset)
	s.Write([]byte(b.String()))
	for {
		k, err := readKey(s)
		if err != nil {
			return game.PlanetNone, false
		}
		if k == '\r' || k == '\n' {
			fmt.Fprintln(s)
			return game.PlanetNone, false
		}
		for _, c := range choices {
			if unicode.ToUpper(k) == rune(c.key) {
				fmt.Fprintf(s, "%s%s%s\n", relationColor(c.rel), tr(s, c.label), ansi.Reset)
				return c.rel, true
			}
		}
	}
}

// splitFirstRune separates a word's first character from the rest, so a caller
// can color them differently without assuming one byte per character.
func splitFirstRune(word string) (first, rest string) {
	for i, r := range word {
		return string(r), word[i+utf8.RuneLen(r):]
	}
	return "", ""
}

// globalReconRequest is Coordinator Ops item 3: one scouting sweep of the whole
// league, answered with a report on every realm every other board holds. The
// answers land in the planet-wide Spy Database, so the sweep is the Coordinator
// spending their agent on everyone's behalf.
func globalReconRequest(s session.Session, w *ctx) Result {
	var boards int
	err := w.mutatePlayer(func(p *game.Empire) error {
		n, e := w.World.GlobalReconRequest(p)
		boards = n
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	if boards == 0 {
		ok(s, "No other planets are known yet, so there is nobody to scout.")
		return Stay
	}
	ok(s, "Recon requests created to all %d planets. The reports will reach the Spy Database.", boards)
	return Stay
}
