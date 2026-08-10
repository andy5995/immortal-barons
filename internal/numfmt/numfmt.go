// Package numfmt renders large numbers for display. It lives on its own so the
// game engine and the menu layer share ONE implementation: the engine writes
// player-visible event text, the menu draws the screens, and a figure must read
// the same either way.
package numfmt

import (
	"fmt"
	"strconv"
	"strings"
)

// Number is the numeric width the display helpers accept: plain counts (int)
// and money (int64). One generic body means a figure reads the same way whether
// it is gold, troopers, or regions.
type Number interface{ ~int | ~int64 }

// Above a billion a figure stops being printed in full and switches to a
// fixed 4-decimal "B" form: 1,000,000,000 -> "1.0000B", 1,847,392,104 ->
// "1.8473B". Nothing in BRE ever reached ten digits, so its screens have no
// column wide enough for one; the treasury can now hold far more than that
// (the sysop's money cap), so every figure needs a form that still fits. The fraction
// is truncated, not rounded, so a total just under a billion-and-one never
// reads as one.
const (
	billion         = 1_000_000_000
	billionDecimals = 4
	billionDivisor  = 100_000 // 10^(9-4): scales the sub-billion remainder to 4 digits
)

// groupSep maps a UI language to its thousands separator. All three are ASCII,
// so they are CP437-safe (and CP437 mode forces English anyway). Unknown
// languages fall back to the comma.
var groupSep = map[string]byte{"": ',', "en": ',', "de": '.', "ru": ' '}

// decimalSep maps a UI language to its decimal mark, for the "B" form. German
// and Russian take the comma where English takes the point.
var decimalSep = map[string]byte{"": '.', "en": '.', "de": ',', "ru": ','}

// Format renders n for display: grouped thousands in lang's locale
// separator (en 1,847,392 / de 1.847.392 / ru "1 847 392"), or the abbreviated
// billions form once |n| reaches a billion (en 1.8473B / de 1,8473B).
//
// It is not gold-specific despite the name — any figure that can grow past a
// screen column's width should go through it.
func Format[T Number](n T, lang string) string {
	sep, ok := groupSep[lang]
	if !ok {
		sep = ','
	}
	v := int64(n)
	neg := v < 0
	if neg {
		v = -v
	}
	var out string
	if v >= billion {
		dec, ok := decimalSep[lang]
		if !ok {
			dec = '.'
		}
		out = fmt.Sprintf("%s%c%0*dB", groupDigits(v/billion, sep), dec,
			billionDecimals, v%billion/billionDivisor)
	} else {
		out = groupDigits(v, sep)
	}
	if neg {
		return "-" + out
	}
	return out
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

// Comma formats n with English thousands separators (478967 -> "478,967"), or
// the billions form past a billion.
func Comma[T Number](n T) string { return Format(n, "") }

// Abbrev formats large totals with a k suffix (34,833,289 -> "34,833k") so
// a planet-wide total fits on one line. A billion and over goes through comma,
// which switches to the "1.3730B" form there. Below 1,000 it prints in full.
func Abbrev[T Number](n T) string {
	abs := int64(n)
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= billion:
		return Comma(n)
	case abs >= 1_000:
		return Comma(int64(n)/1_000) + "k"
	default:
		return Comma(n)
	}
}
