package menu

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The IP Messages screens: BRE's five addressing modes, its planet prompt and
// "List of Planets" table, and the relations line it shows before a message
// goes out. The model behind them — who a planet-addressed message reaches —
// is in game/ibbs_messages.go.

// Planet-list geometry, measured off a live BRE capture: a 75-column inset rule
// around the table, a 3-column number field ("## "), and the planet name in 27
// columns before the location.
const planetNameWidth = 27

// knownPlanets is World.LeaguePlanets under the world lock. Callers must not
// already hold it.
func knownPlanets(w *ctx) []game.LeagueNode {
	var planets []game.LeagueNode
	w.Read(func() { planets = w.LeaguePlanets() })
	return planets
}

// planetsNamed keeps the league planets whose names are in want, so a screen
// that can only target boards it has scores for still shows them with their
// roster number and location.
func planetsNamed(w *ctx, want []string) []game.LeagueNode {
	var list []game.LeagueNode
	for _, p := range knownPlanets(w) {
		if slices.Contains(want, p.Name) {
			list = append(list, p)
		}
	}
	return list
}

// addressablePlanets is knownPlanets for a screen that will SEND to whatever
// the player names. A board this one has only heard from over a packet is
// listed with an invented node number that routing cannot place, so a packet
// addressed to it is passed from board to board until the hop cap destroys it
// (game.Routable). It stays on the screens that merely SHOW the league — the
// planet list, the treaty chart, travel times — because a board heard from
// before the Coordinator's roster catches up is normal and becomes routable on
// its own. It just cannot be picked as a destination.
func addressablePlanets(w *ctx) []game.LeagueNode {
	var list []game.LeagueNode
	w.With(func() {
		for _, p := range w.LeaguePlanets() {
			if w.Routable(p.Name) {
				list = append(list, p)
			}
		}
	})
	return list
}

// addressable keeps the boards a packet from here can actually reach.
func addressable(w *ctx, boards []string) []string {
	var out []string
	w.With(func() {
		for _, b := range boards {
			if w.Routable(b) {
				out = append(out, b)
			}
		}
	})
	return out
}

// pickAddressee is pickPlanetNamed for a screen that will send a packet to the
// planet chosen, so an unroutable one is never offered.
func pickAddressee(s session.Session, w *ctx, want []string) string {
	reach := addressable(w, want)
	if len(reach) == 0 {
		ok(s, "None of those planets is on the league roster, so nothing can be sent to them yet.")
		return ""
	}
	return pickPlanetNamed(s, w, reach)
}

