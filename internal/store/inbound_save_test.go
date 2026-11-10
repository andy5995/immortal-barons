package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A run whose save fails must leave the inbound packets where they are: the
// world that applied them is gone, and deleting them left a trade deal neither
// delivered nor answered (#311).
func TestInboundPacketsOutliveAFailedSave(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{{Number: 1, Name: "Alpha"}, {Number: 2, Name: "Beta"}}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Alpha", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "Beta", roster)
	a.run(t)
	if deliver(t, a, b) == 0 {
		t.Fatal("Alpha sent nothing to deliver")
	}
	waiting := func() int {
		entries, err := os.ReadDir(b.inbound)
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	before := waiting()

	failed := errors.New("disk full")
	run, err := RunPlanetary(b.w, b.inbound, b.outbound, false, func() error { return failed })
	if !errors.Is(err, failed) {
		t.Fatalf("err = %v, want the save's error", err)
	}
	if run.Applied == 0 {
		t.Fatal("the run applied nothing, so it proves nothing about what a failed save keeps")
	}
	if got := waiting(); got != before {
		t.Fatalf("%d of %d inbound packets left after a failed save", got, before)
	}

	b.run(t)
	if got := waiting(); got != 0 {
		t.Fatalf("%d inbound packets left after a saved run", got)
	}
}

// A run that stops after writing its packets but before the last save must
// send the same packets again under the same numbers, and anything queued
// since goes out under a new one. Reusing the number for a packet with more in
// it had the far board delete that packet as already seen (#311).
func TestAStoppedRunResendsItsPacketsUnchanged(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{{Number: 1, Name: "Alpha"}, {Number: 2, Name: "Beta"}}
	a := newBoard(t, filepath.Join(dir, "a"), "Alpha", roster)
	a.w.Config.DataDir = filepath.Join(dir, "data")
	// A real send, since the fault was a sender adding to a numbered packet.
	queue := func(line string) {
		a.w.SendIPMessage(a.w.Empires[0], []string{"Beta"}, false, line)
	}
	sentFiles := func() map[string]string {
		files := map[string]string{}
		entries, err := os.ReadDir(a.outbound)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(a.outbound, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[e.Name()] = string(data)
		}
		return files
	}

	queue("first")
	// What the first save put on disk, and the stop: the save after the files
	// are written fails, so the world on disk still holds the packet.
	var onDisk []game.Packet
	stopped := errors.New("stopped")
	saves := 0
	_, err := SendOutbox(a.w, a.outbound, false, func() error {
		if saves++; saves > 1 {
			return stopped
		}
		if len(sentFiles()) != 0 {
			t.Fatal("a packet file was written before the world that numbered it was saved")
		}
		onDisk = append([]game.Packet(nil), a.w.Outgoing...)
		return nil
	})
	if !errors.Is(err, stopped) {
		t.Fatalf("err = %v, want the stop", err)
	}
	first := sentFiles()
	if len(first) != 1 || len(onDisk) != 1 {
		t.Fatalf("wrote %d files with %d packets saved, want 1 and 1", len(first), len(onDisk))
	}

	// The next run starts from the world on disk, with something new queued.
	a.w.Outgoing = onDisk
	queue("second")
	if _, err := SendOutbox(a.w, a.outbound, false, noSave); err != nil {
		t.Fatal(err)
	}
	after := sentFiles()
	if len(after) != 2 {
		t.Fatalf("%d files in outbound, want the first packet and a new one", len(after))
	}
	for name, data := range first {
		if after[name] != data {
			t.Fatalf("packet %s changed after it was first written", name)
		}
	}
}

// A save that renames the ledger and then fails on the world leaves the ledger
// one save ahead. Loading that pair must not count the failed save's packets as
// applied, or the next run deletes them unread (#311), and must keep the packet
// numbers it handed out.
func TestALedgerAheadOfItsWorldIsRolledBack(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	w, err := NewGame(cfg)
	if err != nil {
		t.Fatal(err)
	}
	from := func(seq uint64) game.Packet { return game.Packet{FromBoard: "Beta", Seq: seq} }
	w.SeenPacket(from(5))
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	world, err := os.ReadFile(worldPath(cfg))
	if err != nil {
		t.Fatal(err)
	}

	// The save that half-landed: the ledger is new, the world is put back.
	w.SeenPacket(from(6))
	w.SeenPacket(game.Packet{FromBoard: "Gamma", Seq: 1})
	sent := w.NextSeq()
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(worldPath(cfg), world, 0o644); err != nil {
		t.Fatal(err)
	}

	for round := range 2 {
		w, err = Load(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !w.IsPacketSeen(from(5)) {
			t.Fatalf("round %d: the packet the saved world applied is no longer seen", round)
		}
		if w.IsPacketSeen(from(6)) || w.IsPacketSeen(game.Packet{FromBoard: "Gamma", Seq: 1}) {
			t.Fatalf("round %d: a packet only the failed save applied counts as seen", round)
		}
		if w.OutSeq != sent {
			t.Fatalf("round %d: OutSeq = %d, want %d kept: that number may have gone out", round, w.OutSeq, sent)
		}
		// A good save from the rolled-back world must leave a pair that loads
		// the same way.
		if err := Save(w, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

// Once the batch is saved, a failure to send is a fault the run reports, not a
// failed run: an error here made every caller return before raising the alarm
// for faults the save had already marked as reported.
func TestAFailedSendAfterTheSaveIsReportedAsAFault(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{{Number: 1, Name: "Alpha"}, {Number: 2, Name: "Beta"}}
	a := newBoard(t, filepath.Join(dir, "a"), "Alpha", roster)
	a.w.Config.DataDir = filepath.Join(dir, "data")
	a.w.SendIPMessage(a.w.Empires[0], []string{"Beta"}, false, "hello")
	saves := 0
	run, err := RunPlanetary(a.w, a.inbound, a.outbound, false, func() error {
		if saves++; saves > 1 {
			return errors.New("disk full")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("err = %v, want the run to finish", err)
	}
	if run.Sent == 0 || len(run.NewFaults) == 0 {
		t.Fatalf("sent %d with faults %q, want packets sent and the failure reported", run.Sent, run.NewFaults)
	}
}
