package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// configEditor is the sysop's Configuration Editor (reached from the
// Coordinator menu). It edits the game-rules fields of the world's Config in
// place and writes them back to config.json. Only game-rules knobs are exposed
// here; the data directory is a launch-time setting, not editable in-game.
//
// Changes take effect going forward: e.g. a new TurnsPerDay applies at the next
// daily maintenance, and AICount does not retroactively add or remove AI
// empires. That is the sysop's job to understand — this screen just stores the
// values.
func configEditor(s session.Session, w *game.World) Result {
	c := &w.Config
	for {
		fmt.Fprint(s, ansi.Clear)
		fmt.Fprintf(s, "%s[Configuration Editor]%s\n%s\n", ansi.FgBrightBlue, ansi.Reset, rule)
		fmt.Fprintf(s, "  1) Turns per day        %d\n", c.TurnsPerDay)
		fmt.Fprintf(s, "  2) Protection turns     %d\n", c.ProtectionTurns)
		fmt.Fprintf(s, "  3) AI empires           %d\n", c.AICount)
		fmt.Fprintf(s, "  4) Game length (days)   %d  (0 = endless)\n", c.GameLength)
		fmt.Fprintf(s, "  5) Inter-BBS play       %s\n", onOffStr(c.IBBS))
		fmt.Fprintf(s, "  6) Board ID             %s\n", c.BoardID)

		switch promptInt(s, "Edit which (0 to save and exit)?") {
		case 0:
			if err := store.SaveConfig(*c); err != nil {
				fail(s, err)
			} else {
				ok(s, "Configuration saved.")
			}
			return Stay
		case 1:
			// A day needs at least one turn, so hold the floor at 1.
			c.TurnsPerDay = max(1, promptSuggested(s, "Turns per day", c.TurnsPerDay, 1000))
		case 2:
			c.ProtectionTurns = promptSuggested(s, "Protection turns", c.ProtectionTurns, 10000)
		case 3:
			c.AICount = promptSuggested(s, "AI empires", c.AICount, 5)
		case 4:
			c.GameLength = promptSuggested(s, "Game length in days (0 = endless)", c.GameLength, 100000)
		case 5:
			c.IBBS = !c.IBBS
		case 6:
			if v := strings.TrimSpace(prompt(s, "Board ID:")); v != "" {
				c.BoardID = v
			}
		}
	}
}

func onOffStr(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}
