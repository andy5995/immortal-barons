package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A packet one board writes shows in the packet log under the same number the
// board that takes it in logs, with what became of it (#311).
func TestPacketLogTracesAPacketAcrossBoards(t *testing.T) {
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
	a.w.SendIPMessage(a.w.Empires[0], []string{"The Eclipse"}, false, "Hello.")
	a.run(t)
	// Kept back to deliver a second time, as a duplicate.
	files, _ := filepath.Glob(filepath.Join(a.outbound, "*"+PacketExt))
	var copies [][]byte
	for _, f := range files {
		data, _ := os.ReadFile(f)
		copies = append(copies, data)
	}
	if deliver(t, a, b) == 0 {
		t.Fatal("Nova Hub wrote nothing")
	}
	b.run(t)
	for i, data := range copies {
		os.WriteFile(filepath.Join(b.inbound, fmt.Sprintf("dup%d%s", i, PacketExt)), data, 0o644)
	}
	b.run(t)

	log, err := os.ReadFile(filepath.Join("data", PacketLogFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	_, rest, ok := strings.Cut(text, "out  #")
	for ok && !strings.Contains(strings.SplitN(rest, "\n", 2)[0], "to The Eclipse (1 messages") {
		_, rest, ok = strings.Cut(rest, "out  #")
	}
	if !ok {
		t.Fatalf("no outbound line for the message:\n%s", text)
	}
	seq, _, _ := strings.Cut(rest, " ")
	for _, fate := range []string{"applied", "deleted: already seen"} {
		want := "in   #" + seq + " from Nova Hub (1 messages, 1 time checks): " + fate + "\n"
		if !strings.Contains(text, want) {
			t.Errorf("packet log lacks %q:\n%s", want, text)
		}
	}
}

// The log is moved aside at its cap, so it never grows past twice that.
func TestPacketLogRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, PacketLogFile)
	os.WriteFile(path, make([]byte, packetLogMaxBytes), 0o644)
	AppendPacketLog(dir, []string{"fresh"}, time.Now())
	if info, err := os.Stat(path + ".1"); err != nil || info.Size() != packetLogMaxBytes {
		t.Fatalf("old log not moved aside: %v", err)
	}
	if data, _ := os.ReadFile(path); !strings.HasSuffix(string(data), "  fresh\n") || len(data) > 100 {
		t.Fatalf("new log = %q", data)
	}
}

// A board name is another board's text, so it cannot add a line of its own.
func TestPacketLogCannotBeForged(t *testing.T) {
	dir := t.TempDir()
	forged := game.Packet{Seq: 9, FromBoard: "Evil\n2026-01-01 00:00:00 UTC  in   #1 from Honest"}
	AppendPacketLog(dir, []string{inLine(forged, "applied")}, time.Now())
	AppendPlanetaryLog(dir, []string{"A packet from " + forged.FromBoard + " was refused."}, time.Now())
	for _, f := range []string{PacketLogFile, PlanetaryLogFile} {
		data, _ := os.ReadFile(filepath.Join(dir, f))
		if n := strings.Count(string(data), "\n"); n != 1 {
			t.Errorf("%s holds %d lines, want 1:\n%s", f, n, data)
		}
	}
}

// A line says what became of the packet, not what was meant to: a forward that
// destroyed the packet, or a move that failed, is logged as that (#311).
func TestPacketLogRecordsWhatHappened(t *testing.T) {
	dir := t.TempDir()
	w := game.NewWorld(game.DefaultConfig())
	w.Config.DataDir = dir
	w.Config.BoardID = "Hub BBS"
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Hub BBS", Hosts: []int{2}},
		{Number: 2, Name: "Leaf BBS"},
	}
	var result InboundResult
	p := game.Packet{FromBoard: "Leaf BBS", ToBoard: "Nowhere BBS", Seq: 4}
	path := filepath.Join(dir, "p.brp")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStagedPacket(w, &result, path, p, []byte("{}"), false); err != nil {
		t.Fatal(err)
	}
	if len(result.Log) != 1 || !strings.Contains(result.Log[0], "destroyed, not passed on") {
		t.Errorf("an unroutable forward logged %q", result.Log)
	}

	result = InboundResult{}
	gone := filepath.Join(dir, "missing.brp") // removing it fails
	p = game.Packet{FromBoard: "Leaf BBS", Seq: 0}
	if err := applyStagedPacket(w, &result, gone, p, nil, false); err == nil {
		t.Error("a failed delete was not raised")
	}
	if len(result.Log) != 1 || !strings.Contains(result.Log[0], "but the file stays in inbound") {
		t.Errorf("a failed delete logged %q", result.Log)
	}
}
