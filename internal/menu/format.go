package menu

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// format.go holds how a figure REACHES the screen: the menu-side names for
// internal/numfmt, and the highlighting that makes a number stand out of a line.
//
// The number-rendering rules live in internal/numfmt so the game engine — which
// writes player-visible event text and cannot import this package — formats a
// figure exactly the way these screens do. These are the menu-side names.
//
// The highlighters below used to live in actions_attack.go, beside the combat
// reports they were first written for; a dozen other screens call them, so they
// belong with the rest of the display layer.

// formatGold renders n in full with lang's thousands separator, however large.
func formatGold[T numfmt.Number](n T, lang string) string { return numfmt.Format(n, lang) }

// comma formats n with English thousands separators.
func comma[T numfmt.Number](n T) string { return numfmt.Comma(n) }

// hiNumsReset returns s with each run of digits (keeping grouping commas that
// sit between digits) wrapped in numColor and then restored to resetColor, so
// figures pop against a colored line — BRE highlights numbers throughout its
// screens (docs/dev/bre-screens.md). resetColor is what the text returns to after
// a number (ansi.Reset for plain lines, or the line's own base color when the
// surrounding text is itself colored, e.g. the white Technology advisor). The
// game builds reports as plain text (it stays display-agnostic), so the coloring
// happens here at the display layer.
func hiNumsReset(s, numColor, resetColor string) string {
	var b strings.Builder
	inNum := false
	for i := 0; i < len(s); {
		// Pass an escape sequence through untouched — its digits (the "96" in
		// ESC[96m) are not figures, and wrapping them in a color code produces a
		// malformed sequence the terminal prints as literal text.
		if n := csiLen(s[i:]); n > 0 {
			if inNum {
				b.WriteString(resetColor)
				inNum = false
			}
			b.WriteString(s[i : i+n])
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		digit := r >= '0' && r <= '9'
		if inNum && r == ',' {
			b.WriteRune(r) // 1,000 — a comma between digits stays inside the run
			continue
		}
		if digit && !inNum {
			b.WriteString(numColor)
			inNum = true
		} else if !digit && inNum {
			b.WriteString(resetColor)
			inNum = false
		}
		b.WriteRune(r)
	}
	if inNum {
		b.WriteString(resetColor)
	}
	return b.String()
}

// csiLen returns the byte length of the ANSI CSI sequence at the head of s, or
// 0 if s does not start with one.
func csiLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	for i := 2; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
	}
	return 0
}

// hiNumsIn highlights figures in the given color on an otherwise-plain line.
func hiNumsIn(s, color string) string { return hiNumsReset(s, color, ansi.Reset) }

// hiNums highlights figures in bright-yellow — BRE's default figure color for
// battle/raid/economy reports.
func hiNums(s string) string { return hiNumsIn(s, ansi.FgBrightYellow) }

// hiTokens paints whole-word occurrences of any of words in color on an
// otherwise-plain line, then back to ansi.Reset — the same shape as hiNums,
// but for known literal words instead of digit runs. Longest word first, so a
// short token that is a substring of a longer one (an empire named "Trade"
// against "Free Trade Agreement") can't steal the longer token's match; \b
// keeps a short token from painting itself inside an unrelated word.
func hiTokens(s string, words []string, color string) string {
	if len(words) == 0 {
		return s
	}
	sorted := append([]string(nil), words...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	quoted := make([]string, len(sorted))
	for i, w := range sorted {
		quoted[i] = regexp.QuoteMeta(w)
	}
	re := regexp.MustCompile(`\b(?:` + strings.Join(quoted, "|") + `)\b`)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		return color + m + ansi.Reset
	})
}
