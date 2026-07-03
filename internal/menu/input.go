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

// parseAmount interprets a numeric input with BRE-style shortcuts:
//
//	">"        -> max
//	"<n>m"     -> n * 1_000_000
//	"<n>k"     -> n * 1_000
//	"<n>"      -> n
//
// Empty or unparseable -> 0. A magnitude suffix on ">" is ignored.
func parseAmount(input string, max int) int {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == ">" {
		return max
	}
	mult := 1
	if strings.HasSuffix(s, "m") {
		mult, s = 1_000_000, strings.TrimSuffix(s, "m")
	} else if strings.HasSuffix(s, "k") {
		mult, s = 1_000, strings.TrimSuffix(s, "k")
	}
	s = strings.TrimSpace(s)
	if s == ">" {
		return max
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n * mult
}

func promptInt(s session.Session, msg string) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, msg, ansi.Reset)
	line, _ := session.ReadLine(s)
	return parseAmount(line, 1<<62)
}

// promptAmount shows "msg (max: MAX) " and returns the parsed amount,
// supporting >, m, and k shortcuts. Values above max are clamped to max.
func promptAmount(s session.Session, msg string, max int) int {
	fmt.Fprintf(s, "\n%s%s (max %d, > = max):%s ", ansi.FgBrightWhite, msg, max, ansi.Reset)
	line, _ := session.ReadLine(s)
	n := parseAmount(line, max)
	if n > max {
		n = max
	}
	if n < 0 {
		n = 0
	}
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
