package menu

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/numfmt"
	"github.com/andy5995/immortal-barons/internal/session"
)

// readKey reads one key, propagating an idle/disconnect boot (ErrSessionEnded)
// via session.End so the whole session unwinds — nested sub-prompts, submenus,
// and the turn flow — instead of a prompt swallowing the boot as a cancel or
// returning up only one level. Swallowing it leaves the session limping on to
// the next guarded read (a false "Disconnected", then a second idle cycle). A
// bare io.EOF (a test stream ending) is returned for the caller to handle.
func readKey(s session.Session) (rune, error) {
	for {
		r, err := s.ReadKey()
		if errors.Is(err, session.ErrSessionEnded) {
			session.End(err)
		}
		if err != nil {
			return r, err
		}
		if r == 0x1b { // ESC: an arrow/PgUp/PgDn/Home/function-key sequence — swallow
			consumeEscape(s) // and read the next real key, so its bytes don't leak
			continue         // into a menu selection or a numeric/y-n prompt
		}
		return r, nil
	}
}

// drainInput drops a line terminator left buffered after a single-key answer,
// so a caller who types "n" then Enter (a telnet line-mode client sends both in
// one burst) does not have the Enter carry into the next single-key prompt as
// its default. A no-op for sessions with no input buffer (web, test fakes).
func drainInput(s session.Session) { session.Drain(s) }

// consumeEscape drains the rest of a terminal escape sequence after an ESC:
// a CSI "ESC [ … final" (arrows ABCD, PgUp/PgDn/Home/End as "…~", etc.) or an
// SS3 "ESC O x" (arrows in application mode). Best-effort; a stream error just
// stops (the caller's next read surfaces it). A lone ESC keypress has no trailing
// bytes, so this consumes the following keystroke — an accepted trade-off, since
// arrow/navigation keys always arrive as a full burst and a bare ESC is rare in
// menu navigation.
func consumeEscape(s session.Session) {
	r, err := s.ReadKey()
	if err != nil {
		return
	}
	switch r {
	case '[': // CSI: read until a final byte in 0x40–0x7E ('A'..'D', '~', 'H', 'F', …)
		for {
			c, err := s.ReadKey()
			if err != nil || (c >= 0x40 && c <= 0x7e) {
				return
			}
		}
	case 'O': // SS3: exactly one more byte
		s.ReadKey()
	}
}

// Navigation keys returned by readNavKey.
const (
	keyRune  = iota // r holds the pressed rune
	keyUp           // up arrow
	keyDown         // down arrow
	keyEnter        // Enter / Return
	keyOther        // an escape sequence we don't act on (left/right/PgUp/…)
)

// readNavKey reads one key for a lightbar. Unlike readKey (which swallows arrow
// escape sequences so their bytes can't leak into a selection), it decodes them
// and returns keyUp/keyDown; keyEnter for Return; or keyRune with the rune for
// anything printable. A session boot still unwinds via session.End.
func readNavKey(s session.Session) (kind int, r rune, err error) {
	c, err := s.ReadKey()
	if errors.Is(err, session.ErrSessionEnded) {
		session.End(err)
	}
	if err != nil {
		return keyOther, 0, err
	}
	switch c {
	case 0x1b:
		return readEscapeNav(s), 0, nil
	case '\r', '\n':
		return keyEnter, 0, nil
	}
	return keyRune, c, nil
}

// readEscapeNav decodes the tail of an escape sequence after an ESC — the CSI
// "ESC [ … A/B" or SS3 "ESC O A/B" arrow forms — returning keyUp/keyDown for the
// up/down arrows and keyOther for anything else. Mirrors consumeEscape's parsing
// but keeps the final byte instead of discarding it.
func readEscapeNav(s session.Session) int {
	c, err := s.ReadKey()
	if err != nil {
		return keyOther
	}
	var final rune
	switch c {
	case '[': // CSI: read to the final byte in 0x40–0x7E
		for {
			b, err := s.ReadKey()
			if err != nil {
				return keyOther
			}
			if b >= 0x40 && b <= 0x7e {
				final = b
				break
			}
		}
	case 'O': // SS3: exactly one more byte
		b, err := s.ReadKey()
		if err != nil {
			return keyOther
		}
		final = b
	default:
		return keyOther // a lone ESC consumed the next key; act on nothing
	}
	switch final {
	case 'A':
		return keyUp
	case 'B':
		return keyDown
	}
	return keyOther
}

