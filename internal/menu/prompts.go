package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prompts.go — the yes/no question. It lives apart from input.go because
// AskYesNo is one of the few things internal/play needs from this package,
// for the onboarding that runs before the menu engine does.

// AskYesNo prompts msg with a "(Y/n)" or "(y/N)" hint (whichever matches
// defYes) and reads a single keypress — no Enter required. 'y'/'Y' returns
// true, 'n'/'N' returns false; Enter or any other key returns defYes.
func AskYesNo(s session.Session, msg string, defYes bool) bool {
	fmt.Fprint(s, "\n")
	return askYesNoHere(s, msg, defYes)
}

// askYesNoHere is AskYesNo without the leading newline, for a prompt BRE puts at
// the end of a line it has already started — the treaty-offer stats line ends
// "…; Score: N; Accept? (Y/n)".
func askYesNoHere(s session.Session, msg string, defYes bool) bool {
	letters := "y/N"
	if defYes {
		letters = "Y/n"
	}
	// BRE colors the y/n hint: the letters cyan, the parens a slightly darker blue.
	hint := ansi.FgBrightBlue + "(" + ansi.FgBrightCyan + letters + ansi.FgBrightBlue + ")" + ansi.Reset
	fmt.Fprintf(s, "%s %s ", i18n.T(sessionLang(s), msg), hint)
	for {
		r, err := readKey(s)
		if err != nil {
			return false // test stream ran out
		}
		drainInput(s) // drop a trailing Enter typed with the single-key answer
		switch r {
		case 'y', 'Y':
			fmt.Fprint(s, "y\n")
			return true
		case 'n', 'N':
			fmt.Fprint(s, "n\n")
			return false
		default:
			if defYes {
				fmt.Fprint(s, "y\n")
			} else {
				fmt.Fprint(s, "n\n")
			}
			return defYes
		}
	}
}
