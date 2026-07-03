// Command barons runs Immortal Barons against the local console. It is the
// simplest front-end: a person plays in their own terminal. The BBS-door
// and web front-ends attach a different Session later.
package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
)

const version = "0.0.1"

func main() {
	c := session.NewConsole()
	defer c.Close()

	splash(c)
	g := game.New()
	nameRealm(c, g)
	menu.Run(c, g, menu.Build())

	fmt.Fprintf(c, "%s\nUntil next turn, Baron.\n", ansi.Reset)
}

func splash(c *session.Console) {
	fmt.Fprintf(c, "%s%s", ansi.Clear, ansi.FgBrightYellow)
	fmt.Fprintf(c, `
      IMMORTAL BARONS  v%s
   a post-apocalyptic strategy game
`, version)
	fmt.Fprintf(c, "%s\n  Press any key to take the throne...", ansi.Reset)
	c.ReadKey()
}

// nameRealm asks the player to name their realm, following the original's
// rule: at least 3 letters/numbers, and not the same as a rival's name.
// If the input stream ends, it keeps the default name.
func nameRealm(c *session.Console, w *game.World) {
	taken := map[string]bool{}
	for _, e := range w.Empires {
		if !e.Human {
			taken[strings.ToLower(e.Name)] = true
		}
	}
	fmt.Fprint(c, ansi.Clear)
	for {
		fmt.Fprintf(c, "%sName Your Empire%s\n\n", ansi.FgBrightCyan, ansi.Reset)
		fmt.Fprintf(c, "%sName your Realm:%s ", ansi.FgBrightWhite, ansi.Reset)
		name, err := session.ReadLine(c)
		if err != nil {
			return // stream ended; keep the default realm name
		}
		name = strings.TrimSpace(name)
		if alnumCount(name) < 3 || taken[strings.ToLower(name)] {
			fmt.Fprintf(c, "%s  Your empire name is invalid. It must have at least 3 letters\n"+
				"  and/or numbers, and not match another player.%s\n\n", ansi.FgRed, ansi.Reset)
			continue
		}
		w.Player().Name = name
		return
	}
}

func alnumCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}
