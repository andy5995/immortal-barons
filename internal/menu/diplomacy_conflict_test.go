package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestSendTradeDealVanishedRecipientConflict proves Send Trade Deal re-finds its
// recipient by realm name inside the transaction. Node B snapshots its
// recipients [Victimville, Decoyland], picks Victimville, and while B is typing
// the amount another node eliminates Victimville. Because encoding/json reuses
// *Empire pointers by slice INDEX, B's reload rebinds Victimville's old slot to
// Decoyland — so sending through a cached pointer would hand the gold to the
// WRONG realm. Re-finding by name sees Victimville is gone and aborts, leaving
// Decoyland's gold and the sender's gold untouched.
func TestSendTradeDealVanishedRecipientConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 100_000_000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		v := w.AddHuman("victim", "Victimville")
		v.Gold = 0
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = 0
	})

	fb := &hookSession{
		// pick (B) Victimville — the picker letters realms by world slot, and
		// Alethia holds (A) — offer 1000 gold (6), done (0), no request (0),
		// confirm (y), 2 days (Enter)
		fakeSession: fakeSession{keys: []rune("B61000\r00y\r")},
		marker:      "Send this trade deal",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("victim")) })
		},
	}
	sendTradeDeal(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Gold != 0 {
		t.Fatalf("Decoyland gold = %d, want 0 — a stale pointer sent gold to the wrong realm", d.Gold)
	}
	if a := committedEmpire(t, cfg, "alice"); a.Gold != 100_000_000 {
		t.Fatalf("sender gold = %d, want 100000000 — an aborted deal spent gold", a.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "no longer there") {
		t.Fatalf("node B should have aborted with the target-gone notice, got: %q", out)
	}
}

// TestAcceptTreatyDoubleAcceptIdempotent proves that two concurrent accepts of
// the same pending offer produce a single treaty, not a duplicate. Bob has
// offered Alice a Free Trade Agreement. Node B (Alice) reaches the accept
// confirmation; while B is at the prompt another node accepts the same offer
// (forming the treaty and consuming the offer). B then commits its own accept:
// re-resolving inside the transaction, AcceptTreaty finds the offer already
// consumed and forms no second treaty.
func TestAcceptTreatyDoubleAcceptIdempotent(t *testing.T) {
	const fta = "Free Trade Agreement"
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, nil)
	commitOnFile(t, cfg, func(w *game.World) {
		bob := w.AddHuman("bob", "Bobbington")
		w.ProposeTreaty(bob, w.FindByOwner("alice"), fta) // Bob offers Alice the FTA
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("y")}, // confirm the accept
		marker:      "Accept this treaty",
		hook: func() { // another node accepts the same offer first
			commitOnFile(t, cfg, func(w *game.World) {
				w.AcceptTreaty(w.FindByOwner("alice"), "Bobbington", fta)
			})
		},
	}
	negotiateWithType(fb, b, "Bobbington", fta)

	w := committedWorld(t, cfg)
	n := 0
	for _, tr := range w.Treaties {
		if tr.Type == fta {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("Free Trade Agreement count = %d, want 1 — the double-accept was not idempotent", n)
	}
}
