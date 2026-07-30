package game

import (
	"strings"
	"testing"
)

// Quotes and deeds must compose into one clean sentence: the quote carries no
// trailing narration and no closing stop, the deed supplies both. A first cut
// mixed the two styles and produced "…she says, raising it. as the treasury
// doors are seen closing."
func TestProclamationsComposeCleanly(t *testing.T) {
	for _, q := range queenProclamations {
		if strings.HasSuffix(q, ".") {
			t.Errorf("quote should not end with a full stop (the deed closes the sentence): %q", q)
		}
		if strings.Contains(q, "she says") || strings.Contains(q, "she declares") {
			t.Errorf("quote carries its own narration; move it to a deed: %q", q)
		}
	}
	for _, d := range proclamationDeeds {
		if !strings.HasSuffix(d, ".") {
			t.Errorf("deed should end the sentence: %q", d)
		}
		if r := rune(d[0]); r >= 'A' && r <= 'Z' {
			t.Errorf("deed continues the sentence, so it starts lowercase: %q", d)
		}
	}
	got := Proclamation("I came, I saw, I invoiced", "and returns to her banquet.")
	want := `The Queen Royale proclaims, "I came, I saw, I invoiced", and returns to her banquet.`
	if got != want {
		t.Errorf("proclamation format:\n got %s\nwant %s", got, want)
	}
}

// Clean-room guard: BRE's own two proclamations must not appear in IB's pool.
func TestProclamationsAreNotBREs(t *testing.T) {
	for _, q := range queenProclamations {
		l := strings.ToLower(q)
		if strings.Contains(l, "not a crook") || strings.Contains(l, "what you can do for me") {
			t.Errorf("this is the original's line, not ours: %q", q)
		}
	}
}
