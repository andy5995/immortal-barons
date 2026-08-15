package game

import (
	"strings"
	"testing"
)

// The three sysop reports read what inbound packets recorded. The layout of
// BBSINFO is held to the original's own captured file.
func TestLeagueReports(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "Alpha BBS"
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-15"
	mine := w.AddHuman("me", "Ironhold")
	mine.Regions.Urban = 100
	mine.syncLand()
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Alpha BBS"}, {Number: 2, Name: "Bravo BBS"}}
	w.ImportBoard(RemoteBoard{BoardID: "Bravo BBS", Scores: []RemoteScore{{Empire: "Redlands", NetWorth: 4860, Score: 2130}}})
	w.LastPacketFrom = map[string]string{"Bravo BBS": "08/15/2026 09:34:36"}
	w.BoardVersion = map[string]string{"Bravo BBS": "0.0.5"}

	info := w.BBSInfoReport()
	for _, want := range []string{"### BBS Name", "Last Recon", "BRE Version", " 2) Bravo BBS", "08/15/2026 09:34:36", "v0.0.5"} {
		if !strings.Contains(info, want) {
			t.Errorf("BBSINFO missing %q:\n%s", want, info)
		}
	}
	if strings.Contains(info, "Alpha BBS") {
		t.Errorf("BBSINFO lists this board itself:\n%s", info)
	}

	last := w.LastPacketReport()
	if !strings.Contains(last, "Bravo BBS") || !strings.Contains(last, "08/15/2026 09:34:36") {
		t.Errorf("LASTPACKET missing the processed stamp:\n%s", last)
	}

	// The player list spans every board — this one's realms and the others'.
	players := w.PlayerListReport()
	for _, want := range []string{"Ironhold", "Redlands", "Alpha BBS", "Bravo BBS"} {
		if !strings.Contains(players, want) {
			t.Errorf("PLAYERLIST missing %q:\n%s", want, players)
		}
	}
}

// A board that has never written shows so plainly rather than being guessed at.
func TestBBSInfoSaysWhenABoardHasNeverWritten(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "Alpha BBS"
	w := NewWorldSeed(cfg, 1)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Alpha BBS"}, {Number: 7, Name: "Silent BBS"}}
	info := w.BBSInfoReport()
	if !strings.Contains(info, "never") || !strings.Contains(info, "unknown") {
		t.Errorf("a silent board should read never/unknown:\n%s", info)
	}
	if !strings.Contains(info, " 7) Silent BBS") {
		t.Errorf("the roster's own node number should be used:\n%s", info)
	}
}

// An accepted packet records its sender's version and the time it landed; a
// replayed one must not, or a board that has gone quiet still looks alive.
func TestOnlyAcceptedPacketsCountAsContact(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "Alpha BBS"
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-15"
	p := Packet{FromBoard: "Bravo BBS", Date: "2026-08-15", Seq: 4, Version: "0.0.5",
		Scores: []RemoteScore{{Empire: "Redlands"}}}
	w.ApplyPacket(p)
	first := w.LastPacketFrom["Bravo BBS"]
	if first == "" || w.BoardVersion["Bravo BBS"] != "0.0.5" {
		t.Fatalf("an accepted packet recorded nothing: %q / %q", first, w.BoardVersion["Bravo BBS"])
	}
	w.LastPacketFrom["Bravo BBS"] = "01/01/2000 00:00:00" // pretend time passed
	w.ApplyPacket(p)                                      // the same packet again
	if w.LastPacketFrom["Bravo BBS"] != "01/01/2000 00:00:00" {
		t.Error("a replayed packet refreshed the contact time, so a silent board would look alive")
	}
}

// The Coordinator can require a version of the whole league. A board below it
// has its packets refused, and BBSINFO names it.
func TestMinimumVersionGatesPackets(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID, cfg.MinBoardVersion = true, "Alpha BBS", "0.0.5"
	w := NewWorldSeed(cfg, 1)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Alpha BBS"}, {Number: 2, Name: "Bravo BBS"}}

	// Too old: refused, and nothing it carried is applied.
	w.ApplyPacket(Packet{FromBoard: "Bravo BBS", Seq: 1, Version: "0.0.4",
		Scores: []RemoteScore{{Empire: "Redlands"}}})
	if len(w.RemoteBoards) != 0 {
		t.Error("a packet from a board below the minimum was applied")
	}
	if news := strings.Join(w.NewsToday, "\n"); !strings.Contains(news, "requires v0.0.5") {
		t.Errorf("the refusal should say why:\n%s", news)
	}
	// A board that states no version cannot prove it meets the bar.
	w.ApplyPacket(Packet{FromBoard: "Bravo BBS", Seq: 2, Scores: []RemoteScore{{Empire: "Redlands"}}})
	if len(w.RemoteBoards) != 0 {
		t.Error("a packet with no version at all was applied under a version requirement")
	}
	// At or above it, business as usual.
	w.ApplyPacket(Packet{FromBoard: "Bravo BBS", Seq: 3, Version: "0.0.5",
		Scores: []RemoteScore{{Empire: "Redlands"}}})
	if len(w.RemoteBoards) != 1 {
		t.Fatal("a board meeting the minimum was refused")
	}
	w.BoardVersion["Bravo BBS"] = "0.0.4" // as if it downgraded
	if info := w.BBSInfoReport(); !strings.Contains(info, "below v0.0.5") {
		t.Errorf("BBSINFO should name the board holding the league up:\n%s", info)
	}
}

func TestVersionAtLeast(t *testing.T) {
	for _, c := range []struct {
		have, want string
		ok         bool
	}{
		{"0.0.5", "0.0.5", true}, {"0.0.6", "0.0.5", true}, {"0.1.0", "0.0.9", true},
		{"1.0.0", "0.9.9", true}, {"0.0.4", "0.0.5", false}, {"0.1", "0.1.0", true},
		{"0.1", "0.1.1", false}, {"", "0.0.1", false}, {"garbage", "0.0.1", false},
	} {
		if got := versionAtLeast(c.have, c.want); got != c.ok {
			t.Errorf("versionAtLeast(%q, %q) = %v, want %v", c.have, c.want, got, c.ok)
		}
	}
}

// Every originating packet must state this board's version, not just the score
// export: the receiving board tests the version on everything that arrives, so a
// packet type that omitted it was refused by any league with a requirement set,
// however current the sending board actually was.
func TestEveryOutboundPacketStatesItsVersion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.IBBS = true
	w.Config.BoardID = "Alpha BBS"
	// A league that requires exactly what this board runs: every packet it sends
	// must clear its own bar.
	w.Config.MinBoardVersion = Version
	// One packet from each shape that leaves a board: a plain reply, a mail
	// carrier, and a broadcast with no addressee.
	w.Outbox = append(w.Outbox,
		Packet{FromBoard: w.Config.BoardID, ToBoard: "Bravo BBS"},
		Packet{FromBoard: w.Config.BoardID, ToBoard: "Bravo BBS", IPMessages: []IPMessage{{Body: "hi"}}},
		Packet{FromBoard: w.Config.BoardID},
	)
	w.StampOutbox()
	for i, p := range w.Outbox {
		if p.Version != Version {
			t.Errorf("packet %d states version %q, want %q", i, p.Version, Version)
		}
		if !w.BoardMeetsMinVersion(p.Version) {
			t.Errorf("packet %d would be refused by a league requiring the current version", i)
		}
	}
}
