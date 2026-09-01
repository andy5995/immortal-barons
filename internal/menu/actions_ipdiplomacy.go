package menu

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_ipdiplomacy.go — the BBS Coordinator's chart of where this planet
// stands with each other planet. It binds nothing and is never transmitted;
// see internal/game/ibbs_diplomacy.go.

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
	w.Read(func() {
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
		fmt.Fprintf(s, "%s(%s%2d%s) %s%s%s%s\n",
			ansi.FgBrightBlack, ansi.FgBrightWhite, r.number, ansi.FgBrightBlack,
			ansi.FgBrightWhite, padColumn(w.Term, r.name, treatyNameWidth),
			relationColored(s, r.relation), ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	pause(s)
	return Stay
}

// diplomacyModification is the BBS Coordinator's Diplomacy Modification screen:
// pick a planet, file where this one stands with it. What that does and does
// not mean is BRE's own note, reproduced below — the chart tells this planet's
// own players something and binds nothing.
func diplomacyModification(s session.Session, w *ctx) Result {
	var isCoordinator bool
	w.Read(func() { isCoordinator = w.BBSCoordinator() == w.Player() })
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
	w.Read(func() { planets = w.KnownBoards() })
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	// pickPlanetNamed, not pickAddressee: this chart is kept here and sent
	// nowhere, so a planet the roster cannot place yet can still be marked.
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
