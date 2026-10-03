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
	maxAge := game.DefaultConfig().HeldMaxAge()
	old := time.Now().Add(-maxAge - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if _, err := releaseHeld(dir, inbound, maxAge); err != nil {
		t.Fatalf("releaseHeld: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a packet past its age was kept")
	}
	if _, err := os.Stat(filepath.Join(inbound, "fresh.brp")); err != nil {
		t.Errorf("a packet within its age was not released: %v", err)
	}
}

// HeldPacketDays in bbs.cfg sets how long a held packet waits (default 14); a value
// that is not a positive number of days leaves the default and is warned about.
func TestHeldPacketDaysComesFromBBSCfg(t *testing.T) {
	if got := game.DefaultConfig().HeldMaxAge(); got != 14*24*time.Hour {
		t.Errorf("default held age = %v, want 14 days", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte("HeldPacketDays 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := game.DefaultConfig()
	if err := LoadBoardConfig(dir, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.HeldMaxAge() != 3*24*time.Hour {
		t.Errorf("HeldPacketDays 3 gave %v", cfg.HeldMaxAge())
	}
	for _, bad := range []string{"0", "-2", "61", "soon"} {
		if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte("HeldPacketDays "+bad+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := game.DefaultConfig()
		LoadBoardConfig(dir, &cfg)
		if cfg.HeldMaxAge() != 14*24*time.Hour {
			t.Errorf("HeldPacketDays %s gave %v, want the default", bad, cfg.HeldMaxAge())
		}
		if len(BoardWarnings(dir, nil)) != 1 {
			t.Errorf("HeldPacketDays %s raised no warning", bad)
		}
	}
}
