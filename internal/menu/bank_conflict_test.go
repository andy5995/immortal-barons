package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestWithdrawVanishedEmpireConflict proves Withdraw re-resolves the active
// empire inside its write transaction. Node B gathers its realm; another node
// removes it, leaving only a decoy realm with a full bank; node B then commits a
// withdrawal. Pointer-reuse-by-index means the removed realm's slot now holds the
// decoy's data, so mutating the pre-gathered pointer would drain the decoy's
// bank. The re-resolve finds no realm and aborts instead.
func TestWithdrawVanishedEmpireConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 0
		p.Bank = 1000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Bank = 5000
		d.Gold = 0
	})

	_ = b.Player()
	commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })

	withdraw := money("Withdraw", func(p *game.Empire) int { return p.Bank }, (*game.World).Withdraw)
	fb := &fakeSession{keys: []rune("1000\r ")}
	withdraw(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Bank != 5000 || d.Gold != 0 {
		t.Fatalf("decoy bank/gold = %d/%d, want 5000/0 — a stale pointer withdrew from the wrong empire", d.Bank, d.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}

// TestInvestVanishedEmpireConflict is the same conflict for Invest.
func TestInvestVanishedEmpireConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 1000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = 1_000_000
	})

	_ = b.Player()
	commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })

	fb := &fakeSession{keys: []rune("5\r1000\r ")}
	investFunds(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); len(d.Investments) != 0 || d.Gold != 1_000_000 {
		t.Fatalf("decoy investments/gold = %d/%d, want 0/1000000 — a stale pointer invested the wrong empire's gold", len(d.Investments), d.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}
