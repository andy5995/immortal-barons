package ftn

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A claim moves packets with store.MoveFile, which falls back to a copy when
// a rename cannot cross drives (GameOutbound on d:, the spool under a data
// directory on X:, as a Windows Coordinator had it). When a move fails anyway,
// the claim says which packet, leaves it where it was, and takes the batch
// folder it had just made away again rather than leaving an empty one in the
// spool for a later run to find.
func TestClaimThatCannotMoveLeavesNoEmptyBatch(t *testing.T) {
	board := game.DefaultConfig()
	board.DataDir = t.TempDir()
	board.OutboundDir = t.TempDir()
	packet := filepath.Join(board.OutboundDir, "L777-1-0000000001.brp")
	if err := os.WriteFile(packet, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(board.DataDir, "spool-out")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	moveFile = func(string, string) error { return errors.New("cannot move the file to a different disk drive") }
	t.Cleanup(func() { moveFile = store.MoveFile })

	if _, err := claimOutboundBatch(root, board); err == nil {
		t.Fatal("a claim whose move failed reported success")
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("the failed claim left %d entries in the spool", len(entries))
	}
	if _, err := os.Stat(packet); err != nil {
		t.Errorf("the packet should still be in the outbound directory: %v", err)
	}

	moveFile = store.MoveFile
	batch, err := claimOutboundBatch(root, board)
	if err != nil || batch == "" {
		t.Fatalf("a claim that can move should succeed: %q, %v", batch, err)
	}
	if _, err := os.Stat(filepath.Join(batch, "packet-000000"+store.PacketExt)); err != nil {
		t.Errorf("the packet did not reach the batch: %v", err)
	}
}