// prompt writes msg and reads a line of input (terminated by Enter),
// echoing keystrokes since the console runs in no-echo mode.
func prompt(s session.Session, msg string) string {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), ansi.Reset)
	line, err := session.ReadLine(s)
	if err != nil {
		// ANY read failure ends the session, not just ErrSessionEnded. Letting a
		// bare io.EOF fall through returns "" to a caller that usually loops on
		// input it cannot parse, and the loop then spins on an input stream that
		// will never produce another byte: `-reset` with stdin closed redrew the
		// Configuration Editor 300,616 times and wrote 262 MB in five seconds
		// before anything killed it. AskRealmName has always ended on any error;
		// this makes the general prompts agree with it.
		session.End(err)
	}
	return line
}

// AskRealmName runs the realm-naming screen — the one place a realm is named,
// at onboarding and again on a rename (System > Preferences), so both ask,
// reject, and confirm in the same words. It re-asks until the name is valid and
// free and the player has confirmed it, and returns gaveUp when they backed out.
//
// lead prints before the prompt: the caller's handle at onboarding ("Bobby, "),
// nothing on a rename. taken reports whether a name is already spoken for —
// the caller supplies it because the two callers read different state (a
// snapshot taken under the world lock at onboarding, the live roster on a
// rename). giveUp is what an empty line means: onboarding asks "Quit?" there
// and ends the session, a rename simply cancels.
func AskRealmName(s session.Session, lang, lead string, taken func(string) bool, giveUp func() bool) (name string, gaveUp bool) {
	for {
		fmt.Fprintf(s, "\n%s%s%s%s ", ansi.FgBrightWhite, lead, i18n.T(lang, "Name your Realm:"), ansi.Reset)
		line, err := session.ReadLine(s)
		if err != nil {
			// The read ended the session — an idle/time boot, or the caller hung up.
			// Unwind it rather than carrying on with whatever was typed so far.
			// ReadLine echoes the newline on Enter but not here, so close the line
			// first or the boot notice lands on top of the prompt.
			fmt.Fprint(s, "\n")
			session.End(err)
		}
		name = strings.TrimSpace(line)
		if name == "" {
			if giveUp() {
				return "", true
			}
			continue
		}
		if !game.ValidRealmName(name) || taken(name) {
			// Say WHICH rule was missed. The three are easy to confuse, and a
			// player framing a name in CP437 glyphs would otherwise be told
			// nothing about why a name they can see on screen is refused.
			why := i18n.T(lang, "Invalid: a realm name needs at least 3 letters or digits. Decoration around them is fine and does not count.")
			switch {
			case len([]rune(name)) > game.RealmNameMaxChars:
				why = fmt.Sprintf(i18n.T(lang, "Too long: a realm name may be at most %d characters."), game.RealmNameMaxChars)
			case taken(name):
				why = i18n.T(lang, "Taken: another realm already answers to that name.")
			}
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed, WrapIndented(why, "  "), ansi.Reset)
			continue
		}
		// Confirm before committing: a typo is easy to make, and the name is
		// permanent — a realm is named once and renamed at most once. Declining
		// re-prompts for a different one.
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan,
			WrapIndented(fmt.Sprintf(i18n.T(lang, "Your realm will be named %s."), name), ""), ansi.Reset)
		if !AskYesNo(s, "Confirm?", true) {
			continue
		}
		return name, false
	}
}

