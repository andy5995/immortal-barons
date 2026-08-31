package menu

import (
	"errors"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
)

func TestFormatGoldLocale(t *testing.T) {
	cases := []struct {
		n    int64
		lang string
		want string
	}{
		{847392104, "de", "847.392.104"},
		{847392104, "ru", "847 392 104"},
		{847392104, "en", "847,392,104"},
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

// A figure a billion or over is printed in full with its thousands grouped, the
// way BRE prints its own — `Bank: 2,000,000,000` and `Today $1,846,153,847`,
// both off live captures. Nothing switches to a decimal "B" form, and no float
// is involved at any size.
func TestFormatGoldBillions(t *testing.T) {
	cases := []struct {
		n    int64
		lang string
		want string
	}{
		{999_999_999, "en", "999,999,999"},
		{1_000_000_000, "en", "1,000,000,000"},
		{1_847_392_104, "en", "1,847,392,104"},
		{2_000_000_000, "en", "2,000,000,000"}, // the cap, exactly as BRE prints it
		{999_000_000_000, "en", "999,000,000,000"},
		{-1_847_392_104, "en", "-1,847,392,104"},
		{1_847_392_104, "de", "1.847.392.104"},
		{1_847_392_104, "ru", "1 847 392 104"},
	}
	for _, c := range cases {
		if got := formatGold(c.n, c.lang); got != c.want {
			t.Errorf("formatGold(%d, %q) = %q, want %q", c.n, c.lang, got, c.want)
		}
	}
}

// The same helper serves counts, not just gold — that is the point of it being
// generic. A plain int caller gets the identical rendering.
func TestFormatGoldTakesCounts(t *testing.T) {
	if got := comma(int(1_500_000_000)); got != "1,500,000,000" {
		t.Errorf("a count of 1.5 billion formatted as %q, want %q", got, "1,500,000,000")
	}
	if got := comma(7212); got != "7,212" {
		t.Errorf("a small count formatted as %q, want %q", got, "7,212")
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

// A message longer than the terminal must break at a space, not wherever the
// terminal edge lands, and every line carries the indent.
func TestWrapIndented(t *testing.T) {
	long := "Your forces changed while you prepared the attack — only units still under your command were sent."
	got := WrapIndented(long, "  ")
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("line lost its indent: %q", line)
		}
		if len([]rune(line)) >= ansi.ScreenCols {
			t.Errorf("line is %d columns, wider than the screen: %q", len([]rune(line)), line)
		}
	}
	if !strings.Contains(got, "\n") {
		t.Error("a 99-character message should have wrapped")
	}
	// Wrapping must not lose or split a word.
	flat := strings.Join(strings.Fields(got), " ")
	if flat != strings.Join(strings.Fields(long), " ") {
		t.Errorf("wrapping changed the text:\n%q\n%q", flat, long)
	}
}

// A failure must be distinguishable from a success without colour: on a
// monochrome terminal, or one the escapes were stripped for, the two otherwise
// print identically.
func TestFailureIsMarkedWithoutColour(t *testing.T) {
	fs := &fakeSession{keys: []rune(" ")}
	fail(fs, errors.New("not enough gold"))
	fo := &fakeSession{}
	okNoPause(fo, "the deal is struck")

	strip := func(s string) string { return anyEscape.ReplaceAllString(s, "") }
	bad, good := strip(fs.out.String()), strip(fo.out.String())
	if !strings.Contains(bad, "! not enough gold") {
		t.Errorf("a failure should carry a marker of its own, got %q", bad)
	}
	if strings.Contains(good, "!") {
		t.Errorf("a success should not carry the failure marker, got %q", good)
	}
}