// nodeLocation joins a roster entry's city/state/country the way BRE's planet
// list prints it, skipping the parts a sysop left blank.
func nodeLocation(n game.LeagueNode) string {
	var parts []string
	for _, p := range []string{n.City, n.State, n.Country} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// showPlanetList draws BRE's "List of Planets" table: a red-ruled board of
// number, planet name and location.
func showPlanetList(s session.Session, t Term, planets []game.LeagueNode) {
	rule := rule75(ansi.FgRed)
	fmt.Fprintf(s, "%s\n", tr(s, "List of Planets"))
	fmt.Fprintf(s, "%s\n", rule)
	fmt.Fprintf(s, "%s%-3s%-*s%s%s\n", ansi.FgWhite, tr(s, "##"), planetNameWidth, tr(s, "Planet Name"), tr(s, "Location"), ansi.Reset)
	for _, p := range planets {
		fmt.Fprintf(s, "%s%2d %s%s %s%s%s\n",
			ansi.FgBrightRed, p.Number,
			ansi.FgBrightWhite, padColumn(t, p.Name, planetNameWidth-1),
			ansi.FgWhite, nodeLocation(p), ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
}

// pickPlanet runs BRE's planet prompt: a name or a number, "?" for the list,
// and an empty line to stop (which the original answers "None"). It returns nil
// when the caller is done choosing. A chosen planet is followed by the standing
// with it, which is where the original prints that line.
func pickPlanet(s session.Session, w *ctx, planets []game.LeagueNode) *game.LeagueNode {
	for {
		// Colors from a live capture (cap/eots-ibbs-01.cap): the label and the
		// ": " white, the parens bright black, the "?" bright white, and what the
		// caller types echoes bright yellow.
		fmt.Fprintf(s, "\n%s%s %s(%s?%s %s%s)%s: %s",
			ansi.FgWhite, tr(s, "Enter Planet Name or Number"),
			ansi.FgBrightBlack, ansi.FgBrightWhite, ansi.FgWhite, tr(s, "for list"),
			ansi.FgBrightBlack, ansi.FgWhite, ansi.FgBrightYellow)
		// Peek the first key so "?" acts at once. It is a shortcut, not an
		// answer — asking to SEE the list and then pressing Enter to prove it is
		// a keystroke that buys nothing, and BRE's own single-key screens set the
		// expectation that "?" just shows you the list.
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return nil
		}
		if r == '?' {
			fmt.Fprintf(s, "?%s\n", ansi.Reset)
			showPlanetList(s, w.Term, planets)
			continue
		}
		// Anything else is the start of a typed answer, read with the original's
		// live completion (see readPlanetAnswer).
		line := ""
		if r != '\r' && r != '\n' {
			line, err = readPlanetAnswer(s, planets, r)
		} else {
			fmt.Fprint(s, "\n")
		}
		fmt.Fprint(s, ansi.Reset)
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgCyan, tr(s, "None"), ansi.Reset)
			return nil
		}
		if p := matchPlanet(planets, line); p != nil {
			showRelation(s, w, p.Name)
			return p
		}
	}
}

// readPlanetAnswer reads the planet prompt a keystroke at a time and COMPLETES
// the line the moment what has been typed matches exactly one planet, which is
// what the original does (#183). first is the key the caller already read.
//
// Why this is worth a loop of its own rather than ReadLineFrom: matching only on
// ENTER makes the player guess whether they have typed enough, and a wrong guess
// costs the whole prompt. Watching the line fill in tells them the moment it is
// unambiguous — with a substring match "s" and "st" can each still be several
// planets while "sta" is one, and nothing on screen says which until it happens.
//
// What the player typed is kept apart from what is DISPLAYED. Completing is a
// display decision, so it must not swallow the keys still to come: a player who
// keeps typing the rest of a name they already see, or a paste that arrives
// faster than they could read it, would otherwise append to the completed text
// and turn a match into nonsense. Every key goes into typed; shown follows from
// it and is redrawn only when it changes, which is why an ambiguous keystroke
// leaves the line alone exactly as the original leaves it.
//
// Typed text echoes bright white and a completed name bright yellow, from the
// capture — the colour is what separates what the player wrote from what the
// game filled in. Backspace edits the typed text, so a completion is never a
// trap.
func readPlanetAnswer(s session.Session, planets []game.LeagueNode, first rune) (string, error) {
	var typed, shown []rune
	// redraw brings the line on screen up to date with want, erasing only what
	// actually has to change — appending a character to a line that is merely
	// growing must not rewrite the whole thing.
	redraw := func(want []rune, color string) {
		common := 0
		for common < len(shown) && common < len(want) && shown[common] == want[common] {
			common++
		}
		if common == len(shown) && common == len(want) {
			return
		}
		fmt.Fprint(s, strings.Repeat("\b \b", len(shown)-common))
		fmt.Fprintf(s, "%s%s", color, string(want[common:]))
		shown = append(append([]rune(nil), want[:common]...), want[common:]...)
	}
	settle := func() {
		if p := matchPlanet(planets, string(typed)); p != nil {
			redraw([]rune(p.Name), ansi.FgBrightYellow)
			return
		}
		redraw(typed, ansi.FgBrightWhite)
	}
	if first >= 32 {
		typed = append(typed, first)
		settle()
	}
	for {
		r, err := s.ReadKey()
		if err != nil {
			return string(shown), err
		}
		switch {
		case r == '\r' || r == '\n':
			fmt.Fprint(s, "\n")
			// The completed name, when one resolved: it is what the player was
			// shown, so it is what they answered.
			return string(shown), nil
		case r == '\b' || r == 127:
			if len(typed) > 0 {
				typed = typed[:len(typed)-1]
				settle()
			}
		case r >= 32:
			typed = append(typed, r)
			settle()
		}
	}
}