// parseAmount interprets a numeric input with BRE-style shortcuts:
//
//	">"        -> max
//	"<n>b"     -> n * 1_000_000_000
//	"<n>m"     -> n * 1_000_000
//	"<n>k"     -> n * 1_000
//	"<n>"      -> n
//
// Empty or unparseable -> 0. A magnitude suffix on ">" is ignored. The whole
// parse runs in int64 so a nine- or ten-digit entry survives a 32-bit build.
func parseAmount[T numfmt.Number](input string, max T) T {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == ">" {
		return max
	}
	var mult int64 = 1
	switch {
	case strings.HasSuffix(s, "b"):
		mult, s = 1_000_000_000, strings.TrimSuffix(s, "b")
	case strings.HasSuffix(s, "m"):
		mult, s = 1_000_000, strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		mult, s = 1_000, strings.TrimSuffix(s, "k")
	}
	s = strings.TrimSpace(s)
	if s == ">" {
		return max
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return T(n * mult)
}

func promptInt(s session.Session, msg string) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, i18n.T(sessionLang(s), msg), ansi.Reset)
	line, err := session.ReadLine(s)
	if err != nil {
		session.End(err) // see prompt: a swallowed EOF spins the caller's loop
	}
	return parseAmount(line, math.MaxInt)
}

// promptSuggested shows "<msg> (<suggested>; <max>): " and returns a value.
// Empty input returns suggested; typing `>` first fills the field with max
// (still editable); otherwise the typed number (k/m shortcuts) is used.
// The result is clamped to [0, max].
func promptSuggested[T numfmt.Number](s session.Session, msg string, suggested, max T) T {
	fmt.Fprint(s, "\n")
	return promptSuggestedTight(s, msg, suggested, max)
}

// promptSuggestedTight is promptSuggested without the leading blank line, for a
// run of prompts that should sit on consecutive lines — the attack force
// selection (Troopers/Jets/Tanks/Bombers) after a single header line.
func promptSuggestedTight[T numfmt.Number](s session.Session, msg string, suggested, max T) T {
	// BRE colors the "(suggested; max)": bright-blue parens/semicolon, bright-cyan
	// suggested, dark-cyan max (the same blue/cyan scheme as the y/n hint).
	prefix := fmt.Sprintf("%s%s %s(%s%s%s; %s%s%s)%s:%s ",
		ansi.FgWhite, i18n.T(sessionLang(s), msg),
		ansi.FgBrightBlue, ansi.FgBrightCyan, comma(suggested), ansi.FgBrightBlue,
		ansi.FgCyan, comma(max), ansi.FgBrightBlue,
		ansi.FgWhite, ansi.Reset)
	fmt.Fprint(s, prefix)
	return editAmount(s, prefix, suggested, max)
}

// promptSuggestedOpen is promptSuggested with the ceiling left OFF the screen,
// for a prompt whose bound is a sentinel rather than a fact about the game. The
// value is still clamped to [0, max].
//
// The trade deal's Request side is the case: BRE bounds "Demand how many <item>?"
// at 0xFFFFFFFF (create_trade_offer, BRE.OVR 0x26a4b — against the player's own
// holdings on the Send side at 0x2679d) and prints no hint at all, because there
// is no honest number to print. What the other realm holds is not yours to see.
// IB printed the sentinel itself, "(0; 1.0737B)", which reads like information
// and is not.
func promptSuggestedOpen[T numfmt.Number](s session.Session, msg string, suggested, max T) T {
	prefix := fmt.Sprintf("\n%s%s %s(%s%s%s)%s:%s ",
		ansi.FgWhite, i18n.T(sessionLang(s), msg),
		ansi.FgBrightBlue, ansi.FgBrightCyan, comma(suggested), ansi.FgBrightBlue,
		ansi.FgWhite, ansi.Reset)
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
	// Same blue/cyan "(suggested; max)" scheme BRE uses (docs/dev/bre-screens.md),
	// matching promptSuggestedTight; the %3d keeps the input column aligned.
	prefix := fmt.Sprintf("%s%s%s %s(%s%3d%s; %s%3d%s)%s:%s ",
		ansi.FgWhite, label, strings.Repeat(" ", pad),
		ansi.FgBrightBlue, ansi.FgBrightCyan, suggested, ansi.FgBrightBlue,
		ansi.FgCyan, max, ansi.FgBrightBlue,
		ansi.FgWhite, ansi.Reset)
	fmt.Fprint(s, prefix)
	return editAmount(s, prefix, suggested, max)
}

