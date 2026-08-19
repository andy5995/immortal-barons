package textwrap

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The audience is a sysop at an 80-column console, where the unwrapped warning
// split "Basename" across the break (#156).
func TestWrapKeepsWordsWholeInsideTheWidth(t *testing.T) {
	const prefix = "barons-ftn: warning: "
	const warning = "attachment subjects have 6 byte(s) to spare in the FTN Type-2 field; " +
		`Set SubjectPath in ftn.cfg: "Basename" writes the filename alone, ` +
		"with AttachDir naming the directory the mailer searches"

	indent := strings.Repeat(" ", len(prefix))
	lines := strings.Split(prefix+Wrap(warning, Console, indent), "\n")
	if len(lines) < 2 {
		t.Fatal("the warning did not wrap at all")
	}
	for _, line := range lines {
		if n := utf8.RuneCountInString(line); n > Console {
			t.Errorf("line is %d columns, want at most %d: %q", n, Console, line)
		}
	}
	if !strings.Contains(strings.Join(lines, "\n"), `"Basename"`) {
		t.Error(`"Basename" was split across a line break`)
	}
	if strings.Join(strings.Fields(strings.Join(lines, " ")), " ") != prefix+warning {
		t.Error("wrapping changed the text")
	}
}

// A multibyte glyph occupies one column, not its byte length; counting bytes
// wrapped these lines early.
func TestWrapCountsRunesNotBytes(t *testing.T) {
	text := strings.Repeat("— ", 20)
	for _, line := range strings.Split(Wrap(text, 10, ""), "\n") {
		if n := utf8.RuneCountInString(line); n > 10 {
			t.Errorf("line is %d columns, want at most 10: %q", n, line)
		}
	}
}

// The indent is what the caller already printed, so the first line has to be
// measured as though it were there.
func TestWrapCountsTheIndentOnTheFirstLine(t *testing.T) {
	indent := strings.Repeat(" ", 20)
	first := strings.Split(Wrap("aaaa bbbb cccc dddd eeee", 30, indent), "\n")[0]
	if utf8.RuneCountInString(indent+first) > 30 {
		t.Errorf("first line plus indent is %d columns, want at most 30: %q", len(indent+first), first)
	}
}

// A path has no spaces to break at, and half a path is worse than a long line.
func TestWrapLeavesAnUnbreakableWordAlone(t *testing.T) {
	path := "/sbbs/xtrn/immortal-barons/data/ibnodes.dat"
	if got := Wrap(path, 20, "  "); got != path {
		t.Errorf("Wrap broke an unbreakable word: %q", got)
	}
}