// matchPlanet resolves a typed planet number or name, as BRE's prompt offers
// ("Enter Planet Name or Number"). BINARY-VERIFIED against select_planet
// (BRE.OVR 0x021dd9, container ovr_0209c5), which settles #183:
//
//   - An exact roster number resolves on its own, checked before anything else
//     and only when that slot is occupied (unit offset 0x1861: Val(typed), then
//     the slot's own occupied byte at +0x28ba).
//   - Otherwise the typed text is matched as a SUBSTRING, not a prefix —
//     confirmed by watching it, not only by reading it: against a roster of Nova
//     Hub / Starship Junkyard / Eye of the Storm / The Eclipse, "s" and "st"
//     both leave the line alone (3 matches then 2, counting Storm and Eclipse)
//     and "sta" completes. A prefix matcher would have finished on the "s". The
//     original upper-cases both sides and calls Turbo Pascal's Pos(typed, name)
//     — argument order confirmed at 0x1535-0x1560 — so "Bit" finds "The X-Bit
//     BBS" from the middle. It runs the same Pos against the planet's NUMBER
//     rendered as a string (0x1585), which is why a bare digit can match several
//     planets once the roster passes nine.
//   - It COUNTS the matches rather than taking the first ([bp-0x30e], 0x1521),
//     and only a count of exactly ONE resolves: 0 sets state 7, 1 sets 14, more
//     than 1 sets 15, and the loop that identifies the slot runs only for 1
//     (0x15c0). Nothing is selected for 0 or for many, and the routine holds no
//     message distinguishing them — an ambiguous answer is refused exactly like
//     an unknown one, which is the safe half of the question #183 left open.
//
// The original matches as you TYPE, completing the line the moment the count
// reaches one, which is what a capture shows as two characters erased and
// replaced by the full name (docs/dev/bre-screens.md). IB resolves the same
// text on ENTER instead; the accepted keystrokes are the same, the live
// completion is not built.
func matchPlanet(planets []game.LeagueNode, typed string) *game.LeagueNode {
	if n, err := strconv.Atoi(typed); err == nil {
		for i := range planets {
			if planets[i].Number == n {
				return &planets[i]
			}
		}
		return nil
	}
	typed = strings.ToUpper(strings.TrimSpace(typed))
	if typed == "" {
		return nil
	}
	var found *game.LeagueNode
	for i := range planets {
		if !strings.Contains(strings.ToUpper(planets[i].Name), typed) &&
			!strings.Contains(strconv.Itoa(planets[i].Number), typed) {
			continue
		}
		if found != nil {
			return nil // more than one: refused, as the original refuses it
		}
		found = &planets[i]
	}
	return found
}

// pickPlanetNamed is the whole prompt for a screen that already knows which
// planets it may target: BRE asks for a planet the same way everywhere, so
// every targeting screen goes through here. Returns "" if the caller cancels or
// none of the named planets is known.
func pickPlanetNamed(s session.Session, w *ctx, want []string) string {
	list := planetsNamed(w, want)
	if len(list) == 0 {
		return ""
	}
	if p := pickPlanet(s, w, list); p != nil {
		return p.Name
	}
	return ""
}

// relationColor is the color BRE prints each planetary status in (live
// capture): peace green, an alliance blue, and no standing in the body color.
// Enemy is the one value the capture never showed, so its red is IB's
// inference from the palette the rest of the game uses for hostility.
func relationColor(r game.PlanetRelation) string {
	switch r {
	case game.PlanetAllied:
		return ansi.FgBrightBlue
	case game.PlanetPeace:
		return ansi.FgBrightGreen
	case game.PlanetEnemy:
		return ansi.FgBrightRed
	}
	return ansi.FgWhite
}

