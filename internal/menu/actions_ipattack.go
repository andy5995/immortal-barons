package menu

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_ipattack.go — launching a strike at another planet: the group
// attack, the individual attack and its variants, and picking a target.

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
	board := pickAddressee(s, w, boards)
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
	board = pickAddressee(s, w, boards)
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
	board = pickAddressee(s, w, boards)
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
