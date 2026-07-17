package menu

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// readKey reads one key, propagating an idle/disconnect boot (ErrSessionEnded)
// via session.End so the whole session unwinds — nested sub-prompts, submenus,
// and the turn flow — instead of a prompt swallowing the boot as a cancel or
// returning up only one level. Swallowing it leaves the session limping on to
// the next guarded read (a false "Disconnected", then a second idle cycle). A
// bare io.EOF (a test stream ending) is returned for the caller to handle.
func readKey(s session.Session) (rune, error) {
	r, err := s.ReadKey()
	if errors.Is(err, session.ErrSessionEnded) {
		session.End(err)
	}
	return r, err
}

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
	fmt.Fprint(s, "\n")
	return promptSuggestedTight(s, msg, suggested, max)
}

// promptSuggestedTight is promptSuggested without the leading blank line, for a
// run of prompts that should sit on consecutive lines — the attack force
// selection (Troopers/Jets/Tanks/Bombers) after a single header line.
func promptSuggestedTight(s session.Session, msg string, suggested, max int) int {
	prefix := fmt.Sprintf("%s%s (%d; %d):%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), suggested, max, ansi.Reset)
	fmt.Fprint(s, prefix)
	return editAmount(s, prefix, suggested, max)
}

// promptProduction is the Set Industries per-unit prompt (no leading blank line,
// so the units sit on consecutive lines). `label` is padded to labelW runes and
// the (suggested; max) numbers are right-aligned to 3 digits, so the input column
// lines up down the whole list. Enter keeps the suggested value and echoes it.
func promptProduction(s session.Session, label string, labelW, suggested, max int) int {
	pad := labelW - utf8.RuneCountInString(label)
	if pad < 0 {
		pad = 0
	}
	prefix := fmt.Sprintf("%s%s%s (%3d; %3d):%s ", ansi.FgBrightWhite, label, strings.Repeat(" ", pad), suggested, max, ansi.Reset)
	fmt.Fprint(s, prefix)
	return editAmount(s, prefix, suggested, max)
}

// editAmount runs the number-editing loop after `prefix` has already been
// printed: >, k/m expansion, backspace, and Enter. Empty Enter keeps the
// suggested value AND echoes it on the line (BRE prints the kept number), so the
// player sees what a bare Enter chose. An over-max entry is clamp-and-confirmed
// (#9): the first Enter corrects the field to the max on screen and a second Enter
// commits it — never a silent one-keystroke clamp.
func editAmount(s session.Session, prefix string, suggested, max int) int {
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
		r, err := readKey(s)
		if err != nil {
			break // test stream ran out: fall back to the suggested value
		}
		switch {
		case r == '\r' || r == '\n':
			if len(b) == 0 {
				v := clampAmt(suggested, max)
				fmt.Fprintf(s, "%d\n", v) // echo the kept value on the line
				return v
			}
			v := parseAmount(string(b), max)
			if v > max {
				// BRE clamp-and-confirm (#9): an over-max entry is corrected to the
				// max on screen and needs a SECOND Enter to commit — feedback and a
				// chance to reconsider, not a silent one-keystroke clamp. Rewrite the
				// field to the max in place and keep waiting.
				str := strconv.Itoa(max)
				for range b {
					fmt.Fprint(s, "\b \b")
				}
				fmt.Fprint(s, str)
				b = []rune(str)
				continue
			}
			fmt.Fprint(s, "\n")
			return clampAmt(v, max)
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
	r, err := readKey(s)
	if err != nil {
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
	// A boot/disconnect during the pause must unwind (readKey handles it); a
	// bare io.EOF (test stream) falls through.
	fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightCyan, i18n.T(sessionLang(s), "─»>Paused<«─"), ansi.Reset)
	readKey(s)
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
	okNoPause(s, format, a...)
	pause(s)
}

// okNoPause is ok without the trailing wait-for-keypress, for a success message
// that a following screen or prompt makes the pause redundant.
func okNoPause(s session.Session, format string, a ...any) {
	fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgBrightWhite, fmt.Sprintf(i18n.T(sessionLang(s), format), a...), ansi.Reset)
}

func fail(s session.Session, err error) {
	fmt.Fprintf(s, "\n  %s%s%s", ansi.FgBrightRed, i18n.T(sessionLang(s), err.Error()), ansi.Reset)
	pause(s)
}
