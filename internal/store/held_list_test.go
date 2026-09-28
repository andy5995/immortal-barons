package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// HeldPackets says why each file is held, using the checks that held it, and
// reads a file it cannot decode as unreadable rather than failing the list.
func TestHeldPacketsSaysWhy(t *testing.T) {
	inbound, outbound := t.TempDir(), t.TempDir()
	cfg := game.DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	cfg.LostForcesDays = 3
	w := game.NewWorldSeed(cfg, 1)
	w.GameDay = 10
	w.InFlight = []game.InFlightStrike{{ID: 1, Kind: "special", TargetBoard: "Far BBS", LaunchedDay: 9}}

	writePacket(t, inbound, "far-1", game.Packet{FromBoard: "Far BBS", Seq: 1, Protocol: game.Protocol + 1})
	if _, err := RunPlanetary(w, inbound, outbound, false); err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(heldPath(cfg.DataDir), "junk"+PacketExt), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := HeldPackets(w)
	if err != nil {
		t.Fatal(err)
	}
	got := map[HeldReason]HeldPacket{}
	for _, h := range list {
		got[h.Reason] = h
	}
	if len(list) != 2 {
		t.Fatalf("HeldPackets listed %d files, want 2: %+v", len(list), list)
	}
	p, ok := got[HeldProtocol]
	if !ok || p.FromBoard != "Far BBS" || !p.PausesLostForces || p.Expires.Sub(p.Arrived) != HeldMaxAge {
		t.Errorf("protocol hold listed as %+v", p)
	}
	if u, ok := got[HeldUnreadable]; !ok || u.FromBoard != "" {
		t.Errorf("unreadable file listed as %+v", u)
	}
}
