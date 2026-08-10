package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
)

// The real lines that broke, each one splitting a number across the margin:
// "1\n07 tanks", "a\nnti-air", "19 ta\nnks". A figure cut in half is worse than
// a cut word — the reader cannot tell 107 from 1 and 07.
func TestReportsWrapAtWordBoundaries(t *testing.T) {
	cases := []struct{ name, text string }{
		{"pirate raid", "The Barbarians raided you, carrying off 515 troopers, 291 jets, 525 turrets, 107 tanks, 3 agents, 24999 gold!"},
		{"bomber strike", "Your bombers hit the airfields: 1431 enemy jets destroyed, 258 bombers lost to anti-air."},
		{"raid result", "You broke the Dunkleoids and recovered 104 troopers, 62 jets, 116 turrets, 19 tanks, 4999 gold, 6 regions."},
	}
	for _, c := range cases {
		for _, line := range strings.Split(wrapReport(c.text), "\n") {
			if n := len([]rune(line)); n >= ansi.ScreenCols {
				t.Errorf("%s: line of %d columns still runs past the screen: %q", c.name, n, line)
			}
		}
		// Every word must survive whole, in order — the point of wrapping at
		// spaces rather than letting the terminal cut wherever it reaches 80.
		got := strings.Fields(wrapReport(c.text))
		want := strings.Fields(c.text)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: wrapping changed the words:\n got %q\nwant %q", c.name, got, want)
		}
	}
}

// A report's own line breaks are its layout — casualties on their own line,
// blank lines between sections. help.Wrap alone would run them all together.
func TestWrapReportKeepsTheReportsOwnLines(t *testing.T) {
	report := "Defeat! Your forces returned exhausted.\n" +
		"Your casualties: 1000 troopers, 200 jets, 400 tanks, 258 bombers.\n" +
		"\n" +
		"The enemy lost: 745 troopers, 517 turrets, 215 tanks, 308 jets.\n"
	got := wrapReport(report)
	if strings.Count(got, "\n") < strings.Count(report, "\n") {
		t.Errorf("line breaks were lost:\n%s", got)
	}
	if !strings.Contains(got, "exhausted.\nYour casualties:") {
		t.Error("two short lines were run into one paragraph")
	}
	if !strings.Contains(got, "bombers.\n\nThe enemy lost:") {
		t.Error("the blank line between sections was dropped")
	}
}

// Wrapping has to happen before the figures are coloured: the escapes take no
// columns on screen but are counted by every length measurement, so a report
// coloured first wraps far short of the margin.
func TestWrapBeforeColouringKeepsLinesFull(t *testing.T) {
	text := "You broke the Dunkleoids and recovered 104 troopers, 62 jets, 116 turrets, 19 tanks, 4999 gold, 6 regions."
	right := strings.Split(hiNums(wrapReport(text)), "\n")
	wrong := strings.Split(wrapReport(hiNums(text)), "\n")
	if len(wrong) <= len(right) {
		t.Skip("no colouring applied in this build, nothing to compare")
	}
	if len(right) != 2 {
		t.Errorf("wrapped to %d lines, want 2 for a 106-column report", len(right))
	}
}

// TestNewsItemWraps checks a long planetary-news line breaks between words and
// indents its continuation under the arrow, rather than being cut at the
// terminal margin mid-number (BRE wraps these with a 5-space hanging indent).
func TestNewsItemWraps(t *testing.T) {
	item := "The planet recoils as Immortal Baron hits Obsidian Sovereigns with nuclear missiles, costing 1,234,567 gold and 89,012 lives across the northern provinces."
	lines := strings.Split(wrapHanging(item, "", newsItemIndent), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the line to wrap, got one line:\n%s", lines[0])
	}
	for i, l := range lines {
		// The arrow and its space sit outside the wrap, so line one carries 4
		// more printable columns than the string itself.
		width := len([]rune(l))
		if i == 0 {
			width += 4
		}
		if width >= ansi.ScreenCols {
			t.Errorf("line %d is %d columns, over the %d-column screen: %q", i+1, width, ansi.ScreenCols, l)
		}
		if i > 0 && !strings.HasPrefix(l, newsItemIndent) {
			t.Errorf("continuation line %d is not indented: %q", i+1, l)
		}
	}
	if got := strings.Join(strings.Fields(strings.Join(lines, " ")), " "); got != item {
		t.Errorf("wrapping changed the text:\n got %q\nwant %q", got, item)
	}
}
