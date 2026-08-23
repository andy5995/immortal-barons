package game

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/bulletin"
)

func bulletinWorld(t *testing.T) *World {
	t.Helper()
	cfg := DefaultConfig()
	cfg.BoardID = "Nova Hub"
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-23"
	return w
}

func file(name, title, body string) BulletinFile {
	return BulletinFile{Name: name, Title: title, Data: []byte(body)}
}

func TestBulletinNewsNamesTheBulletinThatChanged(t *testing.T) {
	w := bulletinWorld(t)
	if !w.RecordBulletins(bulletin.Local, []BulletinFile{file("news.txt", "Board news", "Board news\nv1\n")}) {
		t.Fatal("the first recording reported no change")
	}
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Board news") {
		t.Fatalf("the first bulletin filed %v", w.NewsToday)
	}
	// An added bulletin is news, and it says which one.
	w.NewsToday = nil
	w.RecordBulletins(bulletin.Local, []BulletinFile{
		file("news.txt", "Board news", "Board news\nv1\n"),
		file("party.txt", "Anniversary party", "Anniversary party\ncome along\n"),
	})
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Anniversary party") {
		t.Fatalf("adding a bulletin filed %v", w.NewsToday)
	}
	// An edit to a file already listed is news too, and the unedited one is not.
	w.NewsToday = nil
	w.RecordBulletins(bulletin.Local, []BulletinFile{
		file("news.txt", "Board news", "Board news\nv2\n"),
		file("party.txt", "Anniversary party", "Anniversary party\ncome along\n"),
	})
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Board news") {
		t.Fatalf("editing a bulletin filed %v", w.NewsToday)
	}
	// Recording the same set again says nothing at all.
	w.NewsToday = nil
	if w.RecordBulletins(bulletin.Local, []BulletinFile{
		file("news.txt", "Board news", "Board news\nv2\n"),
		file("party.txt", "Anniversary party", "Anniversary party\ncome along\n"),
	}) {
		t.Error("an unchanged set reported a change")
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("an unchanged set filed news: %v", w.NewsToday)
	}
	// A withdrawn bulletin is a change, but not a news item.
	if !w.RecordBulletins(bulletin.Local, []BulletinFile{file("news.txt", "Board news", "Board news\nv2\n")}) {
		t.Error("a withdrawn bulletin reported no change")
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("a withdrawn bulletin filed news: %v", w.NewsToday)
	}
	if _, still := w.BulletinDigest[bulletinKey(bulletin.Local, "party.txt")]; still {
		t.Error("a withdrawn bulletin is still fingerprinted")
	}
}

// TestAnUpgradedBoardTakesItsExistingBulletinsAsTheBaseline: a world saved
// before bulletins existed knows neither scope, and the files already sitting
// in bull/ are not news — a sysop who has had them there for months would
// otherwise come back to a news page full of them.
func TestAnUpgradedBoardTakesItsExistingBulletinsAsTheBaseline(t *testing.T) {
	w := bulletinWorld(t)
	w.BulletinsKnown = nil // as an older save loads
	w.RecordBulletins(bulletin.Local, []BulletinFile{
		file("news.txt", "Board news", "Board news\n"),
		file("party.txt", "Anniversary party", "come along\n"),
	})
	if len(w.NewsToday) != 0 {
		t.Fatalf("the baseline recording filed news: %v", w.NewsToday)
	}
	w.RecordBulletins(bulletin.Local, []BulletinFile{
		file("news.txt", "Board news", "Board news\n"),
		file("party.txt", "Anniversary party", "come along\n"),
		file("new.txt", "Fresh off the press", "Fresh off the press\n"),
	})
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "Fresh off the press") {
		t.Errorf("after the baseline, news is %v", w.NewsToday)
	}
}

func TestGalacticBulletinNewsSaysGalactic(t *testing.T) {
	w := bulletinWorld(t)
	w.RecordBulletins(bulletin.League, []BulletinFile{file("rules.txt", "League rules", "League rules\n")})
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0], "galactic bulletin") {
		t.Fatalf("league news is %v", w.NewsToday)
	}
	// The two scopes are counted apart: a local file of the same name is its own
	// bulletin, not an edit of the league's.
	w.NewsToday = nil
	w.RecordBulletins(bulletin.Local, []BulletinFile{file("rules.txt", "House rules", "House rules\n")})
	if len(w.NewsToday) != 1 || strings.Contains(w.NewsToday[0], "galactic") {
		t.Fatalf("local news is %v", w.NewsToday)
	}
}

func TestApplyBulletinsRefusesAnUnsafeName(t *testing.T) {
	w := bulletinWorld(t)
	w.applyBulletins(BulletinSet{Files: []BulletinFile{
		file("../../world.json", "Trouble", "Trouble\n"),
		file("big.txt", "Too big", strings.Repeat("x", bulletin.MaxSize+1)),
		file("rules.txt", "League rules", "League rules\n"),
	}})
	if w.PendingBulletins == nil {
		t.Fatal("nothing was handed on to be written")
	}
	if len(w.PendingBulletins.Files) != 1 || w.PendingBulletins.Files[0].Name != "rules.txt" {
		t.Fatalf("pending set is %+v", w.PendingBulletins.Files)
	}
}

func TestOnlyTheCoordinatorBroadcastsBulletins(t *testing.T) {
	w := bulletinWorld(t)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "The Eclipse"}, {Number: 2, Name: "Nova Hub"}}
	w.ExportBulletins([]BulletinFile{file("rules.txt", "League rules", "League rules\n")})
	if len(w.Outbox) != 0 {
		t.Fatalf("a member board queued a bulletin broadcast: %+v", w.Outbox)
	}
	w.LeagueNodes[0].Name, w.LeagueNodes[1].Name = "Nova Hub", "The Eclipse"
	w.ExportBulletins([]BulletinFile{file("rules.txt", "League rules", "League rules\n")})
	if len(w.Outbox) != 1 || w.Outbox[0].Bulletins == nil {
		t.Fatalf("the Coordinator queued %+v", w.Outbox)
	}
	if !carriesCoordinatorOrders(w.Outbox[0]) {
		t.Error("a bulletin broadcast is not treated as Coordinator orders, so it needs no signature")
	}
}
