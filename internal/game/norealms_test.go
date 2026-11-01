package game

import "testing"

// A board left with no living human realm says so, and the board that reads it
// clears that planet's roster, releases the lock it held and drops the reports
// on its realms (#314). Before, such a board sent nothing at all.
func TestABoardWithNoRealmsClearsItsRoster(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.DupeChecking = true, true
	far := NewWorldSeed(cfg, 1)
	far.Empires = nil
	far.Config.BoardID = "there"
	fe := far.AddHuman("alice", "Far Reach")
	fe.Founded = "" // unknown founding: the lock falls on the realm here

	here := NewWorldSeed(cfg, 2)
	here.Config.BoardID = "here"
	he := here.AddHuman("alice", "Near Reach")

	export := func() Packet {
		t.Helper()
		far.Outbox = nil
		far.ExportScores()
		if len(far.Outbox) != 1 {
			t.Fatalf("ExportScores queued %d packets, want 1", len(far.Outbox))
		}
		return far.Outbox[0]
	}
	here.receive(export())
	here.SpyDatabase = []SpyEntry{{SpyReport: SpyReport{Board: "there", Empire: "Far Reach"}}}
	if !here.DupeLocked(he) || len(here.ScoredBoards()) != 1 {
		t.Fatalf("setup: locked %v, scored boards %v", here.DupeLocked(he), here.ScoredBoards())
	}

	far.RemoveEmpire(fe)
	p := export()
	if !p.NoRealms || len(p.Scores) != 0 {
		t.Fatalf("an empty board sent NoRealms=%v with %d scores", p.NoRealms, len(p.Scores))
	}
	here.receive(p)
	if here.DupeLocked(he) {
		t.Error("the lock survived the other board reporting it has no realms")
	}
	if got := here.ScoredBoards(); len(got) != 0 {
		t.Errorf("a planet with no realms is still offered as a target: %v", got)
	}
	if len(here.SpyDatabase) != 0 {
		t.Errorf("reports on a planet with no realms survived: %+v", here.SpyDatabase)
	}
}

// A NoRealms packet that arrives behind a later one from its board is stale
// state like any other, and must not clear the newer roster (#311).
func TestALateNoRealmsPacketKeepsTheNewerRoster(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "here"
	w := NewWorldSeed(cfg, 1)
	w.ApplyPacket(Packet{FromBoard: "there", Seq: 9, Scores: []RemoteScore{{Empire: "Still Here"}}})
	w.ApplyPacket(Packet{FromBoard: "there", Seq: 8, NoRealms: true})
	if got := w.ScoredBoards(); len(got) != 1 {
		t.Errorf("a late NoRealms packet cleared the newer roster: %v", got)
	}
}
