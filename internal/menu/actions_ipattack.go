package menu

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
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
	// Gathered once, under the lock, and read from the copy afterwards (#206).
	// ImportBoard appends to RemoteBoards as inbound packets are applied, which
	// on a multi-node board is another process's transaction — so walking the
	// slice unlocked is a torn read of its header, and a pointer INTO it held
	// across the prompts below can outlive the array it points at.
	var boards []string
	scores := map[string][]remoteBaron{}
	w.Read(func() {
		boards = w.ScoredBoards()
		for _, b := range w.RemoteBoards {
			scores[b.BoardID] = remoteBarons(b.Scores)
		}
	})
	if noScoredPlanets(s, len(boards)) {
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which planet?"), ansi.Reset)
	board := pickAddressee(s, w, boards)
	if board == "" {
		return Stay
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
		if pick = pickRemoteBaronFrom(s, w.Term, scores[board], tr(s, "Target which baron?"), protectedNoStrike); pick == "" {
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
	// One routine in the original prompts for the four counts, quotes the price
	// and asks to confirm, and all three attack paths call it — so a group attack
	// is quoted, confirmed and charged exactly as a strike sent alone (#252).
	okNoPause(s, "This attack will cost %s gold.", comma(w.AttackGoldCost(p, force)))
	if !askYesNoHere(s, "Send this Attack?", true) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		_, e := w.World.CreateGroupAttack(p, board, target, hours, force)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	// Nothing is printed. The original returns straight to the menu on a
	// successful creation — four completed runs in
	// cap/20240527-134Pho_Lazarus_Public.cap agree byte for byte, and
	// create_group_attack calls no printer after the confirm
	// (docs/dev/bre-screens.md). IB announced the party's number and departure
	// here until 2026-09-11: useful, and not what the original does. The number
	// is on the Join Group Attack table, which is where a baron joins by it.
	return Stay
}

// joinGroupAttack adds the player's offense to a group attack still forming.
func joinGroupAttack(s session.Session, w *ctx) Result {
	rows := formingGroupRows(s, w)
	if len(rows) == 0 {
		ok(s, "No group attacks are forming right now.")
		return Stay
	}
	printGroupAttackTable(s, w.Term, rows)
	// Both gates are tested HERE, after the table, and in this order — the
	// original's own (docs/mechanics-reference.md, #162): its refusal sits inline
	// at BRE.OVR 0x02d1c5, after the party table is drawn and before the New
	// Realm Protection test. Reading what is forming costs nothing, and a baron
	// who has not started their turn is exactly the one deciding whether to. IB
	// refused the whole item from the menu until 2026-09-11.
	if !turnPlayedThisEntry(s, w) {
		return Stay
	}
	if blockedByProtection(s, w) {
		return Stay
	}
	// Answered with the Id the table SHOWS, not with a row number: the Id column
	// is the attack's own id and it is what the player is reading off the screen.
	id, slot := promptGroupChoice(s, rows)
	if id == 0 {
		return Stay
	}
	force := promptAttackForce(s, w.Player())
	if force.Empty() {
		return Stay
	}
	okNoPause(s, "This attack will cost %s gold.", comma(w.AttackGoldCost(w.Player(), force)))
	if !askYesNoHere(s, "Send this Attack?", true) {
		return Stay
	}
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
	ok(s, "You joined group attack #%d.", slot)
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
	okNoPause(s, "This attack will cost %s gold.", comma(w.AttackGoldCost(w.Player(), force)))
	if !askYesNoHere(s, "Send this Attack?", true) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		_, e := w.World.CreateIndividualAttack(p, board, target, kind, force)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	// No number: the id a strike carries is a wire key off a counter shared with
	// every other interplanetary action, so it is a running total of everything
	// the board has ever sent rather than anything about this attack (#254). A
	// group attack shows its SLOT because a baron joins one by that number; a
	// lone strike is never referred to again, so there is nothing to show.
	ok(s, "Your %s force is on its way to %s on %s.", kind, target, board)
	return Stay
}

// errAttackAborted is BRE's wording when the attack-type menu is quit out of.
var errAttackAborted = errors.New("Attack aborted.")

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

// remoteBaron is one baron on another planet as this board last heard of them:
// the name a strike is addressed to, and whether that hearing had them under
// New Realm Protection.
type remoteBaron struct {
	name            string
	protected       bool
	land, score, nw int
}

// remoteBarons reads a planet's last scores packet into the rows a target list
// draws. New Realm Protection bars EVERY list this feeds — spying no less than
// striking — so there is no caller that wants the flag suppressed; a pair of
// hostile/observing constants and the parameter selecting between them survived
// here with only the hostile one ever passed.
//
// What we know can be stale, so the flag is a courtesy, not the enforcement —
// that stays with the target board (game.resolveRemoteAttack), which refuses an
// arriving strike on its own authority.
func remoteBarons(scores []game.RemoteScore) []remoteBaron {
	rows := make([]remoteBaron, 0, len(scores))
	for _, sc := range scores {
		rows = append(rows, remoteBaron{
			name: sc.Empire, protected: sc.Protected,
			land: sc.Land, score: sc.Score, nw: sc.NetWorth,
		})
	}
	return rows
}

// pickRemoteBaronFrom draws a planet's barons as the same lettered score table
// the local screens use and reads the choice, refusing a realm the last scores
// packet had under New Realm Protection.
//
// LETTERED, as the original letters every roster of players: its own pickers are
// `Choose a Target [A-Y,?=List RETURN to Abort]` and `(A-Y,Z=All,?=List) Send
// to:`, and only PLANETS are chosen by name or number (select_planet 0x021dd9
// against select_player 0x022a0c). IB numbered this one list until 2026-08-26,
// which was the last place a player was picked any other way.
//
// The protected barons were HIDDEN here until 2026-08-26, with a count printed
// under the list saying how many had been held back. That told the player a name
// existed without saying which, so a planet's roster and its target list
// disagreed with nothing to explain the gap; they are listed now, and a shielded
// realm wears its letter in brackets like everywhere else (#214).
//
// refusal is what the player is told when they pick one, because the reason
// differs by what the list is FOR — a strike is refused, and so is a trade deal
// (#195), but not with the same sentence.
func pickRemoteBaronFrom(s session.Session, t Term, rows []remoteBaron, ask, refusal string) string {
	targets := make([]targetRow, 0, len(rows))
	for i, r := range rows {
		if i >= game.PlanetSlots {
			break // no letter left to give it
		}
		targets = append(targets, targetRow{
			name: r.name, letter: string(rune('A' + i)),
			land: r.land, score: r.score, netWorth: r.nw,
			attackable: !r.protected, protected: r.protected,
		})
	}
	name, _ := pickAttackTarget(s, t, targets, targetPrompt{
		ask:     ask,
		refuse:  refusal,
		nothing: "None of the barons known on that planet can be reached.",
	})
	return name
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
		boards = w.ScoredBoards()
		scores = map[string][]remoteBaron{}
		for _, b := range w.RemoteBoards {
			scores[b.BoardID] = remoteBarons(b.Scores)
		}
	})
	if noScoredPlanets(s, len(boards)) {
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
	baron = pickRemoteBaronFrom(s, w.Term, scores[board], tr(s, baronPrompt), refusal)
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

// formingGroupRows is every party still assembling here, as the table's rows.
// Two screens read it: Join Group Attack, and the Coordinator's own call-off
// (#270), which lists the same parties before asking which one to disband.
func formingGroupRows(s session.Session, w *ctx) []gaRow {
	var rows []gaRow
	now := time.Now()
	w.Read(func() {
		if w.Player() == nil {
			return
		}
		for _, ga := range w.GroupAttacks {
			if ga.Due(now, w.GameDay) {
				continue
			}
			tgt := ga.TargetEmpire
			if tgt == "" {
				// A party aimed at the whole planet names no baron, and the
				// original fills the column with ALL rather than a sentence
				// (cap/20240527-134Pho_Lazarus_Public.cap).
				tgt = tr(s, "ALL")
			}
			r := gaRow{
				// The party's SLOT, which is what the two-column Id field holds
				// and what the prompt is answered with; ga.ID is the world-wide
				// counter behind it and never reaches the screen.
				id:     ga.Slot,
				attack: ga.ID,
				by:     "?",
				planet: ga.TargetBoard,
				target: tgt,
				left:   leftUntil(now, ga),
			}
			if len(ga.Contributors) > 0 {
				if creator := w.FindByOwner(ga.Contributors[0].Owner); creator != nil {
					r.by = creator.Letter()
				}
			}
			// The columns show the force ALREADY POOLED, every contributor's
			// detachment together -- what the player is deciding whether to
			// reinforce, not what any one baron put in.
			for _, c := range ga.Contributors {
				r.troopers += c.Troopers
				r.jets += c.Jets
				r.tanks += c.Tanks
				r.bombers += c.Bombers
			}
			rows = append(rows, r)
		}
	})
	return rows
}
