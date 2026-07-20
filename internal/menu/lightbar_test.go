package menu

import "testing"

func TestLightbarSelect(t *testing.T) {
	items := []string{"Alpha", "Bravo", "Charlie", "Delta"}
	// Selectable rows: 0 Alpha, 1 Bravo, 2 Charlie, 3 Delta, 4 = Back.
	const esc = 0x1b
	cases := []struct {
		name string
		keys []rune
		want int
	}{
		{"CSI down twice + Enter", []rune{esc, '[', 'B', esc, '[', 'B', '\r'}, 2},
		{"SS3 down + Enter", []rune{esc, 'O', 'B', '\r'}, 1},
		{"up wraps onto Back + Enter", []rune{esc, '[', 'A', '\r'}, -1},
		{"up twice reaches last topic + Enter", []rune{esc, '[', 'A', esc, '[', 'A', '\r'}, 3},
		{"prefix type-ahead 'br' + Enter", []rune("br\r"), 1},
		{"prefix restarts on non-match", []rune("bd\r"), 3},  // 'b'->Bravo, 'd' has no "bd*" -> restart to Delta
		{"type-ahead onto Back + Enter", []rune("ba\r"), -1}, // "Back" is the only "ba*"
		{"Q backs out", []rune("q"), -1},
		{"Backspace backs out", []rune{0x7f}, -1},
		{"digits do nothing then Enter picks first", []rune("55\r"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeSession{keys: tc.keys}
			if got := lightbarSelect(f, "Pick", items, "Back"); got != tc.want {
				t.Errorf("lightbarSelect = %d, want %d", got, tc.want)
			}
		})
	}
}
