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

// Format prints a figure in full however large it grows, with its thousands
// grouped. That is what BRE's MONEY screens do: `Bank: 2,000,000,000` and
// `Today $1,846,153,847` are its own screens at the top of its range, so ten
// digits need no abbreviating and nothing here needs a float. IB rendered a
// billion and over as a fixed 4-decimal "1.8473B" until v0.0.8; the one place
// BRE itself printed that form was a numeric prompt's ceiling, which IB no
// longer shows either.
//
// **This is one of FOUR ways BRE spells a figure, and it is chosen per SCREEN,
// not per quantity.** The others are all below: GroupLong (grouped only past
// four digits), Short (the score table's two-step k/m) and Thousands (the Daily
// Bulletin's /1000 with an unconditional `k`). The sentence above read as a
// global claim about BRE until 2026-08-30 and suppressed the question for
// months — while a capture had carried `You took 78k Gold` since July, and the
// removed Abbrev's own comment described BRE's k/m columns thirty lines down.
// Do not generalise any of the four past the screens it belongs to.

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

// IB carried an Abbrev here until 2026-08-30: a k/m/b ladder stepping at each
// magnitude, used for the Daily Bulletin's Total Net Worth row alone. It was
// removed with #205's premise, which claimed BRE's columns "pick ONE suffix and
// keep it, differing by column rather than by size". They do not — one capture's
// Net Worth column runs 100k / 1006k / 1026k / 10m — so the rule it existed to
// avoid was never real. The three formats below are BRE's own.

// ShortThreshold is the value at or above which a score-table figure is divided
// down a step. BINARY-VERIFIED: BRE.EXE image 0x0EA9C / 0x0EADC compare the
// figure against 0x2710 before each divide.
const ShortThreshold = 10000

// Short renders a figure the way BRE's score tables do: once it reaches
// ShortThreshold it is divided by 1000 and given a bare "k", and once more for
// "m". BINARY-VERIFIED against BRE.EXE image 0x0EA89 (the helper
// show_player_list calls for its Score and Net Worth columns, and nothing
// else calls for Territory).
//
// The two divides are separate `if`s, NOT a loop, and that is the whole
// character of the format: 1,213,456 falls to 1213, which is under the
// threshold, so it stays "1213k" and never reaches "m". A capture has a net
// worth of 3,180,000 printing "3180k" beside another realm's "12m"
// (cap/20240527-134Pho_Lazarus_Public.cap) — the crossover is at exactly
// 10,000,000, not at a million. There is no "b" tier either: two billion
// prints "2000m".
//
// Division truncates and a negative figure is never shortened, both as the
// original has it (its first test is `jl` past the whole thing).
func Short[T Number](n T) string {
	v := int64(n)
	if v < 0 {
		return strconv.FormatInt(v, 10)
	}
	suffix := ""
	if v >= ShortThreshold {
		v /= 1000
		suffix = "k"
	}
	if v >= ShortThreshold {
		v /= 1000
		suffix = "m"
	}
	return strconv.FormatInt(v, 10) + suffix
}

// GroupLong renders a figure the way BRE's own grouping helper does: grouped
// with lang's separator, but ONLY once the digits run past four, so a
// four-digit figure prints bare. BINARY-VERIFIED: BRE.EXE image 0x0E2F7 tests
// the rendered digit string's LENGTH against 4 and returns it ungrouped when it
// is not longer. A capture has the same score table printing Territory 3469
// beside another planet's 14,203 (cap/20240527-134Pho_Lazarus_Public.cap).
//
// The sign is not counted: the original takes the absolute value before
// rendering, so -12345 groups on its five digits.
//
// This is NOT Format's rule, and the two are deliberately separate. Format
// groups at any size and is what IB prints for money and event text; this one
// belongs to the score table's Territory column, where matching the original
// matters more than internal consistency.
func GroupLong[T Number](n T, lang string) string {
	v := int64(n)
	neg := v < 0
	if neg {
		v = -v
	}
	digits := strconv.FormatInt(v, 10)
	out := digits
	if len(digits) > 4 {
		out = Format(v, lang)
	}
	if neg {
		return "-" + out
	}
	return out
}

// Thousands renders a figure the way BRE's Daily Bulletin renders Total Net
// Worth: divided by 1000 and suffixed "k", ALWAYS — there is no threshold, so a
// planet worth nothing prints "0k" — with the quotient then grouped by
// GroupLong's rule. Captures run `0k`, `12k`, `1255k` and `97,678k`, so the
// comma appears only once the quotient itself passes four digits, and the
// suffix never steps to "m" however large the figure grows
// (cap/121125-666H4H_Camembert_Public.cap).
//
// This is a THIRD rule, distinct from Short and from Format. Short is the score
// table's two-step k/m; this one never reaches m. The Bulletin's other two rows
// (Population, Regions) are plain GroupLong with no suffix at all.
func Thousands[T Number](n T, lang string) string {
	return GroupLong(int64(n)/1000, lang) + "k"
}
