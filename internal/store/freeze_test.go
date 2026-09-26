package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A frozen board's planetary run starts nothing — no scores, no probes — and
// writes only what the freeze allows; a reply its game produced waits in the
// Outbox for the thaw instead of going out under the old protocol.
func TestAFrozenRunWritesOnlyTheFreezesOwnPackets(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID, cfg.IBBS = "boardA", true
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("p", "Player")
	w.Frozen = true
	w.Outbox = []game.Packet{
		{FromBoard: "boardA", ToBoard: "boardB", Notice: "a reply that must wait"},
		{FromBoard: "boardA", Freeze: &game.LeagueFreeze{Serial: 1, Frozen: true}},
	}
	if _, err := RunPlanetary(w, in, out, false); err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	entries, _ := os.ReadDir(out)
	for _, ent := range entries {
		b, err := os.ReadFile(filepath.Join(out, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var p game.Packet
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		if !game.FrozenSendable(p) {
			t.Errorf("a frozen board wrote %s", p.PacketType())
		}
	}
	if len(entries) != 1 {
		t.Errorf("wrote %d packets, want only the freeze order", len(entries))
	}
	if len(w.Outbox) != 1 || w.Outbox[0].Notice == "" {
		t.Errorf("the waiting reply was not kept for the thaw: %+v", w.Outbox)
	}
}
