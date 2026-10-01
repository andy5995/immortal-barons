package main

import "testing"

// Numbers sort as numbers, so board 10 comes after board 9, and the rest as
// text without regard to case.
func TestLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"9", "10", true},
		{"10", "9", false},
		{"2 days", "10 days", true},
		{"1.5 days", "0.4 days", false},
		{"alpha", "Beta", true},
		{"never", "2 days", false},
	} {
		if got := less(c.a, c.b); got != c.want {
			t.Errorf("less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Copy's text is the headings then each row, tab-separated, one per line.
func TestTSV(t *testing.T) {
	got := tsv([]string{"ID", "What"}, [][]string{{"29", "Send Spy (1 agent)"}, {"30", "Nuclear Assault"}})
	want := "ID\tWhat\n29\tSend Spy (1 agent)\n30\tNuclear Assault\n"
	if got != want {
		t.Errorf("tsv = %q, want %q", got, want)
	}
}
