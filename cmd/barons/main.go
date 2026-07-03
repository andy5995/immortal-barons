// Command barons runs Immortal Barons against the local console. It is the
// simplest front-end: a person plays in their own terminal. The BBS-door
// and web front-ends attach a different Session later.
package main

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
)

func main() {
	c := session.NewConsole()
	defer c.Close()

	splash(c)
	g := game.New()
	menu.Run(c, g, menu.Build())

	fmt.Fprintf(c, "%s\nUntil next turn, Baron.\n", ansi.Reset)
}

func splash(c *session.Console) {
	fmt.Fprintf(c, "%s%s", ansi.Clear, ansi.FgBrightYellow)
	fmt.Fprint(c, `
      IMMORTAL BARONS
   a post-apocalyptic strategy game
`)
	fmt.Fprintf(c, "%s\n  Press any key to take the throne...", ansi.Reset)
	c.ReadKey()
}
