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

func TestClampAmt(t *testing.T) {
	cases := []struct {
		name string
		n    int
		max  int
		want int
	}{
		{"below zero clamps to 0", -5, 100, 0},
		{"above max clamps to max", 500, 100, 100},
		{"in range unchanged", 42, 100, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampAmt(c.n, c.max); got != c.want {
				t.Errorf("clampAmt(%d, %d) = %d, want %d", c.n, c.max, got, c.want)
			}
		})
	}
}

func TestPromptSuggested(t *testing.T) {
	cases := []struct {
		name      string
		keys      string
		suggested int
		max       int
		want      int
	}{
		{"empty enter returns suggested", "\r", 7, 100, 7},
		{"> prefills max, enter accepts it", ">\r", 7, 100, 100},
		{"typed number is used", "5\r", 7, 100, 5},
		{"> then backspace edits back to nothing, enter returns suggested", ">\r", 0, 42, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSession{keys: []rune(c.keys)}
			got := promptSuggested(f, "How much?", c.suggested, c.max)
			if got != c.want {
				t.Errorf("promptSuggested(%q, suggested=%d, max=%d) = %d, want %d",
					c.keys, c.suggested, c.max, got, c.want)
			}
		})
	}
}
