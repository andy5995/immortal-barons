package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
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
	got, target := pickRecipient(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if target != targetOne {
		t.Fatalf("pickRecipient('A') target = %v, want targetOne", target)
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
	f := &fakeSession{keys: []rune("?0")} // '?' prints the roster, then cancel
	pickRecipient(f, w, pickOpts{prompt: "Send to:"})
	out := f.out.String()
	// Comma-grouped: IB groups the figures BRE prints bare (a recorded
	// divergence, docs/dev/bre-screens.md).
	if !strings.Contains(out, "7,777") {
		t.Errorf("Score column should show the empire's Score (7,777):\n%s", out)
	}
}

func TestPickRecipientAll(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'Z'}}
	got, target := pickRecipient(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if target != targetAll || got != nil {
		t.Errorf("pickRecipient('Z') = (%v, %v), want (nil, targetAll)", got, target)
	}
}

func TestPickRecipientCancel(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'0'}}
	got, target := pickRecipient(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if target != targetNone || got != nil {
		t.Errorf("pickRecipient('0') = (%v, %v), want (nil, targetNone)", got, target)
	}
}

// treatyWith makes the player and e hold a standing treaty, the state that
// unlocks the all-allies target.
func treatyWith(w *ctx, e *game.Empire) {
	p := w.Player()
	w.World.ProposeTreaty(e, p, "Free Trade Agreement")
	w.World.AcceptTreaty(p, e.Name, "Free Trade Agreement")
}

func TestPickRecipientAllAllies(t *testing.T) {
	w := newWorld()
	rs := recipients(w)
	if len(rs) == 0 {
		t.Skip("no recipients seeded")
	}
	treatyWith(w, rs[0])
	f := &fakeSession{keys: []rune{'*'}}
	got, target := pickRecipient(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if target != targetAllies || got != nil {
		t.Errorf("pickRecipient('*') = (%v, %v), want (nil, targetAllies)", got, target)
	}
	// Strip the SGR runs: BRE colours each piece of this prompt separately, so
	// the escape sequences sit between the '*' and its label.
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "*=All Allies") {
		t.Errorf("prompt should offer *=All Allies:\n%s", f.out.String())
	}
}

// With no treaty there is nobody to reach that way, so the target is not
// offered and the key is not live.
func TestPickRecipientAllAlliesHiddenWithoutTreaty(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'*'}}
	got, target := pickRecipient(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if target != targetNone || got != nil {
		t.Errorf("pickRecipient('*') without a treaty = (%v, %v), want (nil, targetNone)", got, target)
	}
	if strings.Contains(f.out.String(), "All Allies") {
		t.Errorf("prompt should not offer All Allies without a treaty:\n%s", f.out.String())
	}
}

// The all-allies send reaches every treaty partner and nobody else.
func TestSendMessageToAllAllies(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rs := recipients(w)
	if len(rs) < 3 {
		t.Fatalf("need 3 recipients, got %d", len(rs))
	}
	ally, other := rs[0], rs[1]
	treatyWith(w, ally)
	body := "Muster at the river."
	f := &fakeSession{keys: []rune("*" + body + "\r/s")}
	sendMessage(f, w)
	if len(ally.Mail) != 1 {
		t.Errorf("ally got %d messages, want 1", len(ally.Mail))
	} else if !strings.Contains(ally.Mail[0].Body, body) {
		t.Errorf("ally message = %q, want it to carry %q", ally.Mail[0].Body, body)
	}
	if len(other.Mail) != 0 {
		t.Errorf("a realm with no treaty got %d messages, want 0", len(other.Mail))
	}
}