// editAmount runs the number-editing loop after `prefix` has already been
// printed: >, k/m expansion, backspace, and Enter. Empty Enter keeps the
// suggested value AND echoes it on the line (BRE prints the kept number), so the
// player sees what a bare Enter chose. An over-max entry is clamp-and-confirmed
// (#9): the first Enter corrects the field to the max on screen and a second Enter
// commits it — never a silent one-keystroke clamp.
func editAmount[T numfmt.Number](s session.Session, prefix string, suggested, max T) T {
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
				str := strconv.FormatInt(int64(max), 10)
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
			str := strconv.FormatInt(int64(max), 10)
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
		case r == 'b' || r == 'B': // expand in place: 1 b -> 1000000000
			b = append(b, '0', '0', '0', '0', '0', '0', '0', '0', '0')
			fmt.Fprint(s, "000000000")
		case r == 127 || r == 8: // backspace
			if len(b) > 0 {
				b = b[:len(b)-1]
				fmt.Fprint(s, "\b \b")
			}
		default:
			// A numeric field takes only digits (its >/k/m/backspace shortcuts are
			// handled above). Silently ignore any other rune so a stray printable byte
			// — a nav/function key in DOORWAY mode, a non-CSI escape our readKey does
			// not drain, a paste — can't echo into the field (as the "→→" a caller saw)
			// or corrupt the parsed amount.
			if r >= '0' && r <= '9' {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
	}
	return clampAmt(suggested, max)
}

func clampAmt[T numfmt.Number](n, max T) T {
	if n < 0 {
		return 0
	}
	if n > max {
		return max
	}
	return n
}

// ChoiceQuit prints the standard menu prompt ("> Quit", with "Quit" shown after
// the ">" as the Enter default) for a custom numbered list, so those lists match
// the menu engine's readChoice. It reads one key and returns the chosen number
// 1..max, or 0 to quit (Enter, '0', or any non-matching key).
func ChoiceQuit(s session.Session, max int) int {
	quit := i18n.T(sessionLang(s), "Quit")
	fmt.Fprintf(s, "\n%s>%s %s", ansi.FgBrightWhite, ansi.Reset, quit)
	r, err := readKey(s)
	if err != nil {
		return 0
	}
	drainInput(s)                           // drop a trailing Enter sent in one burst with the selection key
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

// choiceQuitOrHelp is ChoiceQuit for a list that also offers a '?' Help item,
// as BRE's attack-type menu does. Returns the chosen number 1..max (0 to quit),
// and help=true when '?' was pressed, so the caller can render a topic and draw
// the menu again.
func choiceQuitOrHelp(s session.Session, max int) (n int, help bool) {
	quit := i18n.T(sessionLang(s), "Quit")
	// The bare ">" is the project's prompt, deliberately replacing BRE's
	// "Choice>" because it needs no translation (see readChoice in menu.go).
	fmt.Fprintf(s, "\n%s>%s %s", ansi.FgBrightWhite, ansi.Reset, quit)
	r, err := readKey(s)
	if err != nil {
		return 0, false
	}
	drainInput(s)
	if r == '\r' || r == '\n' || r == '0' {
		fmt.Fprint(s, "\n")
		return 0, false
	}
	for range []rune(quit) { // a real choice: erase the shown default first
		fmt.Fprint(s, "\b \b")
	}
	if r == '?' {
		fmt.Fprint(s, "?\n")
		return 0, true
	}
	c := int(r - '0')
	if r < '1' || r > '9' || c > max {
		fmt.Fprint(s, "\n")
		return 0, false
	}
	fmt.Fprintf(s, "%d\n", c)
	return c, false
}

// pauseTight is pause with no blank line before the prompt, for a screen that
// already ends on its own line — BRE puts its "Paused" bar directly under the
// splash's last line, not a row below it.
func pauseTight(s session.Session) {
	fmt.Fprintf(s, "%s%s%s", ansi.FgBrightCyan, i18n.T(sessionLang(s), "─»>Paused<«─"), ansi.Reset)
	readKey(s)
	drainInput(s)
	endPausedLine(s)
}

func pause(s session.Session) {
	// A boot/disconnect during the pause must unwind (readKey handles it); a
	// bare io.EOF (test stream) falls through.
	fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightCyan, i18n.T(sessionLang(s), "─»>Paused<«─"), ansi.Reset)
	readKey(s)
	drainInput(s) // drop a trailing Enter sent in one burst with the dismissing key
	endPausedLine(s)
}

