package game

import "testing"

func TestImportBoardAppendsNewBoard(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-01",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 50000, Land: 120}},
	})

	if len(w.RemoteBoards) != 1 {
		t.Fatalf("got %d remote boards, want 1", len(w.RemoteBoards))
	}
	if w.RemoteBoards[0].BoardID != "wildside" {
		t.Fatalf("got BoardID %q, want wildside", w.RemoteBoards[0].BoardID)
	}
}

func TestImportBoardReplacesSameBoardID(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-01",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 50000, Land: 120}},
	})
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-03",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 90000, Land: 150}},
	})

	if len(w.RemoteBoards) != 1 {
		t.Fatalf("got %d remote boards, want 1 (should replace, not duplicate)", len(w.RemoteBoards))
	}
	got := w.RemoteBoards[0]
	if got.Date != "2026-07-03" || got.Scores[0].NetWorth != 90000 {
		t.Fatalf("got %+v, want updated board with Date 2026-07-03 and NetWorth 90000", got)
	}
}

func TestImportBoardDifferentBoardIDsBothKept(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{BoardID: "wildside", Date: "2026-07-01"})
	w.ImportBoard(RemoteBoard{BoardID: "otherboard", Date: "2026-07-02"})

	if len(w.RemoteBoards) != 2 {
		t.Fatalf("got %d remote boards, want 2", len(w.RemoteBoards))
	}
}

func TestGroupAttackRoundTrip(t *testing.T) {
	// Board A assembles a group attack (leader + ally) against an empire on
	// board B; it departs, becomes an outbound packet, and board B resolves it.
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := NewWorldSeed(cfgA, 1)
	leader := wA.AddHuman("leader", "Alpha")
	ally := wA.AddHuman("ally", "Ally")

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Regions = RegionMix{Coastal: 100}
	target.syncLand()
	target.Troopers, target.Turrets, target.Tanks = 0, 0, 0 // defenseless

	leader.Gold, ally.Gold = 1_000_000, 1_000_000 // fund the pool
	ga, err := wA.CreateGroupAttack(leader, "boardB", "Victim", wA.GameDay+1, 100_000)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := wA.JoinGroupAttack(ally, ga.ID, 50_000); err != nil {
		t.Fatalf("join: %v", err)
	}

	wA.GameDay++ // reach departure day
	wA.LaunchDueGroupAttacks()
	if len(wA.Outbox) != 1 || len(wA.Outbox[0].Attacks) != 1 {
		t.Fatalf("expected one outbound attack, got %+v", wA.Outbox)
	}
	// 150,000 gold pooled / GroupAttackGoldPerOffense (500) = 300 offense.
	if got := wA.Outbox[0].Attacks[0].Offense; got != 300 {
		t.Errorf("combined offense: want 300, got %d", got)
	}
	if len(wA.GroupAttacks) != 0 {
		t.Errorf("departed attack should be removed from the pending list")
	}

	result := wB.ApplyPacket(wA.Outbox[0])
	if target.Land >= 100 {
		t.Errorf("target should have lost land, still has %d", target.Land)
	}
	if len(result.Results) != 1 || !result.Results[0].Won {
		t.Errorf("attack should have won: %+v", result.Results)
	}
	if result.ToBoard != "boardA" {
		t.Errorf("result should route back to boardA, got %q", result.ToBoard)
	}
}

func TestJoinDepartedAttackFails(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("e", "E")
	e.Gold = 10_000
	ga, _ := w.CreateGroupAttack(e, "boardB", "", w.GameDay, 1000) // departs today
	if err := w.JoinGroupAttack(e, ga.ID, 500); err != ErrDeparted {
		t.Errorf("joining a departed attack should fail with ErrDeparted, got %v", err)
	}
	if err := w.JoinGroupAttack(e, 999, 500); err != ErrNoAttack {
		t.Errorf("unknown id should fail with ErrNoAttack, got %v", err)
	}
}

func TestBBSCoordinatorElection(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Bravo")
	c := w.AddHuman("c", "Charlie")

	if w.BBSCoordinator() != nil {
		t.Error("no votes cast yet -> no coordinator")
	}
	w.VoteCoordinator(a, "b") // Bravo: 1
	w.VoteCoordinator(c, "b") // Bravo: 2
	w.VoteCoordinator(b, "a") // Alpha: 1
	if co := w.BBSCoordinator(); co != b {
		t.Errorf("Bravo has the most votes and should be coordinator, got %v", co)
	}
	// Changing a vote re-elects.
	w.VoteCoordinator(c, "a") // now Alpha 2, Bravo 1
	if co := w.BBSCoordinator(); co != a {
		t.Errorf("after re-vote Alpha should be coordinator, got %v", co)
	}
}

func TestLeagueConfigOnlyFromCoordinator(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}

	// Coordinator board (Alpha = node #1) authors the league config.
	cfgA := DefaultConfig()
	cfgA.BoardID, cfgA.GameLength, cfgA.TurnsPerDay = "AlphaBBS", 42, 15
	wA := NewWorldSeed(cfgA, 1)
	wA.LeagueNodes = roster
	if !wA.IsLeagueCoordinator() {
		t.Fatal("Alpha (node #1) should be the League Coordinator")
	}
	wA.ExportLeagueConfig()
	pkt := wA.Outbox[0]

	// Member board (Bravo) adopts the coordinator's config.
	cfgB := DefaultConfig()
	cfgB.BoardID, cfgB.GameLength, cfgB.TurnsPerDay = "BravoBBS", 0, 10
	wB := NewWorldSeed(cfgB, 1)
	wB.LeagueNodes = roster
	wB.ApplyPacket(pkt)
	if wB.Config.GameLength != 42 || wB.Config.TurnsPerDay != 15 {
		t.Errorf("member should adopt LC config, got length=%d turns=%d", wB.Config.GameLength, wB.Config.TurnsPerDay)
	}

	// A config packet from a NON-coordinator board must be ignored.
	cfgC := DefaultConfig()
	cfgC.BoardID, cfgC.GameLength = "CharlieBBS", 5
	wC := NewWorldSeed(cfgC, 1)
	wC.LeagueNodes = roster
	wC.ApplyPacket(Packet{FromBoard: "BravoBBS", LeagueConfig: &LeagueConfig{GameLength: 999}})
	if wC.Config.GameLength != 5 {
		t.Errorf("config from a non-coordinator board must be ignored, got %d", wC.Config.GameLength)
	}
}
