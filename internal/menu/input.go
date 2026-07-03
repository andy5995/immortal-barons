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
	var b []rune
	for {
		r, err := s.ReadKey()
		if err != nil {
			break
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(s, "\n")
			return string(b)
		case 127, 8: // DEL / backspace
			if len(b) > 0 {
				b = b[:len(b)-1]
				fmt.Fprint(s, "\b \b")
			}
		default:
			if r >= 32 {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
	}
	return string(b)
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
