package menu

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// The rule must come out byte-for-byte as BRE draws it (cap/bre-kd3-treaty-replies.cap).
func TestEventRuleMatchesBRE(t *testing.T) {
	when := time.Date(2026, 7, 31, 7, 43, 11, 0, time.Local)
	got := sgr.ReplaceAllString(eventRule(1, when), "")
	want := "─────(1)────────────────────────────────────────07/31/2026  07:43:11────────"
	if got != want {
		t.Errorf("rule =\n%q\nwant\n%q", got, want)
	}
	if n := len([]rune(got)); n != eventRuleWidth {
		t.Errorf("rule is %d columns, want %d", n, eventRuleWidth)
	}
}

// An event carried over from a save written before events had a timestamp still
// gets its rule — an unbroken one, with no blank gap where the stamp would be.
func TestEventRuleWithoutTimestamp(t *testing.T) {
	got := sgr.ReplaceAllString(eventRule(2, time.Time{}), "")
	if strings.Contains(got, " ") {
		t.Errorf("undated rule should be solid, got %q", got)
	}
	if !strings.HasPrefix(got, "─────(2)") {
		t.Errorf("undated rule should still be numbered, got %q", got)
	}
}

// The recap numbers its entries from 1 and stamps each one.
func TestShowTurnEventsNumbersAndStampsEachEntry(t *testing.T) {
	f := &fakeSession{keys: []rune("\r")}
	w := newWorld()
	when := time.Date(2026, 7, 31, 9, 7, 7, 0, time.Local)
	w.Player().Events = []game.Event{
		{When: when, Text: "Beta accepted your Full Defense Alliance proposal."},
		{When: when, Text: "Gamma rejected your Full Defense Alliance proposal."},
	}

	showTurnEvents(f, w)

	out := sgr.ReplaceAllString(f.out.String(), "")
	for _, want := range []string{"(1)", "(2)", "07/31/2026  09:07:07", "Beta accepted", "Gamma rejected"} {
		if !strings.Contains(out, want) {
			t.Errorf("recap missing %q:\n%s", want, out)
		}
	}
	if len(w.Player().Events) != 0 {
		t.Errorf("recap should consume the events, %d left", len(w.Player().Events))
	}
}
