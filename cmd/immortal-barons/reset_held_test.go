package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

// A reset archives packets held for a newer protocol along with inbound and
// outbound (#261): otherwise the upgrade that usually follows a season boundary
// releases last season's packets into the new world.
func TestResetArchivesHeldPackets(t *testing.T) {
	cfg := leagueBoard(t)
	held := filepath.Join(cfg.DataDir, store.HeldDir)
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	pkt := "L900-2-0000000000007-1.brp"
	if err := os.WriteFile(filepath.Join(held, pkt), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	preparePacketDirs(cfg)

	if _, err := os.Stat(filepath.Join(held, pkt)); err == nil {
		t.Fatalf("%s is still in %s after the reset", pkt, held)
	}
	archived, _ := filepath.Glob(filepath.Join(held, "reset-*", pkt))
	if len(archived) != 1 {
		t.Fatalf("%s was not archived under %s/reset-*", pkt, held)
	}

	// The next planetary run must not bring it back into inbound.
	if err := runPlanetary(cfg, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Inbound(), pkt)); err == nil {
		t.Errorf("the archived held packet was released into inbound")
	}
	if left, _ := filepath.Glob(filepath.Join(held, "reset-*", pkt)); len(left) != 1 {
		t.Errorf("the archived held packet left its archive")
	}
}
