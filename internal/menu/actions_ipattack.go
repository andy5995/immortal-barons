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
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which baron?"), ansi.Reset)
		if pick = pickRemoteBaronFrom(s, remoteBarons(rb.Scores, hostile), protectedNoStrike); pick == "" {
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

// protectedNoStrike is the refusal a war list gives for a realm the last scores
// packet had under New Realm Protection. Takes the realm name.
const protectedNoStrike = "%s is under New Realm Protection and cannot be attacked."

// hostile says whether a target list is being drawn for something New Realm
// Protection stops. Spying is not — a protected realm can still be looked at —
// so an observing caller sees every realm.
const (
	hostile   = true
	observing = false
)

// remoteBaron is one baron on another planet as this board last heard of them:
// the name a strike is addressed to, and whether that hearing had them under
// New Realm Protection.
type remoteBaron struct {
	name      string
	protected bool
}

// remoteBarons reads a planet's last scores packet into the rows a target list
// draws. `hostile` says whether protection is a bar for what the list is being
// drawn for — spying is not, so an observing caller is told nothing about it.
//
// What we know can be stale, so the flag is a courtesy, not the enforcement —
// that stays with the target board (game.resolveRemoteAttack), which refuses an
// arriving strike on its own authority.
func remoteBarons(scores []game.RemoteScore, hostile bool) []remoteBaron {
	rows := make([]remoteBaron, 0, len(scores))
	for _, sc := range scores {
		rows = append(rows, remoteBaron{name: sc.Empire, protected: hostile && sc.Protected})
	}
	return rows
}

// pickRemoteBaronFrom draws the baron list and reads the choice, refusing a
// realm the packet had under New Realm Protection. It returns "" when the
// player backs out or picks a protected baron.
//
// The protected barons were HIDDEN from this list until 2026-08-26, with a count
// printed under it saying how many had been held back. That told the player a
// name existed without saying which, so a planet's roster and its target list
// disagreed with nothing to explain the gap; they are listed with the same `(P)`
// flag the local screens carry now (#214), and the refusal names the realm.
// refusal is what the player is told when they pick a protected realm; it takes
// the realm name. It is a parameter because the reason differs by what the list
// is FOR — a strike is refused, and so is a trade deal (#195), but not with the
// same sentence.
func pickRemoteBaronFrom(s session.Session, rows []remoteBaron, refusal string) string {
	labels := make([]string, len(rows))
	names := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.name + protectedMark(s, r.protected)
		names[i] = r.name
	}
	pick := pickFromListValues(s, "Baron", labels, names)
	if pick == "" {
		return ""
	}
	for _, r := range rows {
		if r.name == pick && r.protected {
			ok(s, refusal, r.name)
			return ""
		}
	}
	return pick
}

// pickRemoteBaron asks for a planet and then a named baron on it, for a strike.
// Unlike the group-attack picker it offers no whole-planet choice, because an
// individual attack has to name its target.
func pickRemoteBaron(s session.Session, w *ctx) (board, baron string) {
	return pickRemoteBaronOn(s, w, "Target which planet?", "Target which baron?", protectedNoStrike)
}

// pickRemoteBaronOn is that walk with its wording supplied: the two prompts and
// what a protected realm is refused WITH. Every caller asks the same two
// questions in the same order and differs only in why it is asking — a strike, a
// trade deal (#195) — so the words are the parameters and the walk is not
// written twice. Empty strings mean the player backed out.
func pickRemoteBaronOn(s session.Session, w *ctx, planetPrompt, baronPrompt, refusal string) (board, baron string) {
	var boards []string
	var scores map[string][]remoteBaron
	w.Read(func() {
		scores = map[string][]remoteBaron{}
		for _, b := range w.RemoteBoards {
			boards = append(boards, b.BoardID)
			scores[b.BoardID] = remoteBarons(b.Scores, hostile)
		}
	})
	if len(boards) == 0 {
		ok(s, "No other planets are known yet. Wait for inter-BBS scores to arrive.")
		return "", ""
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, planetPrompt), ansi.Reset)
	board = pickAddressee(s, w, boards)
	if board == "" {
		return "", ""
	}
	if len(scores[board]) == 0 {
		ok(s, "No barons are known on that planet yet.")
		return "", ""
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, baronPrompt), ansi.Reset)
	baron = pickRemoteBaronFrom(s, scores[board], refusal)
	if baron == "" {
		return "", ""
	}
	return board, baron
}

// pickRemoteTarget is pickRemoteBaronOn plus the chosen baron's imported score,
// for the callers that need the figures as well as the name. found is false if
// the caller backs out or no planets or barons are known.
//
// It walked the planets and barons itself until #195's slop audit: a third
// spelling of one walk, and the only one that read w.RemoteBoards outside a
// transaction — taking a pointer into the slice and using it across a prompt.
func pickRemoteTarget(s session.Session, w *ctx, planetPrompt, baronPrompt string) (board, baron string, sc game.RemoteScore, found bool) {
	board, baron = pickRemoteBaronOn(s, w, planetPrompt, baronPrompt, protectedNoStrike)
	if board == "" || baron == "" {
		return "", "", sc, false
	}
	w.Read(func() {
		for _, b := range w.RemoteBoards {
			if b.BoardID != board {
				continue
			}
			for _, x := range b.Scores {
				if x.Empire == baron {
					sc = x
				}
			}
		}
	})
	return board, baron, sc, true
}