// relationColored is one status ready to print. The word carries the meaning on
// its own, so a reader who cannot tell the colors apart loses nothing.
func relationColored(s session.Session, r game.PlanetRelation) string {
	return relationColor(r) + tr(s, string(r))
}

// showRelation prints BRE's "Our current relations with X" line. The original
// prints it from the shared planet prompt, so it follows every planet a player
// names — a message, a terror op, a database lookup.
func showRelation(s session.Session, w *ctx, planet string) {
	var r game.PlanetRelation
	w.Read(func() { r = w.PlanetRelationWith(planet) })
	fmt.Fprintf(s, "%s%s %s%s%s: %s%s\n",
		ansi.FgWhite, tr(s, "Our current relations with"),
		ansi.FgBrightWhite, planet, ansi.FgWhite,
		relationColored(s, r), ansi.Reset)
}

// The five IP Messages items: one planet, as many as the sender keeps naming,
// the whole league, allied planets (which IB has no list for), and one planet's
// elected Coordinator.
func ipMessageSingle(s session.Session, w *ctx) Result {
	return ipMessageToOnePlanet(s, w, false)
}

func ipMessageCoordinator(s session.Session, w *ctx) Result {
	return ipMessageToOnePlanet(s, w, true)
}

// ipMessageToOnePlanet is Single Planet and Planet Coordinator, which differ
// only in how narrowly the far side delivers what arrives.
//
// Single Planet then asks WHO on that planet, as the original does: naming the
// planet is only half the address, and BRE follows it with the same
// `(A-Y,Z=All,?=List) Send to:` toggle list local mail uses
// (send_interbbs_message, BRE.OVR 0x1f335, calling the recipient prompt at
// 0x1ed14 and its per-letter toggle at 0x1f1ac). IB went straight to the editor
// and had no way to write to one baron (#193).
//
// Planet Coordinator skips the roster: that address names an office, not a
// realm.
func ipMessageToOnePlanet(s session.Session, w *ctx, toCoordinator bool) Result {
	planets := addressablePlanets(w)
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	p := pickPlanet(s, w, planets)
	if p == nil {
		return Stay
	}
	if toCoordinator {
		return composeIPMessage(s, w, []string{p.Name}, true)
	}
	rows := remoteRecipients(w, p.Name)
	if len(rows) == 0 {
		// No scores packet has arrived from that board yet, so there is no roster
		// to letter. Writing to the planet still works and is what IB did before
		// the picker existed, so the message goes planet-wide rather than the
		// screen refusing to send anything at all.
		return composeIPMessage(s, w, []string{p.Name}, false)
	}
	picked := runPicker(s, w, rows, 0, pickOpts{
		prompt: "Send to:", allowAll: true,
		title: fmt.Sprintf(tr(s, "Players at %s"), p.Name),
	})
	if len(picked) == 0 {
		return Stay
	}
	barons := make([]string, 0, len(picked))
	for _, i := range picked {
		barons = append(barons, rows[i].name)
	}
	return composeIPMessageToBarons(s, w, p.Name, barons)
}

// remoteRecipients letters the barons on another planet for the Send Message
// picker, from the last scores packet that board sent.
//
// THE LETTERS ARE THIS BOARD'S, not the far planet's. BRE's picker letter is a
// raw index into that planet's 25-slot empire array, gaps and all, because BRE
// holds the whole array; IB addresses a realm across the wire by NAME and
// game.RemoteScore carries no slot, so a letter here numbers the rows of the
// packet in the order they arrived. That order is fixed for as long as the
// packet is, which is what makes the letter mean the same realm on the roster
// and at the prompt. It is not the letter that realm answers to at home, and a
// later packet may renumber it — see docs/mechanics-reference.md, "IP Messages".
//
// Recipients are capped at pickLetters because the prompt has no letter for a
// twenty-sixth row. A planet cannot hold more (game.PlanetSlots), so the cap
// only bites on a malformed packet.
func remoteRecipients(w *ctx, board string) []pickRow {
	var rows []pickRow
	w.With(func() {
		for _, b := range w.RemoteBoards {
			if b.BoardID != board {
				continue
			}
			for i, sc := range b.Scores {
				if i >= pickLetters {
					break
				}
				rows = append(rows, pickRow{
					letter: rune('A' + i), name: sc.Empire, protected: sc.Protected,
					land: sc.Land, score: sc.Score, nw: sc.NetWorth,
				})
			}
		}
	})
	return rows
}

