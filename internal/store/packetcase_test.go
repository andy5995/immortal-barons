package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestIsPacketFileIgnoresCase(t *testing.T) {
	for _, name := range []string{"L001-abc.brp", "L001-abc.BRP", "L001-abc.Brp", "ABC.bRp"} {
		if !IsPacketFile(name) {
			t.Errorf("IsPacketFile(%q) = false, want true", name)
		}
	}
	// The temporary name a half-written packet carries must stay invisible, or a
	// reader picks up an incomplete file (writePacketAtomic).
	for _, name := range []string{"mail.pkt", "notes.txt", "packet.brp.tmp", ".brp-1.tmp", "brp", ""} {
		if IsPacketFile(name) {
			t.Errorf("IsPacketFile(%q) = true, want false", name)
		}
	}
}

// TestUppercasePacketsAreRead is the bug in #179: FTN transport hands files
// over in upper case routinely, and an exact match against ".brp" left the file
// in the inbound directory unread and unreported — not applied, not refused,
// not counted as skipped, so nothing told the sysop anything was there.
func TestUppercasePacketsAreRead(t *testing.T) {
	inbound := t.TempDir()

	sender := game.DefaultConfig()
	sender.BoardID = "Nova Hub"
	from := game.NewWorldSeed(sender, 1)
	from.LastMaintDate = "2026-08-23"
	from.AddHuman("baron", "Ironhold")
	from.ExportScores()
	from.StampOutbox()
	if _, err := WriteOutbox(from, inbound, false); err != nil {
		t.Fatal(err)
	}

	// Rename what the transport delivered to upper case, as a mailer would.
	entries, err := os.ReadDir(inbound)
	if err != nil {
		t.Fatal(err)
	}
	shouted := 0
	for _, e := range entries {
		if !IsPacketFile(e.Name()) {
			continue
		}
		if err := os.Rename(filepath.Join(inbound, e.Name()),
			filepath.Join(inbound, strings.ToUpper(e.Name()))); err != nil {
			t.Fatal(err)
		}
		shouted++
	}
	if shouted != 1 {
		t.Fatalf("expected one packet to deliver, got %d", shouted)
	}

	receiver := game.DefaultConfig()
	receiver.BoardID = "The Eclipse"
	to := game.NewWorldSeed(receiver, 1)
	to.LastMaintDate = "2026-08-23"
	result, err := ReadInbound(to, inbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 1 {
		t.Fatalf("an uppercase packet was not applied: %+v", result)
	}
	if len(to.RemoteBoards) != 1 || to.RemoteBoards[0].BoardID != "Nova Hub" {
		t.Errorf("the packet's scores did not land: %+v", to.RemoteBoards)
	}
	// A packet that was read is consumed, so it cannot be applied twice.
	left, err := os.ReadDir(inbound)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range left {
		if IsPacketFile(e.Name()) {
			t.Errorf("%s was left in the inbound directory", e.Name())
		}
	}
}
