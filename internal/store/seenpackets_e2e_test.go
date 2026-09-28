package store

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A league round trip applies each packet exactly once, with SeenPackets no
// longer growing per packet: the numbered packets a league exchanges are
// refused on replay by HighSeq alone, across a save and a reload, and a save
// carrying the entries older builds wrote loses them on load.
func TestALeagueRoundTripAppliesEachPacketOnceWithoutGrowingSeenPackets(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub", City: "Brisbane"},
		{Number: 2, Name: "The Eclipse", City: "Sydney"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	b.w.Config.DataDir = filepath.Join(dir, "b", "data")
	if err := os.MkdirAll(b.w.Config.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a.w.SendIPMessage(a.w.Empires[0], []string{"The Eclipse"}, false, "Once only.")
	a.run(t)
	// Keep a copy of what Nova Hub sent, to drop in again as a replay.
	sent := map[string][]byte{}
	entries, err := os.ReadDir(a.outbound)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(a.outbound, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sent[e.Name()] = data
	}
	if n := deliver(t, a, b); n == 0 {
		t.Fatal("Nova Hub wrote no packets")
	}
	b.run(t)

	if got := len(b.w.Empires[0].Mail); got != 1 {
		t.Fatalf("The Eclipse received %d messages, want 1", got)
	}
	high := b.w.HighSeq["Nova Hub"]
	if high == 0 {
		t.Fatalf("no sequence recorded for Nova Hub: %v", b.w.HighSeq)
	}
	if len(b.w.SeenPackets) != 0 {
		t.Errorf("numbered packets were stored in SeenPackets: %v", b.w.SeenPackets)
	}

	// An entry an older build wrote for one of those packets, and a content
	// fingerprint it still needs, both saved to disk.
	var num [8]byte
	binary.BigEndian.PutUint64(num[:], high)
	legacy := fmt.Sprintf("Nova Hub#%x", num)
	fingerprint := "Old BBS#" + fmt.Sprintf("%064x", 1)
	b.w.SeenPackets = map[string]bool{legacy: true, fingerprint: true}
	if err := Save(b.w, b.w.Config); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(b.w.Config)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SeenPackets[legacy] || !loaded.SeenPackets[fingerprint] || len(loaded.SeenPackets) != 1 {
		t.Errorf("after load SeenPackets = %v, want the fingerprint alone", loaded.SeenPackets)
	}
	if loaded.HighSeq["Nova Hub"] != high {
		t.Errorf("HighSeq did not survive the save: %v", loaded.HighSeq)
	}

	// The same packets dropped in again are all refused, and nothing is
	// applied twice.
	b.w = loaded
	for name, data := range sent {
		if err := os.WriteFile(filepath.Join(b.inbound, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run, err := RunPlanetary(b.w, b.inbound, b.outbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.AlreadySeen != len(sent) || run.Applied != 0 {
		t.Errorf("replay: applied %d, already seen %d; want 0 and %d", run.Applied, run.AlreadySeen, len(sent))
	}
	if got := len(b.w.Empires[0].Mail); got != 1 {
		t.Errorf("after the replay The Eclipse has %d messages, want 1", got)
	}
}
