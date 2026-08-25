package game

import "math"

// money.go — the width-safe arithmetic every price and payment goes through.
// Money is int64 because plain int is 32 bits on the door builds this project
// supports, and unit counts are int; these convert between the two without
// wrapping.

// number is the numeric width these scaling helpers accept: plain counts (int)
// and money (int64). One generic body keeps the int64 intermediate — and so the
// 32-bit correctness the comment above describes — for both.
type number interface{ ~int | ~int64 }

// pctOf is v * p / 100 with an int64 intermediate, for a percentage taken of a
// quantity that can reach money scale. A plain v*p/100 overflows int32 on a
// 32-bit build once v passes ~21 million, which turns a rich empire's spending
// budget negative and makes it buy nothing at all.
func pctOf[T number](v T, p int) T { return T(int64(v) * int64(p) / 100) }

// goldCost is the price of n units at `unit` gold each, in money width. Always
// int64: a large order at a high price passes 2^31, which on a 32-bit door
// wrapped the bill negative and handed the goods over for free.
func goldCost(n, unit int) int64 { return int64(n) * int64(unit) }

// UnitsAffordable is how many units at `price` gold each `gold` buys. The divide
// happens in money width; the count comes back in count width, clamped so a vast
// treasury against a cheap unit cannot wrap int on a 32-bit door.
//
// Exported because every buy screen needs it. Four of them had hand-rolled the
// expression instead, and they had already drifted: one omitted the price guard
// and one the MaxInt32 clamp, so with the money cap raised past 2 billion (the
// sysop knob reaches 999) a rich realm bidding on a cheap remote listing wrapped
// int on a 32-bit door and was told it could not afford one unit.
func UnitsAffordable(gold int64, price int) int {
	if price <= 0 {
		return 0
	}
	return int(min(gold/int64(price), math.MaxInt32))
}
