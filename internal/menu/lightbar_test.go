package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
)

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

// A session that cannot render ANSI gets a numbered list instead: the lightbar
// there has no highlight to move and no in-place repaint (issue #99).
func TestPlainSessionGetsNumberedList(t *testing.T) {
	items := []string{"Alpha", "Bravo", "Charlie", "Delta"}
	cases := []struct {
		name string
		keys []rune
		want int
	}{
		{"pick by number", []rune("2\r"), 1},
		{"0 goes back", []rune("0\r"), -1},
		{"Q goes back", []rune("q\r"), -1},
		{"out of range then a valid pick", []rune("9\r4\r"), 3},
		{"blank line then a valid pick", []rune("\r1\r"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeSession{keys: tc.keys}
			if got := chooseFromList(f, true, "Pick", items, "Back"); got != tc.want {
				t.Errorf("chooseFromList = %d, want %d", got, tc.want)
			}
			out := f.out.String()
			if !strings.Contains(out, "1) Alpha") || !strings.Contains(out, "0) Back") {
				t.Errorf("plain list should be numbered with a Back line, got:\n%s", out)
			}
			// Colour escapes are still emitted here — the session wrapper strips
			// them on the way out. What must not appear is a repaint: the cursor
			// and erase controls the lightbar draws between frames.
			for _, ctl := range []string{ansi.Home, ansi.Clear, ansi.EraseLine, ansi.EraseDown} {
				if strings.Contains(out, ctl) {
					t.Errorf("the plain list must not repaint the screen (found %q)", ctl)
				}
			}
		})
	}
}
