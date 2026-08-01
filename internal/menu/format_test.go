package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestAbbrevMoney(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1_000, "1k"},
		{34_833_289, "34,833k"},
		{999_999_999, "999,999k"},
		{1_000_000_000, "1,000m"},
		{1_373_000_000, "1,373m"},
		{-500, "-500"},
		{-34_833_289, "-34,833k"},
		{-1_000_000_000, "-1,000m"},
	}
	for _, c := range cases {
		if got := abbrevMoney(c.n); got != c.want {
			t.Errorf("abbrevMoney(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestFormatGoldLocale(t *testing.T) {
	cases := []struct {
		n    int
		lang string
		want string
	}{
		{1847392104, "de", "1.847.392.104"},
		{1847392104, "ru", "1 847 392 104"},
		{1847392104, "en", "1,847,392,104"},
		{-1234567, "en", "-1,234,567"},
		{-1234567, "", "-1,234,567"}, // "" defaults to the English comma
		{1234567, "xx", "1,234,567"}, // unknown language falls back to comma
		{0, "de", "0"},
	}
	for _, c := range cases {
		if got := formatGold(c.n, c.lang); got != c.want {
			t.Errorf("formatGold(%d, %q) = %q, want %q", c.n, c.lang, got, c.want)
		}
	}
	if comma(1234567) != "1,234,567" {
		t.Errorf("comma should equal English formatGold, got %q", comma(1234567))
	}
}

func TestHQStatus(t *testing.T) {
	cases := []struct {
		hq   int
		want string
	}{
		{0, "None"},
		{1, "1%"},
		{50, "50%"},
		{99, "99%"},
		{100, "Complete"},
		{150, "Complete"},
	}
	for _, c := range cases {
		if got := hqStatus(&game.Empire{HQ: c.hq}); got != c.want {
			t.Errorf("hqStatus(HQ=%d) = %q, want %q", c.hq, got, c.want)
		}
	}
}

func TestTrimTrailingBlank(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no trailing blanks", []string{"a", "b"}, "a\nb"},
		{"strips trailing empties", []string{"a", "", ""}, "a"},
		{"strips trailing whitespace-only", []string{"a", "b", "  ", "\t"}, "a\nb"},
		{"keeps interior blank", []string{"a", "", "b"}, "a\n\nb"},
		{"all blank -> empty", []string{"", "  ", ""}, ""},
		{"empty slice -> empty", nil, ""},
		{"single line", []string{"only"}, "only"},
	}
	for _, c := range cases {
		if got := trimTrailingBlank(c.lines); got != c.want {
			t.Errorf("%s: trimTrailingBlank(%q) = %q, want %q", c.name, c.lines, got, c.want)
		}
	}
}
