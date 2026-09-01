package game

import (
	"strings"
	"testing"
)

// renameNewsCount counts the planet-news lines announcing that `from` became
// `to`, so a test can tell "posted once" from "posted on every run" — the whole
// point of the bound in announceRemoteRenames.
func renameNewsCount(w *World, from, to string) int {
	n := 0
	for _, line := range w.NewsToday {
		if strings.Contains(line, from) && strings.Contains(line, to) && strings.Contains(line, "henceforth known as") {
			n++
		}
	}
	return n
}

// TestRemoteRenameIsAnnouncedOnce is the whole of #235: a realm renamed on
// another board is announced here, from the packet fact, and is not announced
// again by the imports that follow — RemoteScore.FormerName rides in every later
// export, so only the snapshot comparison stops the line repeating.
func TestRemoteRenameIsAnnouncedOnce(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	w.ImportBoard(RemoteBoard{BoardID: "Far BBS", Scores: []RemoteScore{
		{Empire: "Old", NetWorth: 100}, {Empire: "Bystander", NetWorth: 50},
	}})
	if got := renameNewsCount(w, "Old", "New"); got != 0 {
		t.Fatalf("news before the rename arrived = %d, want 0", got)
	}
	renamed := RemoteBoard{BoardID: "Far BBS", Scores: []RemoteScore{
		{Empire: "New", FormerName: "Old", NetWorth: 100}, {Empire: "Bystander", NetWorth: 50},
	}}
	w.ImportBoard(renamed)
	if got := renameNewsCount(w, "Old", "New"); got != 1 {
		t.Fatalf("news after the rename arrived = %d, want 1 (news: %v)", got, w.NewsToday)
	}
	// The board this happened on must be named: the reader knows this realm from
	// the interplanetary screens, not from next door.
	if !strings.Contains(strings.Join(w.NewsToday, "\n"), "Far BBS") {
		t.Errorf("the announcement does not name the planet: %v", w.NewsToday)
	}
	// Every later export carries FormerName too. This is the repeat the bound exists to stop.
	w.ImportBoard(renamed)
	w.ImportBoard(renamed)
	if got := renameNewsCount(w, "Old", "New"); got != 1 {
		t.Errorf("news after three imports = %d, want 1: the announcement repeats on every run", got)
	}
}

// TestRemoteRenameIsSilentWhenTheOldNameWasNeverSeen covers the board that joins
// the league, or misses a packet, after the rename: FormerName is on the wire
// but this planet never knew the realm by it, so there is nothing to connect.
func TestRemoteRenameIsSilentWhenTheOldNameWasNeverSeen(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	w.ImportBoard(RemoteBoard{BoardID: "Far BBS", Scores: []RemoteScore{{Empire: "Bystander"}}})
	w.ImportBoard(RemoteBoard{BoardID: "Far BBS", Scores: []RemoteScore{
		{Empire: "New", FormerName: "Old"}, {Empire: "Bystander"},
	}})
	if got := renameNewsCount(w, "Old", "New"); got != 0 {
		t.Errorf("news = %d, want 0: this planet never knew the realm as Old", got)
	}
}

// TestExportedScoresCarryTheFormerName is the wire half: the rename reaches
// another board's news through an actual exported packet applied by ApplyPacket,
// not through a hand-built RemoteBoard, and does not repeat on the next run.
func TestExportedScoresCarryTheFormerName(t *testing.T) {
	sender := NewWorldSeed(DefaultConfig(), 7)
	sender.Empires = nil
	sender.Config.BoardID = "Far BBS"
	e := sender.AddHuman("someone", "Old")
	e.Protection = 0

	receiver := NewWorldSeed(DefaultConfig(), 7)
	receiver.Config.BoardID = "Home BBS"

	export := func() Packet {
		t.Helper()
		sender.Outbox = nil
		sender.ExportScores()
		if len(sender.Outbox) != 1 {
			t.Fatalf("ExportScores queued %d packets, want 1", len(sender.Outbox))
		}
		return sender.Outbox[0]
	}

	receiver.ApplyPacket(export()) // the realm as "Old"

	if err := sender.RenameEmpire(e, "New"); err != nil {
		t.Fatalf("RenameEmpire: %v", err)
	}
	p := export()
	if len(p.Scores) != 1 || p.Scores[0].Empire != "New" || p.Scores[0].FormerName != "Old" {
		t.Fatalf("exported scores = %+v; want New, formerly Old", p.Scores)
	}
	receiver.ApplyPacket(p)
	if got := renameNewsCount(receiver, "Old", "New"); got != 1 {
		t.Fatalf("news on the receiving board = %d, want 1 (news: %v)", got, receiver.NewsToday)
	}
	// The next day's export still carries FormerName, and must not re-announce.
	receiver.ApplyPacket(export())
	if got := renameNewsCount(receiver, "Old", "New"); got != 1 {
		t.Errorf("news after the following export = %d, want 1", got)
	}
	// And the snapshot itself moved onto the new name, with no ghost row.
	var names []string
	for _, b := range receiver.RemoteBoards {
		if b.BoardID == "Far BBS" {
			for _, s := range b.Scores {
				names = append(names, s.Empire)
			}
		}
	}
	if len(names) != 1 || names[0] != "New" {
		t.Errorf("Far BBS snapshot = %v, want [New]", names)
	}
}
