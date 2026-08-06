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
const (
	planetRuleWidth  = 75
	planetRuleDouble = 15
	planetNameWidth  = 27
)

// knownPlanets is World.LeaguePlanets under the world lock. Callers must not
// already hold it.
func knownPlanets(w *ctx) []game.LeagueNode {
	var planets []game.LeagueNode
	w.With(func() { planets = w.LeaguePlanets() })
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
func showPlanetList(s session.Session, planets []game.LeagueNode) {
	rule := ansi.FgRed + insetRule(planetRuleWidth, planetRuleDouble) + ansi.Reset
	fmt.Fprintf(s, "%s\n", tr(s, "List of Planets"))
	fmt.Fprintf(s, "%s\n", rule)
	fmt.Fprintf(s, "%s%-3s%-*s%s%s\n", ansi.FgWhite, tr(s, "##"), planetNameWidth, tr(s, "Planet Name"), tr(s, "Location"), ansi.Reset)
	for _, p := range planets {
		fmt.Fprintf(s, "%s%2d %s%-*s%s%s%s\n",
			ansi.FgBrightRed, p.Number,
			ansi.FgBrightWhite, planetNameWidth, p.Name,
			ansi.FgWhite, nodeLocation(p), ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
}

// pickPlanet runs BRE's planet prompt: a name or a number, "?" for the list,
// and an empty line to stop (which the original answers "None"). It returns nil
// when the caller is done choosing.
func pickPlanet(s session.Session, planets []game.LeagueNode) *game.LeagueNode {
	for {
		fmt.Fprintf(s, "\n%s%s %s(%s?%s %s%s)%s: ",
			ansi.FgWhite, tr(s, "Enter Planet Name or Number"),
			ansi.FgBrightBlack, ansi.FgBrightWhite, ansi.FgWhite, tr(s, "for list"),
			ansi.FgBrightBlack, ansi.Reset)
		line, err := session.ReadLine(s)
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgCyan, tr(s, "None"), ansi.Reset)
			return nil
		case line == "?":
			showPlanetList(s, planets)
			continue
		}
		if p := matchPlanet(planets, line); p != nil {
			return p
		}
	}
}

// matchPlanet resolves a typed planet number or name, as BRE's prompt offers
// ("Enter Planet Name or Number"). The name is matched case-insensitively but
// in full: whether the original accepts an abbreviation was never observed, and
// guessing at one would send a message to the wrong planet. Anything it does not
// recognize returns nil, so the prompt asks again.
func matchPlanet(planets []game.LeagueNode, typed string) *game.LeagueNode {
	if n, err := strconv.Atoi(typed); err == nil {
		for i := range planets {
			if planets[i].Number == n {
				return &planets[i]
			}
		}
		return nil
	}
	for i := range planets {
		if strings.EqualFold(planets[i].Name, typed) {
			return &planets[i]
		}
	}
	return nil
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
	if p := pickPlanet(s, list); p != nil {
		return p.Name
	}
	return ""
}

// planetRelation is this planet's standing with another one. IB has no
// planet-to-planet treaty mechanic, so it is always "None" — as it read on the
// observed BRE board too. Both screens that show a relation go through here, so
// the day planets can hold treaties there is one place to change.
func planetRelation(s session.Session) string { return tr(s, "None") }

// showRelation prints BRE's "Our current relations with X" line, which it shows
// before a message goes to a named planet.
func showRelation(s session.Session, planet string) {
	fmt.Fprintf(s, "%s%s %s%s%s: %s%s\n",
		ansi.FgWhite, tr(s, "Our current relations with"),
		ansi.FgBrightWhite, planet, ansi.FgWhite,
		planetRelation(s), ansi.Reset)
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
func ipMessageToOnePlanet(s session.Session, w *ctx, toCoordinator bool) Result {
	planets := knownPlanets(w)
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	p := pickPlanet(s, planets)
	if p == nil {
		return Stay
	}
	showRelation(s, p.Name)
	return composeIPMessage(s, w, []string{p.Name}, toCoordinator)
}

func ipMessageSelect(s session.Session, w *ctx) Result {
	planets := knownPlanets(w)
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "To end selection, hit enter at the prompt."), ansi.Reset)
	var chosen []string
	for {
		p := pickPlanet(s, planets)
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
	w.With(func() { all = w.KnownBoards() })
	if len(all) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	return composeIPMessage(s, w, all, false)
}

// ipMessageAllied is BRE's Allied Planets item. IB models treaties between
// realms, not between planets, so there is no allied-planet list to write to
// yet; the item stays on the menu because the menu matches BRE's, and it says
// so rather than silently doing nothing.
func ipMessageAllied(s session.Session, w *ctx) Result {
	ok(s, "We have no planetary alliances — planets here hold no treaties with one another.")
	return Stay
}

// composeIPMessage runs the same multi-line editor local mail uses and queues
// the result for each named planet. It leaves with the packets in the outbox —
// they travel on the sysop's next inter-BBS run, which is why the reply comes
// back in hours or days rather than in the same turn.
func composeIPMessage(s session.Session, w *ctx, boards []string, toCoordinator bool) Result {
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
