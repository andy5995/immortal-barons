package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

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

func TestComma(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 612: "612", 1000: "1,000", 15853: "15,853", 478967: "478,967", 1000000: "1,000,000", -1234: "-1,234"}
	for in, want := range cases {
		if got := comma(in); got != want {
			t.Errorf("comma(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestEditAmountKillLine covers Ctrl-U in the numeric editor, which is the case
// this key was asked for: a mistyped 1,000,000,000 cleared in one keystroke
// instead of ten backspaces, with the field still editable afterwards.
func TestEditAmountKillLine(t *testing.T) {
	keys := append([]rune("1000000000"), session.KillLine)
	keys = append(keys, []rune("25\r")...)
	fs := &fakeSession{keys: keys}
	if got := editAmount(fs, "", 0, 1_000_000_000); got != 25 {
		t.Errorf("amount = %d, want 25", got)
	}
	if n := strings.Count(fs.out.String(), "\b \b"); n != 10 {
		t.Errorf("erased %d digits, want 10", n)
	}
}

// The k/m/b shortcuts expand in place, so Ctrl-U has to erase what is ON the
// line -- every digit the expansion wrote -- not the keystrokes that produced
// it. One "b" is ten columns.
func TestEditAmountKillLineErasesAnExpandedShortcut(t *testing.T) {
	fs := &fakeSession{keys: append([]rune{'1', 'b', session.KillLine}, []rune("7\r")...)}
	if got := editAmount(fs, "", 0, 1_000_000_000); got != 7 {
		t.Errorf("amount = %d, want 7", got)
	}
	if n := strings.Count(fs.out.String(), "\b \b"); n != 10 {
		t.Errorf("erased %d columns, want 10 — 1b renders as ten digits", n)
	}
}
