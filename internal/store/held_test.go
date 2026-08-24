package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func writeHeldTestPacket(t *testing.T, dir, name string, p game.Packet) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A packet this build cannot read is set aside, not refused: it may carry a
// roster update, mail, or a returning strike that is perfectly good once the
// boards are level, and the sender has no way to know to send it again.
func TestAPacketForAnotherProtocolIsHeldAndLaterReleased(t *testing.T) {
	data, inbound := t.TempDir(), t.TempDir()
	future := game.Packet{FromBoard: "Alpha BBS", Protocol: game.Protocol + 1}
	path := writeHeldTestPacket(t, inbound, "future"+PacketExt, future)

	if err := holdPacket(data, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the packet was left in inbound, so it would be re-read every run")
	}
	held := filepath.Join(data, HeldDir, "future"+PacketExt)
	if _, err := os.Stat(held); err != nil {
		t.Fatalf("the packet was not preserved: %v", err)
	}

	// Still unreadable: it stays put rather than being applied.
	moved, err := releaseHeld(data, inbound)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Errorf("released %d packets this build still cannot read", moved)
	}

	// Rewrite it as something this build speaks -- standing in for the upgrade.
	writeHeldTestPacket(t, filepath.Join(data, HeldDir), "future"+PacketExt,
		game.Packet{FromBoard: "Alpha BBS", Protocol: game.Protocol})
	moved, err = releaseHeld(data, inbound)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("released %d packets, want 1", moved)
	}
	// Back to INBOUND, not applied directly: it must go through the league,
	// duplicate, addressing and signature checks like any other packet.
	if _, err := os.Stat(filepath.Join(inbound, "future"+PacketExt)); err != nil {
		t.Errorf("the released packet did not return to inbound: %v", err)
	}
}

// The v0.0.7 case: every packet already in flight was written before the field
// existed. Holding those would strand the league on the release meant to fix it.
func TestAPacketWithNoProtocolIsNotHeld(t *testing.T) {
	data, inbound := t.TempDir(), t.TempDir()
	writeHeldTestPacket(t, inbound, "legacy"+PacketExt, game.Packet{FromBoard: "Alpha BBS"})
	if !game.SpeaksOurProtocol(0) {
		t.Fatal("a packet with no protocol must be readable, or existing traffic stops dead")
	}
	if moved, _ := releaseHeld(data, inbound); moved != 0 {
		t.Errorf("releaseHeld touched %d files with no held directory", moved)
	}
}

func TestReleaseHeldIsQuietWithNoHeldDirectory(t *testing.T) {
	moved, err := releaseHeld(t.TempDir(), t.TempDir())
	if err != nil || moved != 0 {
		t.Errorf("got (%d, %v), want (0, nil)", moved, err)
	}
}
