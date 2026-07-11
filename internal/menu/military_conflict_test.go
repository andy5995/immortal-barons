package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestBuyTroopersVanishedEmpireConflict proves a military purchase (buy2 →
// Recruit) re-resolves the active empire inside its write transaction. Node B
// gathers its realm; another node removes it, leaving only a decoy realm; node B
// then commits. Because the reload reuses *Empire pointers by slice index, the
// removed realm's slot now carries the decoy's data — so mutating the
// pre-gathered pointer would recruit troopers with the decoy's gold. The
// re-resolve finds no realm and aborts with the realm-changed notice instead.
func TestBuyTroopersVanishedEmpireConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = game.PriceTrooper * 100
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = game.PriceTrooper * 100
	})
	startTroopers := committedEmpire(t, cfg, "decoy").Troopers

	_ = b.Player()
	commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })

	buyTroopers := buy2("Troopers", false, func(w *ctx) int { return w.Prices.Trooper }, (*game.World).Recruit)
	fb := &fakeSession{keys: []rune("10\r")}
	buyTroopers(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Troopers != startTroopers {
		t.Fatalf("decoy troopers = %d, want %d — a stale pointer recruited for the wrong empire", d.Troopers, startTroopers)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}
