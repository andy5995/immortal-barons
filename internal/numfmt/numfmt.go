// Package numfmt renders large numbers for display. It lives on its own so the
// game engine and the menu layer share ONE implementation: the engine writes
// player-visible event text, the menu draws the screens, and a figure must read
// the same either way.
package numfmt

import (
	"strconv"
	"strings"
)

// Number is the numeric width the display helpers accept: plain counts (int)
// and money (int64). One generic body means a figure reads the same way whether
// it is gold, troopers, or regions.
type Number interface{ ~int | ~int64 }

// A figure is printed in full however large it grows, with its thousands
// grouped. That is what BRE does: `Bank: 2,000,000,000` and `Today
// $1,846,153,847` are its own screens at the top of its range, so ten digits
// need no abbreviating and nothing here needs a float. IB rendered a billion
// and over as a fixed 4-decimal "1.8473B" until v0.0.8; the one place BRE
// itself printed that form was a numeric prompt's ceiling, which IB no longer
// shows either.

// groupSep maps a UI language to its thousands separator. All three are ASCII,
// so they are CP437-safe (and CP437 mode forces English anyway). Unknown
// languages fall back to the comma.
var groupSep = map[string]byte{"": ',', "en": ',', "de": '.', "ru": ' '}

// Format renders n for display with lang's thousands separator: en 1,847,392 /
// de 1.847.392 / ru "1 847 392".
//
// It is not gold-specific despite the name — any figure that can grow past a
// screen column's width should go through it.
func Format[T Number](n T, lang string) string {
	sep, ok := groupSep[lang]
	if !ok {
		sep = ','
	}
	v := int64(n)
	if v < 0 {
		return "-" + groupDigits(-v, sep)
	}
	return groupDigits(v, sep)
}

// groupDigits writes a non-negative value with sep every three digits.
func groupDigits(v int64, sep byte) string {
	str := strconv.FormatInt(v, 10)
	var b strings.Builder
	for i := 0; i < len(str); i++ {
		if i > 0 && (len(str)-i)%3 == 0 {
			b.WriteByte(sep)
		}
		b.WriteByte(str[i])
	}
	return b.String()
}

// Comma formats n with English thousands separators (478967 -> "478,967").
func Comma[T Number](n T) string { return Format(n, "") }

// Abbrev formats a large total compactly so it fits one column, stepping the
// suffix with the magnitude: k at a thousand, m at a million, b at a billion,
// and nothing at all below a thousand. The fraction is truncated, not rounded,
// so a figure a hair under the next step never reads as having reached it.
//
// BRE does NOT step: its own columns pick ONE suffix and keep it however far the
// figure runs, which is why a capture holds `Total Net Worth: 25,750k` on the
// Daily Bulletin and `1962k   12m` side by side in one See Scores row — k and m
// on the same screen at the same magnitude, differing by column rather than by
// size. Stepping is IB's choice (Andy's, #205): one rule everywhere beats two
// columns that disagree, and it keeps any figure inside four digits and a
// letter.
func Abbrev[T Number](n T) string {
	abs := int64(n)
	if abs < 0 {
		abs = -abs
	}
	for _, step := range abbrevSteps {
		if abs >= step.at {
			return Comma(int64(n)/step.at) + step.suffix
		}
	}
	return Comma(n)
}

// abbrevSteps runs largest first, so the first match is the right one.
var abbrevSteps = []struct {
	at     int64
	suffix string
}{
	{1_000_000_000, "b"},
	{1_000_000, "m"},
	{1_000, "k"},
}
