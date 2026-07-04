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

// promptSuggested shows "<msg> (<suggested>; <max>): " and returns a value.
// Empty input returns suggested; typing `>` first fills the field with max
// (still editable); otherwise the typed number (k/m shortcuts) is used.
// The result is clamped to [0, max].
func promptSuggested(s session.Session, msg string, suggested, max int) int {
	fmt.Fprintf(s, "\n%s%s (%d; %d):%s ", ansi.FgBrightWhite, msg, suggested, max, ansi.Reset)
	var b []rune
	for {
		r, err := s.ReadKey()
		if err != nil {
			break
		}
		switch {
		case r == '\r' || r == '\n':
			fmt.Fprint(s, "\n")
			if len(b) == 0 {
				return clampAmt(suggested, max)
			}
			return clampAmt(parseAmount(string(b), max), max)
		case r == '>' && len(b) == 0:
			str := strconv.Itoa(max)
			for _, c := range str {
				b = append(b, c)
			}
			fmt.Fprint(s, str) // echo the prefilled max, still editable
		case r == 'k' || r == 'K': // expand in place: 1 k -> 1000
			b = append(b, '0', '0', '0')
			fmt.Fprint(s, "000")
		case r == 'm' || r == 'M':
			b = append(b, '0', '0', '0', '0', '0', '0')
			fmt.Fprint(s, "000000")
		case r == 127 || r == 8: // backspace
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
	return clampAmt(suggested, max)
}

func clampAmt(n, max int) int {
	if n < 0 {
		return 0
	}
	if n > max {
		return max
	}
	return n
}

func pause(s session.Session) {
	// BRE's pause prompt.
	fmt.Fprintf(s, "\n%s─»>Paused<«─%s", ansi.FgBrightCyan, ansi.Reset)
	s.ReadKey()
}

// comma formats n with thousands separators (478967 -> "478,967").
func comma(n int) string {
	str := strconv.Itoa(n)
	neg := n < 0
	if neg {
		str = str[1:]
	}
	var b strings.Builder
	for i := 0; i < len(str); i++ {
		if i > 0 && (len(str)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(str[i])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// statLine prints one BRE-style result line — a highlighted, comma-formatted
// number followed by text — and skips the line entirely when n is zero (BRE
// only lists results that actually happened).
func statLine(s session.Session, n int, text string) {
	if n == 0 {
		return
	}
	fmt.Fprintf(s, "  %s%s%s %s\n", ansi.FgBrightCyan, comma(n), ansi.Reset, text)
}

func ok(s session.Session, format string, a ...any) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgGreen, fmt.Sprintf(format, a...), ansi.Reset)
	pause(s)
}

func fail(s session.Session, err error) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgRed, err, ansi.Reset)
	pause(s)
}
