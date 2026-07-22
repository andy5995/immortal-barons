package menu

import (
	"strings"
	"testing"
)

func TestRecipientsExcludesPlayerAndDead(t *testing.T) {
	w := newWorld()
	// newWorld seeds one AI empire plus the human player.
	if len(w.Empires) < 2 {
		t.Fatalf("expected at least 2 empires, got %d", len(w.Empires))
	}
	rs := recipients(w)
	for _, e := range rs {
		if e == w.Player() {
			t.Errorf("recipients() included the player %q", e.Name)
		}
		if !e.Alive {
			t.Errorf("recipients() included a dead empire %q", e.Name)
		}
	}

	// Kill the first non-player recipient and confirm it disappears.
	if len(rs) > 0 {
		rs[0].Alive = false
		rs2 := recipients(w)
		for _, e := range rs2 {
			if e == rs[0] {
				t.Errorf("recipients() still included empire killed after first call")
			}
		}
		if len(rs2) != len(rs)-1 {
			t.Errorf("recipients() after death = %d entries, want %d", len(rs2), len(rs)-1)
		}
	}
}

func TestRecipientIndex(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		n    int
		want int
	}{
		{"A selects first", 'A', 3, 0},
		{"lowercase a selects first", 'a', 3, 0},
		{"B selects second", 'B', 3, 1},
		{"out of range n", 'C', 2, -1},
		{"below A", '0', 3, -1},
		{"beyond 25 cap", rune('A' + 25), 30, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := recipientIndex(c.r, c.n); got != c.want {
				t.Errorf("recipientIndex(%q, %d) = %d, want %d", c.r, c.n, got, c.want)
			}
		})
	}
}

func TestPickRecipientSelectsByLetter(t *testing.T) {
	w := newWorld()
	rs := recipients(w)
	if len(rs) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'A'}}
	got, all := pickRecipient(f, w, "Send to:", true)
	if all {
		t.Fatalf("pickRecipient with 'A' returned all=true")
	}
	if got != rs[0] {
		t.Errorf("pickRecipient('A') = %v, want %v", got, rs[0])
	}
}

// The picker's Score column must show the empire's cumulative Score, not a
// second copy of Land (which made every row read Land==Score).
func TestPickRecipientShowsScoreNotLand(t *testing.T) {
	w := newWorld()
	rs := recipients(w)
	if len(rs) == 0 {
		t.Skip("no recipients seeded")
	}
	w.With(func() {
		rs[0].Land = 4242
		rs[0].Score = 7777
	})
	f := &fakeSession{keys: []rune{'0'}} // 0 cancels; we only need the rendered list
	pickRecipient(f, w, "Send to:", false)
	out := f.out.String()
	if !strings.Contains(out, "7777") {
		t.Errorf("Score column should show the empire's Score (7777):\n%s", out)
	}
}

func TestPickRecipientAll(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'Z'}}
	got, all := pickRecipient(f, w, "Send to:", true)
	if !all || got != nil {
		t.Errorf("pickRecipient('Z') = (%v, %v), want (nil, true)", got, all)
	}
}

func TestPickRecipientCancel(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'0'}}
	got, all := pickRecipient(f, w, "Send to:", true)
	if all || got != nil {
		t.Errorf("pickRecipient('0') = (%v, %v), want (nil, false)", got, all)
	}
}