// endPausedLine closes the line the Paused bar sits on, once the key that
// dismisses it has arrived. Without it the cursor is left at the end of the bar
// and whatever draws next continues that line — a menu opens with its title rule
// and no newline of its own, so the two ran together as
// "─»>Paused<«──────[System]──────".
func endPausedLine(s session.Session) { fmt.Fprint(s, "\n") }

// statLine prints one BRE-style result line — a highlighted, comma-formatted
// number followed by text — and skips the line entirely when n is zero (BRE
// only lists results that actually happened).
func statLine[T numfmt.Number](s session.Session, n T, text string) {
	if n == 0 {
		return
	}
	fmt.Fprintf(s, "  %s%s%s %s\n", ansi.FgBrightCyan, comma(n), ansi.Reset, i18n.T(sessionLang(s), text))
}

// WrapIndented breaks text at word boundaries to the screen width, indenting
// every line by indent. Without it a message longer than the terminal is broken
// by the terminal itself, mid-word and flush against the left margin — and a
// translated string is often half again as long as the English it came from
// (a German line ran off the margin and split "Reich" as "Re"/"ich").
//
// Exported for internal/play, whose onboarding prompts run before the menu
// engine exists and print straight to the session.
func WrapIndented(text, indent string) string { return wrapHanging(text, indent, indent) }

// wrapReport word-wraps a multi-line report, keeping the line breaks it already
// has. help.Wrap collapses all whitespace, newlines included, so passing a whole
// combat or raid report through it would run every line into one paragraph —
// each line has to be wrapped on its own.
//
// Wrap BEFORE colouring. hiNums and friends insert escape sequences that no
// terminal displays but every length count sees, so a report wrapped afterwards
// breaks far short of the margin.
func wrapReport(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue // a blank line is the report's own spacing
		}
		lines[i] = help.Wrap(l, ansi.ScreenCols-1)
	}
	return strings.Join(lines, "\n")
}

// wrapHanging is WrapIndented with a different lead on the first line, so a
// marker can sit outside the text and the continuation still lines up under it.
func wrapHanging(text, first, cont string) string {
	lines := strings.Split(help.Wrap(text, ansi.ScreenCols-len(cont)-1), "\n")
	for i, l := range lines {
		if i == 0 {
			lines[i] = first + l
			continue
		}
		lines[i] = cont + l
	}
	return strings.Join(lines, "\n")
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
	msg := fmt.Sprintf(i18n.T(sessionLang(s), format), a...)
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightWhite, WrapIndented(msg, "  "), ansi.Reset)
}

func fail(s session.Session, err error) {
	// The "!" is what tells a failure from a success on a terminal that shows no
	// colour, or to a reader who cannot tell this red from the white above it.
	// ok() prints the same shape without it.
	fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightRed, wrapHanging(i18n.T(sessionLang(s), err.Error()), "  ! ", "    "), ansi.Reset)
	pause(s)
}
