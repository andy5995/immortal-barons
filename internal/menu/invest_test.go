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
