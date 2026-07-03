package menu

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		name  string
		input string
		max   int
		want  int
	}{
		{"max shortcut", ">", 100, 100},
		{"k suffix", "5k", 100000, 5000},
		{"m suffix", "2m", 100000000, 2000000},
		{"plain number", "42", 1000, 42},
		{"empty", "", 100, 0},
		{"unparseable", "abc", 100, 0},
		{"max with ignored suffix", ">m", 100, 100},
		{"whitespace tolerated", "  7  ", 100, 7},
		{"case insensitive K", "5K", 100000, 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseAmount(c.input, c.max)
			if got != c.want {
				t.Errorf("parseAmount(%q, %d) = %d, want %d", c.input, c.max, got, c.want)
			}
		})
	}
}
