package menu

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// prompt writes msg and reads a line of input (terminated by Enter),
// echoing keystrokes since the console runs in no-echo mode.
func prompt(s session.Session, msg string) string {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), ansi.Reset)
	line, err := session.ReadLine(s)
	if errors.Is(err, session.ErrSessionEnded) {
		session.End(err) // idle boot / disconnect: unwind (a bare io.EOF falls through)
	}
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
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), ansi.Reset)
	line, err := session.ReadLine(s)
	if errors.Is(err, session.ErrSessionEnded) {
		session.End(err)
	}
	return parseAmount(line, math.MaxInt)
}

// promptSuggested shows "<msg> (<suggested>; <max>): " and returns a value.
// Empty input returns suggested; typing `>` first fills the field with max
// (still editable); otherwise the typed number (k/m shortcuts) is used.
// The result is clamped to [0, max].
func promptSuggested(s session.Session, msg string, suggested, max int) int {
	prefix := fmt.Sprintf("%s%s (%d; %d):%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), suggested, max, ansi.Reset)
	fmt.Fprint(s, "\n"+prefix)

	// Register the line being edited so an interrupting idle/time warning can
	// reprint it and restore the cursor. A no-op for sessions with no Deadline
	// (e.g. the test fakeSession), which don't implement InputLineSetter.
	ls, _ := s.(session.InputLineSetter)
	if ls != nil {
		defer ls.SetInputLine("")
	}

	var b []rune
	for {
		if ls != nil {
			ls.SetInputLine(prefix + string(b))
		}
		r, err := s.ReadKey()
		if err != nil {
			if errors.Is(err, session.ErrSessionEnded) {
				session.End(err) // idle boot / disconnect: unwind the whole turn
			}
			break // test stream ran out: fall back to the suggested value
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

// choiceQuit prints the standard menu prompt ("Choice> Quit", with "Quit" shown
// after the prompt as the Enter default) for a custom numbered list, so those
// lists match the menu engine's readChoice. It reads one key and returns the
// chosen number 1..max, or 0 to quit (Enter, '0', or any non-matching key).
func choiceQuit(s session.Session, max int) int {
	lang := sessionLang(s)
	quit := i18n.T(lang, "Quit")
	fmt.Fprintf(s, "\n%s%s%s %s", ansi.FgBrightWhite, i18n.T(lang, "Choice>"), ansi.Reset, quit)
	r, err := s.ReadKey()
	if err != nil {
		if errors.Is(err, session.ErrSessionEnded) {
			session.End(err)
		}
		return 0
	}
	if r == '\r' || r == '\n' || r == '0' { // Enter/0 selects the shown Quit
		fmt.Fprint(s, "\n")
		return 0
	}
	for range []rune(quit) { // a real choice: erase the shown default first
		fmt.Fprint(s, "\b \b")
	}
	n := int(r - '0')
	if r < '1' || r > '9' || n > max {
		fmt.Fprint(s, "\n")
		return 0
	}
	fmt.Fprintf(s, "%d\n", n) // echo the choice
	return n
}

func pause(s session.Session) {
	// BRE's pause prompt. A boot/disconnect here (ErrSessionEnded) must unwind:
	// otherwise the flow falls through, redraws the menu, and only boots again
	// on the next read — a double "Disconnected" with a delay. A bare io.EOF
	// (test stream) still falls through.
	fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightCyan, i18n.T(sessionLang(s), "─»>Paused<«─"), ansi.Reset)
	if _, err := s.ReadKey(); errors.Is(err, session.ErrSessionEnded) {
		session.End(err) // boot during a pause: unwind instead of falling through
	}
}

// groupSep maps a UI language to its thousands separator. All three are ASCII,
// so they are CP437-safe (and CP437 mode forces English anyway). Unknown
// languages fall back to the comma.
var groupSep = map[string]byte{"": ',', "en": ',', "de": '.', "ru": ' '}

// formatGold groups n's thousands with lang's locale separator
// (en 1,847,392,104 / de 1.847.392.104 / ru "1 847 392 104").
func formatGold(n int, lang string) string {
	sep, ok := groupSep[lang]
	if !ok {
		sep = ','
	}
	str := strconv.Itoa(n)
	neg := n < 0
	if neg {
		str = str[1:]
	}
	var b strings.Builder
	for i := 0; i < len(str); i++ {
		if i > 0 && (len(str)-i)%3 == 0 {
			b.WriteByte(sep)
		}
		b.WriteByte(str[i])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// comma formats n with English thousands separators (478967 -> "478,967").
func comma(n int) string { return formatGold(n, "") }

// abbrevMoney formats large totals with a k/m suffix (34833289 -> "34,833k",
// 1373000000 -> "1,373m") so a planet-wide total fits on one line. The switch
// to "m" only kicks in at a billion, since gold/net-worth totals commonly run
// into the tens of millions (still readable as "k") per the money caps in
// docs/mechanics-reference.md. Below 1,000 it falls back to comma().
func abbrevMoney(n int) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1_000_000_000:
		return comma(n/1_000_000) + "m"
	case abs >= 1_000:
		return comma(n/1_000) + "k"
	default:
		return comma(n)
	}
}

// statLine prints one BRE-style result line — a highlighted, comma-formatted
// number followed by text — and skips the line entirely when n is zero (BRE
// only lists results that actually happened).
func statLine(s session.Session, n int, text string) {
	if n == 0 {
		return
	}
	fmt.Fprintf(s, "  %s%s%s %s\n", ansi.FgBrightCyan, comma(n), ansi.Reset, i18n.T(sessionLang(s), text))
}

// ok and fail translate their message text by the caller's language, so every
// call site becomes translatable just by adding the string to the catalogs.
func ok(s session.Session, format string, a ...any) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgGreen, fmt.Sprintf(i18n.T(sessionLang(s), format), a...), ansi.Reset)
	pause(s)
}

func fail(s session.Session, err error) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgRed, i18n.T(sessionLang(s), err.Error()), ansi.Reset)
	pause(s)
}
