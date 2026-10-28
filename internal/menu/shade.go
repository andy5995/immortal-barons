package menu

// The bars on the Empire Status block and the Bank's investment chart fill in
// shades rather than solid blocks, so the bright colors do not glare: a ▒ per
// full step and a lighter ░ for any remainder.
const (
	shadeFull = "▒"
	shadePart = "░"
)

// shadeCells is how many whole cells of a cells-long bar pct percent fills, and
// whether a partly filled cell follows them. pct is held to 0..100.
func shadeCells(pct, cells int) (full int, part bool) {
	pct = min(max(pct, 0), 100)
	return pct * cells / 100, pct*cells%100 != 0
}
