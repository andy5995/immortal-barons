package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The invest prompt offers the principal that fills the chosen date's room below
// the ceiling, not the gold in hand (#284); the figures are the original's own.
func TestInvestPromptOffersDateRoom(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.InvestRate = 70
	e := w.AddHuman("alice", "Alice")
	e.Gold = 100_000_000
	e.Investments = []game.Investment{{Amount: 1, Return: 1_923_554_260, MaturesDay: w.GameDay + 9}}
	c := &ctx{World: w, handle: "alice"}

	f := &fakeSession{keys: []rune("9\r41581417\r\r ")}
	investFunds(f, c)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "(0; 41,581,417)") {
		t.Fatalf("prompt did not offer 41,581,417:\n%s", out)
	}
	if !strings.Contains(out, "76,445,739") {
		t.Errorf("expected return 76,445,739 not quoted:\n%s", out)
	}
	if len(e.Investments) != 2 || e.Gold != 58_418_583 {
		t.Errorf("investments/gold = %d/%d, want 2/58418583", len(e.Investments), e.Gold)
	}
}

// The term prompt holds BRE's bounds (#291): a term under the 2-day minimum
// cancels, as BRE's bank does, and one over the 10-day maximum is corrected to
// 10 on screen and committed by a second Enter.
func TestInvestTermBounds(t *testing.T) {
	cases := []struct {
		name     string
		keys     string
		wantDays int // 0: nothing invested
	}{
		{"below minimum cancels", "1\r", 0},
		{"zero cancels", "0\r", 0},
		{"minimum", "2\r1000\r\r", 2},
		{"over maximum clamps", "15\r\r1000\r\r", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := game.NewWorldSeed(game.DefaultConfig(), 1)
			e := w.AddHuman("alice", "Alice")
			e.Gold = 100_000
			c := &ctx{World: w, handle: "alice"}

			f := &fakeSession{keys: []rune(tc.keys)}
			investFunds(f, c)

			out := stripANSI(f.out.String())
			if !strings.Contains(out, "day minimum on investments") {
				t.Fatalf("invest screen not reached:\n%s", out)
			}
			if tc.wantDays == 0 {
				if len(e.Investments) != 0 || e.Gold != 100_000 {
					t.Errorf("investments/gold = %d/%d, want 0/100000", len(e.Investments), e.Gold)
				}
				if strings.Contains(out, "How much would you like to invest?") {
					t.Errorf("a cancelled term still asked for an amount:\n%s", out)
				}
				return
			}
			if len(e.Investments) != 1 {
				t.Fatalf("investments = %d, want 1:\n%s", len(e.Investments), out)
			}
			if got := e.Investments[0].MaturesDay - w.GameDay; got != tc.wantDays {
				t.Errorf("matures in %d days, want %d", got, tc.wantDays)
			}
			if e.Gold != 99_000 {
				t.Errorf("gold = %d, want 99000", e.Gold)
			}
		})
	}
}

// A loan term under the 1-day minimum cancels with nothing borrowed, as BRE's
// bank does and as the investment term does.
func TestLoanTermUnderMinimumCancels(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Loans = 1_000, nil
	f := &fakeSession{keys: []rune("0\r")}
	cashRelief(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "When do you wish to pay your loan back") {
		t.Fatalf("never reached the loan term prompt:\n%s", out)
	}
	if strings.Contains(out, "How many would you like to borrow?") || len(p.Loans) != 0 || p.Gold != 1_000 {
		t.Errorf("a 0-day term should cancel: loans %+v, gold %d\n%s", p.Loans, p.Gold, out)
	}
}
