package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/bulletin"
	"github.com/andy5995/immortal-barons/internal/game"
)

// bulletinBoard is a world whose data directory is a temp dir, so the bulletin
// directories under it are the test's own.
func bulletinBoard(t *testing.T, name string, roster []game.LeagueNode) *game.World {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.BoardID = name
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-23"
	w.LeagueNodes = roster
	return w
}

func putBulletin(t *testing.T, w *game.World, scope bulletin.Scope, name, body string) {
	t.Helper()
	if err := bulletin.Write(w.Config.DataDir, scope, name, []byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestSyncReadsTheBoardsOwnBulletinsAndFilesTheNews(t *testing.T) {
	w := bulletinBoard(t, "Nova Hub", nil)
	putBulletin(t, w, bulletin.Local, "news.txt", "Board news\nthe usual\n")
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Board news") {
		t.Fatalf("the sysop's first bulletin filed %v", w.NewsToday)
	}
	w.NewsToday = nil
	putBulletin(t, w, bulletin.Local, "party.txt", "Anniversary party\ncome along\n")
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Anniversary party") {
		t.Fatalf("news is %v", w.NewsToday)
	}
	// A board in no league has no Coordinator to fill bull/league, so what its
	// sysop puts there is recorded like the rest — but it is still not offered
	// for broadcast.
	w.NewsToday = nil
	putBulletin(t, w, bulletin.League, "rules.txt", "House league rules\nplay fair\n")
	league, err := SyncBulletins(w)
	if err != nil || league != nil {
		t.Errorf("a board that is not the Coordinator offered %v to broadcast (%v)", league, err)
	}
	if !newsMentions(w, "House league rules") {
		t.Errorf("a stand-alone board's galactic bulletin filed no news: %v", w.NewsToday)
	}
}

// TestAMemberBoardDoesNotAuthorItsOwnLeagueBulletins: bull/league belongs to
// the Coordinator there, so a file a member's sysop puts in it is neither news
// nor kept — the next broadcast prunes it.
func TestAMemberBoardDoesNotAuthorItsOwnLeagueBulletins(t *testing.T) {
	w := bulletinBoard(t, "Nova Hub", []game.LeagueNode{
		{Number: 1, Name: "The Eclipse"}, {Number: 2, Name: "Nova Hub"},
	})
	putBulletin(t, w, bulletin.League, "mine.txt", "My own rules\n")
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("a member board filed news for its own league bulletin: %v", w.NewsToday)
	}
}

func TestSyncWritesADeliveredLeagueSetAndPrunesWhatIsGone(t *testing.T) {
	w := bulletinBoard(t, "Nova Hub", []game.LeagueNode{
		{Number: 1, Name: "The Eclipse"}, {Number: 2, Name: "Nova Hub"},
	})
	putBulletin(t, w, bulletin.League, "stale.txt", "Withdrawn\nold\n")
	w.PendingBulletins = &game.BulletinSet{Files: []game.BulletinFile{
		{Name: "rules.txt", Title: "League rules", Data: []byte("League rules\nbe nice\n")},
	}}
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if w.PendingBulletins != nil {
		t.Error("the delivered set was not drained")
	}
	got, err := bulletin.List(w.Config.DataDir, bulletin.League)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "rules.txt" || got[0].Title != "League rules" {
		t.Fatalf("bull/league holds %+v", got)
	}
	// A file deleted by hand comes back: the Coordinator's set is authoritative.
	if err := os.Remove(filepath.Join(bulletin.Dir(w.Config.DataDir, bulletin.League), "rules.txt")); err != nil {
		t.Fatal(err)
	}
	w.PendingBulletins = &game.BulletinSet{Files: []game.BulletinFile{
		{Name: "rules.txt", Title: "League rules", Data: []byte("League rules\nbe nice\n")},
	}}
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if got, _ := bulletin.List(w.Config.DataDir, bulletin.League); len(got) != 1 {
		t.Errorf("a hand-deleted league bulletin was not restored: %+v", got)
	}
}

// TestCoordinatorDistributesBulletinsToTheLeague is the whole path over real
// directories: the Coordinator's sysop copies a file into bull/league, and it
// turns up in the member board's own bull/league with a news item naming it.
func TestCoordinatorDistributesBulletinsToTheLeague(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub", City: "Brisbane"},
		{Number: 2, Name: "The Eclipse", City: "Sydney"},
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	lc := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	member := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	lc.w.Config.DataDir, member.w.Config.DataDir = filepath.Join(dir, "a"), filepath.Join(dir, "b")
	lc.w.CoordKey, lc.w.CoordPub = sec, pub
	member.w.CoordPub = pub

	putBulletin(t, lc.w, bulletin.League, "rules.txt", "League rules\nNo bullying.\n")
	lc.run(t)
	if n := deliver(t, lc, member); n == 0 {
		t.Fatal("the Coordinator wrote no packets")
	}
	member.run(t)

	got, err := bulletin.List(member.w.Config.DataDir, bulletin.League)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "League rules" {
		t.Fatalf("the member board holds %+v", got)
	}
	data, err := os.ReadFile(got[0].Path)
	if err != nil || string(data) != "League rules\nNo bullying.\n" {
		t.Fatalf("the distributed file reads %q (%v)", data, err)
	}
	if !newsMentions(member.w, "League rules") {
		t.Errorf("the member board's news does not name the new bulletin: %v", member.w.NewsToday)
	}
	// An edit on the Coordinator's board reaches the member as an update.
	member.w.NewsToday = nil
	putBulletin(t, lc.w, bulletin.League, "rules.txt", "League rules\nNo bullying. No spying.\n")
	lc.run(t)
	deliver(t, lc, member)
	member.run(t)
	data, _ = os.ReadFile(got[0].Path)
	if !strings.Contains(string(data), "No spying") {
		t.Errorf("the edited bulletin did not arrive: %q", data)
	}
	if !newsMentions(member.w, "updated") {
		t.Errorf("the edit filed no news: %v", member.w.NewsToday)
	}
	// A rebroadcast of the same set says nothing further.
	member.w.NewsToday = nil
	lc.run(t)
	deliver(t, lc, member)
	member.run(t)
	if len(member.w.NewsToday) != 0 {
		t.Errorf("an unchanged rebroadcast filed news: %v", member.w.NewsToday)
	}
}

// TestAMemberBoardCannotDictateBulletins: the set is Coordinator-authored, so a
// packet from anyone else is refused like a forged ruleset.
func TestAMemberBoardCannotDictateBulletins(t *testing.T) {
	w := bulletinBoard(t, "Nova Hub", []game.LeagueNode{
		{Number: 1, Name: "The Eclipse"}, {Number: 2, Name: "Nova Hub"},
	})
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w.CoordPub = pub
	w.ApplyPacket(game.Packet{
		FromBoard: "Impostor",
		Date:      "2026-08-23",
		Bulletins: &game.BulletinSet{Files: []game.BulletinFile{
			{Name: "rules.txt", Title: "Surrender now", Data: []byte("Surrender now\n")},
		}},
	})
	if w.PendingBulletins != nil {
		t.Fatalf("an unsigned bulletin set was accepted: %+v", w.PendingBulletins)
	}
	if _, err := SyncBulletins(w); err != nil {
		t.Fatal(err)
	}
	if got, _ := bulletin.List(w.Config.DataDir, bulletin.League); len(got) != 0 {
		t.Errorf("bull/league holds %+v", got)
	}
}

func newsMentions(w *game.World, want string) bool {
	for _, line := range w.NewsToday {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
