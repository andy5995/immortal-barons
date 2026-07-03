package menu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prompt writes msg and reads a line of input (terminated by Enter),
// echoing keystrokes since the console runs in no-echo mode.
func prompt(s session.Session, msg string) string {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, msg, ansi.Reset)
	line, _ := session.ReadLine(s)
	return line
}

func promptInt(s session.Session, msg string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(prompt(s, msg)))
	return n
}

func pause(s session.Session) {
	fmt.Fprintf(s, "\n%sPress any key...%s", ansi.FgWhite, ansi.Reset)
	s.ReadKey()
}

func ok(s session.Session, format string, a ...any) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgGreen, fmt.Sprintf(format, a...), ansi.Reset)
	pause(s)
}

func fail(s session.Session, err error) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgRed, err, ansi.Reset)
	pause(s)
}