// composeIPMessageToBarons writes one message per named baron on board, each
// addressed with game.IPMessage.ToEmpire so the far side drops it in that
// realm's mailbox alone. Several recipients are several messages rather than a
// recipient list on one, which is what lets this ride the packet unchanged.
func composeIPMessageToBarons(s session.Session, w *ctx, board string, barons []string) Result {
	if len(addressable(w, []string{board})) == 0 {
		ok(s, "None of those planets is on the league roster, so nothing can be sent to them yet.")
		return Stay
	}
	text, send := composeMessage(s)
	if !send || strings.TrimSpace(text) == "" {
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
	err := w.mutatePlayer(func(p *game.Empire) error {
		w.World.SendIPMessageToBarons(p, board, barons, text)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your message is on its way to %d baron(s) on %s.", len(barons), board)
	return Stay
}

func ipMessageSelect(s session.Session, w *ctx) Result {
	planets := addressablePlanets(w)
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "To end selection, hit enter at the prompt."), ansi.Reset)
	var chosen []string
	for {
		p := pickPlanet(s, w, planets)
		if p == nil {
			break
		}
		if !slices.Contains(chosen, p.Name) {
			chosen = append(chosen, p.Name)
		}
	}
	if len(chosen) == 0 {
		return Stay
	}
	return composeIPMessage(s, w, chosen, false)
}

// ipMessageAll writes to the rest of the league. It uses KnownBoards rather
// than the picker's list because "all planets" means the other ones — mailing
// your own board is only ever a deliberate choice made by naming it.
func ipMessageAll(s session.Session, w *ctx) Result {
	var all []string
	w.Read(func() { all = w.KnownBoards() })
	if len(all) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	return composeIPMessage(s, w, all, false)
}

// ipMessageAllied writes to every planet the Coordinator's diplomacy chart
// calls Allied. It is the one thing that chart drives rather than describes.
func ipMessageAllied(s session.Session, w *ctx) Result {
	var allied []string
	w.Read(func() { allied = w.AlliedPlanetNames() })
	if len(allied) == 0 {
		ok(s, "We have no allied planets.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s %s%s%s\n", ansi.FgWhite, tr(s, "Writing to:"),
		ansi.FgBrightWhite, strings.Join(allied, ", "), ansi.Reset)
	return composeIPMessage(s, w, allied, false)
}

// composeIPMessage runs the same multi-line editor local mail uses and queues
// the result for each named planet. It leaves with the packets in the outbox —
// they travel on the sysop's next inter-BBS run, which is why the reply comes
// back in hours or days rather than in the same turn.
func composeIPMessage(s session.Session, w *ctx, boards []string, toCoordinator bool) Result {
	// The whole-league and allied modes never went through the picker, and a
	// board can stop being routable between the pick and the send, so the list is
	// filtered once more here — before the sender types anything.
	boards = addressable(w, boards)
	if len(boards) == 0 {
		ok(s, "None of those planets is on the league roster, so nothing can be sent to them yet.")
		return Stay
	}
	text, send := composeMessage(s)
	if !send || strings.TrimSpace(text) == "" {
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
	err := w.mutatePlayer(func(p *game.Empire) error {
		w.World.SendIPMessage(p, boards, toCoordinator, text)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your message is on its way to %d planet(s).", len(boards))
	return Stay
}
