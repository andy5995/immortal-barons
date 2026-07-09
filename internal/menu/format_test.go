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

// daysAgoLocalized formats a day delta; with the test fakeSession (no language)
// the tr() calls fall back to the English msgids.
func TestDaysAgoLocalized(t *testing.T) {
	const now = "2026-07-09"
	cases := []struct {
		name string
		then string
		want string
	}{
		{"same day", "2026-07-09", "today"},
		{"one day", "2026-07-08", "1 day ago"},
		{"several days", "2026-07-04", "5 days ago"},
		{"future clamps to today", "2026-07-10", "today"},
		{"unparseable then falls back verbatim", "not-a-date", "not-a-date"},
	}
	for _, c := range cases {
		f := &fakeSession{}
		if got := daysAgoLocalized(f, c.then, now); got != c.want {
			t.Errorf("%s: daysAgoLocalized(%q, %q) = %q, want %q", c.name, c.then, now, got, c.want)
		}
	}
}
