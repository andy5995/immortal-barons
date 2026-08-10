package menu

import "github.com/andy5995/immortal-barons/internal/numfmt"

// The number-rendering rules live in internal/numfmt so the game engine — which
// writes player-visible event text and cannot import this package — formats a
// figure exactly the way these screens do. These are the menu-side names.

// formatGold renders n with lang's thousands separator, or the abbreviated
// billions form ("1.8473B") once it reaches a billion.
func formatGold[T numfmt.Number](n T, lang string) string { return numfmt.Format(n, lang) }

// comma formats n with English thousands separators.
func comma[T numfmt.Number](n T) string { return numfmt.Comma(n) }

// abbrevMoney formats a planet-wide total compactly ("34,833k").
func abbrevMoney[T numfmt.Number](n T) string { return numfmt.Abbrev(n) }
