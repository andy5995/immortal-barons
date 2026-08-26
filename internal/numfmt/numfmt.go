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

// Abbrev formats large totals with a k suffix (34,833,289 -> "34,833k") so a
// planet-wide total fits on one line, as BRE's own Daily Bulletin does it
// (`Total Net Worth: 2720k`). The suffix carries on past a billion —
// 1,373,000,000 -> "1,373,000k" — keeping one form down the column. Below
// 1,000 it prints in full.
func Abbrev[T Number](n T) string {
	abs := int64(n)
	if abs < 0 {
		abs = -abs
	}
	if abs >= 1_000 {
		return Comma(int64(n)/1_000) + "k"
	}
	return Comma(n)
}
