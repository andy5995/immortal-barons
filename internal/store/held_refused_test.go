package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A packet refused because its signature did not match the roster key is HELD,
// not destroyed (#185). ApplyPacket refuses before it records anything, so the
// packet was never applied and never marked seen — holding it means a board
// that later gains the right key applies the backlog by itself.
func TestRefusedPacketIsHeldNotDestroyed(t *testing.T) {
	dir := t.TempDir()
	held := filepath.Join(dir, HeldDir)
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "in.brp")
	if err := os.WriteFile(src, []byte(`{"Protocol":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := holdPacket(dir, src); err != nil {
		t.Fatalf("holdPacket: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("the packet was left in the inbound directory")
	}
	if _, err := os.Stat(filepath.Join(held, "in.brp")); err != nil {
		t.Errorf("the packet was not held: %v", err)
	}
}

// The wait is bounded: a board that is simply forging would otherwise fill the
// held directory for the life of the league, since its packets never verify.
func TestHeldPacketsAgeOut(t *testing.T) {
	dir := t.TempDir()
	inbound := filepath.Join(dir, "inbound")
	if err := os.MkdirAll(inbound, 0o755); err != nil {
		t.Fatal(err)
	}
	held := heldPath(dir)
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(held, "fresh.brp")
	stale := filepath.Join(held, "stale.brp")
	// The current protocol, so releaseHeld does not skip it for the OTHER
	// reason a packet stays held.
	body := []byte(fmt.Sprintf(`{"Protocol":%d}`, game.Protocol))
	for _, f := range []string{fresh, stale} {
		if err := os.WriteFile(f, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-HeldMaxAge - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if _, err := releaseHeld(dir, inbound); err != nil {
		t.Fatalf("releaseHeld: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a packet past HeldMaxAge was kept")
	}
	if _, err := os.Stat(filepath.Join(inbound, "fresh.brp")); err != nil {
		t.Errorf("a packet within HeldMaxAge was not released: %v", err)
	}
}
