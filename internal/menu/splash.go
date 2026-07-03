package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Splash prints the Immortal Barons title banner and a short hints panel,
// then waits for a keypress. Original art/text (not copied from BRE).
func Splash(s session.Session) {
	fmt.Fprint(s, ansi.Clear)
	fmt.Fprintf(s, "%s%s", ansi.FgBrightCyan, banner)
	fmt.Fprintf(s, "%s\n         a post-apocalyptic strategy game%s\n\n", ansi.FgCyan, ansi.Reset)
	fmt.Fprintf(s, "%sQuick tips:%s\n", ansi.FgBrightYellow, ansi.Reset)
	fmt.Fprintf(s, "%s  - At number prompts: type > for the max, or use k/m for thousands/millions.\n", ansi.FgWhite)
	fmt.Fprintf(s, "  - Press ? at a menu for help; * opens the System Menu.\n")
	fmt.Fprintf(s, "  - Your empire is saved between visits.%s\n", ansi.Reset)
	fmt.Fprintf(s, "\n%sPress any key to continue...%s", ansi.FgWhite, ansi.Reset)
	s.ReadKey()
}

const banner = `
  +--------------------------------------------------+
  |   ___ __  __ __  __  ___  ___ _____ _   _        |
  |  |_ _|  \/  |  \/  |/ _ \| _ \_   _/_\ | |       |
  |   | || |\/| | |\/| | (_) |   / | |/ _ \| |__     |
  |  |___|_|  |_|_|  |_|\___/|_|_\ |_/_/ \_\____|    |
  |                                                    |
  |    B A R O N S   O F   T H E   W A S T E S        |
  +--------------------------------------------------+
`
