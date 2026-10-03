package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A league round trip applies each packet exactly once: the packets a league
// exchanges are refused on replay by HighSeq, which is saved in the ledger file
// and not in world.json.
func TestALeagueRoundTripAppliesEachPacketOnce(t *testing.T) {
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
	if err := Save(b.w, b.w.Config); err != nil {
		t.Fatal(err)
	}
	world, err := os.ReadFile(filepath.Join(b.w.Config.DataDir, "world.json"))
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(world, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"OutSeq", "HighSeq", "SeenPackets"} {
		if _, ok := keys[k]; ok {
			t.Errorf("world.json holds %s; it belongs in %s", k, LedgerFile)
		}
	}
	loaded, err := Load(b.w.Config)
	if err != nil {
		t.Fatal(err)
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

// A world.json written before the ledger had a file of its own still holds
// OutSeq and HighSeq. Loading it takes them across, and the next save writes
// them to the ledger file.
func TestTheLedgerMigratesOutOfWorldJSON(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	if err := Save(game.NewWorldSeed(cfg, 1), cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.DataDir, "world.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	keys["OutSeq"] = json.RawMessage("41")
	keys["HighSeq"] = json.RawMessage(`{"Far BBS": 9}`)
	if data, err = json.Marshal(keys); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(cfg.DataDir, LedgerFile)); err != nil {
		t.Fatal(err)
	}

	w, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w.OutSeq != 41 || w.HighSeq["Far BBS"] != 9 {
		t.Fatalf("migrated OutSeq %d, HighSeq %v; want 41 and Far BBS at 9", w.OutSeq, w.HighSeq)
	}
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, LedgerFile)); err != nil {
		t.Errorf("the save wrote no ledger file: %v", err)
	}
	if w, err = Load(cfg); err != nil {
		t.Fatal(err)
	}
	if w.OutSeq != 41 || w.HighSeq["Far BBS"] != 9 {
		t.Errorf("after the save OutSeq %d, HighSeq %v; want 41 and Far BBS at 9", w.OutSeq, w.HighSeq)
	}
}

// A reset starts the game over and leaves the ledger alone: the board goes on
// numbering its packets where it was, and still refuses old ones.
func TestAResetKeepsTheLedger(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)
	w.OutSeq = 41
	w.HighSeq = map[string]uint64{"Far BBS": 9}
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	w.Reset()
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	if fresh, err := NewGame(cfg); err != nil {
		t.Fatal(err)
	} else if fresh.OutSeq != 41 || fresh.HighSeq["Far BBS"] != 9 {
		t.Errorf("a new game on this board has OutSeq %d, HighSeq %v; want 41 and Far BBS at 9",
			fresh.OutSeq, fresh.HighSeq)
	}
	if w, err := Load(cfg); err != nil {
		t.Fatal(err)
	} else if w.OutSeq != 41 || w.HighSeq["Far BBS"] != 9 {
		t.Errorf("after a reset OutSeq %d, HighSeq %v; want 41 and Far BBS at 9", w.OutSeq, w.HighSeq)
	}
}

// A packet with no sequence number cannot be checked for a replay, so it is
// deleted unapplied and the sysop is told.
func TestAnUnnumberedPacketIsDeletedUnapplied(t *testing.T) {
	inbound := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	w := game.NewWorldSeed(cfg, 1)
	writePacket(t, inbound, "old", game.Packet{
		FromBoard: "Bravo BBS", ToBoard: "Alpha BBS",
		Scores: []game.RemoteScore{{Empire: "Apples", Land: 500}},
	})
	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 0 || len(w.RemoteBoards) != 0 {
		t.Errorf("applied %d, remote boards %v; want nothing applied", result.Applied, w.RemoteBoards)
	}
	if len(w.SysopNotices) != 1 {
		t.Errorf("sysop notices %q, want the refusal", w.SysopNotices)
	}
	if left := packetFiles(t, inbound); len(left) != 0 {
		t.Errorf("the packet was left in inbound: %v", left)
	}
}

// A ledger that exists but cannot be read stops the load: numbering this
// board's packets from 1 again would have every other board drop them.
func TestAnUnreadableLedgerStopsTheLoad(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	if err := Save(game.NewWorldSeed(cfg, 1), cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, LedgerFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfg); err == nil || !strings.Contains(err.Error(), "packet ledger") {
		t.Errorf("Load gave %v, want the ledger error", err)
	}
	if _, err := NewGame(cfg); err == nil {
		t.Error("NewGame started over an unreadable ledger")
	}
}
