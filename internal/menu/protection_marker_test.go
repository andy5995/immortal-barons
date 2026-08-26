package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// protectionWorld seeds a board with two computer rivals so a target list has
// both a shielded realm and an open one on it — a list with nothing attackable
// left never reaches the prompt at all.
func protectionWorld(t *testing.T) (*ctx, *game.Empire, *game.Empire) {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.AICount = 2
	cfg.Lottery = false
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("tester", "Testland")
	c := &ctx{World: w, handle: "tester", Term: Term{UTF8: true}}
	c.Today = "2026-07-03"

	p := c.Player()
	p.Protection = 0
	var rivals []*game.Empire
	for _, e := range c.Empires {
		if e != p && e.Alive {
			rivals = append(rivals, e)
		}
	}
	if len(rivals) < 2 {
		t.Fatalf("the seeded board holds %d rivals, want 2", len(rivals))
	}
	rivals[0].Protection = 5
	rivals[1].Protection = 0
	return c, rivals[0], rivals[1]
}

// A realm under New Realm Protection is listed as a target WITH its own slot
// letter and with the (P) flag after its name (#214). It carried no letter at
// all until 2026-08-26, which said the row was different without saying how.
func TestProtectedRealmIsListedWithItsLetterAndFlag(t *testing.T) {
	w, shielded, open := protectionWorld(t)
	// RETURN aborts at the target prompt, so the list is drawn and nothing else.
	f := &fakeSession{keys: []rune("\r")}
	regularAttack(f, w)
	plain := stripANSI(f.out.String())

	if !strings.Contains(plain, "Empire Name") || !strings.Contains(plain, "Attack which realm?") {
		t.Fatalf("the script never reached the target list:\n%s", plain)
	}
	row := findLine(plainLines(plain), shielded.Name)
	if row == "" {
		t.Fatalf("the shielded realm %q is missing from the list:\n%s", shielded.Name, plain)
	}
	if !strings.Contains(row, "["+shielded.Letter()+"]") {
		t.Errorf("the shielded realm lost its slot letter %q: %q", shielded.Letter(), row)
	}
	if !strings.Contains(row, "(P)") {
		t.Errorf("the shielded realm carries no protection flag: %q", row)
	}
	// An unshielded realm must not pick the flag up as a side effect.
	if other := findLine(plainLines(plain), open.Name); strings.Contains(other, "(P)") {
		t.Errorf("an unshielded realm was flagged: %q", other)
	}
}

// The letter is now pressable, so the ENFORCEMENT has to answer it: pressing a
// shielded realm's letter is refused by name and launches nothing.
func TestAttackingAProtectedRealmIsStillRefused(t *testing.T) {
	w, shielded, _ := protectionWorld(t)
	land := shielded.Land

	f := &fakeSession{keys: []rune(shielded.Letter() + "\r")}
	regularAttack(f, w)
	plain := stripANSI(f.out.String())

	if !strings.Contains(plain, "Empire Name") {
		t.Fatalf("the script never reached the target list:\n%s", plain)
	}
	if !strings.Contains(plain, "New Realm Protection") || !strings.Contains(plain, shielded.Name) {
		t.Errorf("pressing a shielded realm's letter was not refused by name:\n%s", plain)
	}
	if strings.Contains(plain, "Send how many Troopers?") {
		t.Errorf("the force prompts were reached for a shielded target:\n%s", plain)
	}
	if w.Player().AttacksToday != 0 {
		t.Errorf("AttacksToday = %d, want 0 — a refused attack must not be counted", w.Player().AttacksToday)
	}
	if shielded.Land != land {
		t.Errorf("the shielded realm lost land (%d -> %d)", land, shielded.Land)
	}
}

// A baron on another planet whose last scores packet had them under protection
// is LISTED with the (P) flag, and the strike is refused when that row is
// picked. They were hidden from the list until 2026-08-26, under a count that
// named nobody (#214).
func TestProtectedRemoteBaronIsListedAndRefused(t *testing.T) {
	w := ipWorld()
	w.RemoteBoards = []game.RemoteBoard{
		{BoardID: "The Eclipse", Scores: []game.RemoteScore{
			{Empire: "Iron Dominion", Land: 900},
			{Empire: "Fresh Meat", Land: 20, Protected: true},
		}},
	}
	p := w.Player()
	p.Agents, p.Protection = 50, 0

	// "?4\r" names The Eclipse by its roster number, then "2\r" picks the
	// protected baron off the baron list.
	f := &fakeSession{keys: []rune("?4\r2\r")}
	doTerrorOp(f, w, game.TerrorOpSpy)
	plain := stripANSI(f.out.String())

	if !strings.Contains(plain, "Terrorize which baron?") {
		t.Fatalf("the script never reached the baron list:\n%s", plain)
	}
	if !strings.Contains(plain, "Fresh Meat (P)") {
		t.Errorf("the protected baron is not listed with its flag:\n%s", plain)
	}
	if !strings.Contains(plain, "Iron Dominion") {
		t.Errorf("the open baron went missing from the list:\n%s", plain)
	}
	if !strings.Contains(plain, "New Realm Protection") {
		t.Errorf("picking the protected baron was not refused:\n%s", plain)
	}
	if len(w.Outbox) != 0 {
		t.Errorf("a strike was queued against a protected baron: %+v", w.Outbox)
	}
}

// Protection bars SPYING as surely as it bars a strike — Andy's call, and the
// behaviour every path already had. docs/mechanics-reference.md claimed the
// opposite until 2026-08-26 ("a protected realm is a legal SPY target, so a spy
// list carries no flag at all"), and a hostile/observing constant pair encoded
// that claim in the interplanetary picker with only the hostile half ever
// passed. The doc was wrong about the code; this is what the code does.
func TestSpyingIsRefusedOnAProtectedRealm(t *testing.T) {
	w, shielded, _ := protectionWorld(t)
	before := shielded.Agents

	// Pick the shielded realm by its own slot letter at the covert target list.
	f := &fakeSession{keys: []rune(shielded.Letter() + "\r")}
	covertAction(game.OpSendSpy)(f, w)
	out := stripANSI(f.out.String())

	// Reached the list, and reached it with that realm on it — not an empty
	// screen that would pass this test while proving nothing.
	if !strings.Contains(out, shielded.Name) {
		t.Fatalf("never reached a target list holding %s:\n%s", shielded.Name, out)
	}
	if !strings.Contains(out, "New Realm Protection") {
		t.Errorf("spying on a protected realm was not refused:\n%s", out)
	}
	// And no report came back, which is what a spy that ran would produce.
	if shielded.Agents != before {
		t.Errorf("the target's agents moved (%d -> %d); an op ran", before, shielded.Agents)
	}
	if p := w.Player(); p.TurnProgress.CovertOpsUsed[game.OpSendSpy] {
		t.Error("a refused spy still consumed the once-per-turn slot")
	}
}

// The interplanetary spy is the same rule across the league: the scores packet's
// flag is drawn on the baron list and the pick is refused.
func TestInterplanetarySpyingIsRefusedOnAProtectedBaron(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		w.Config.BoardID = "Home BBS"
		w.Player().Protection = 0
		w.Player().Agents = 50
		w.ImportBoard(game.RemoteBoard{BoardID: "Far BBS", Scores: []game.RemoteScore{
			{Empire: "Fresh Realm", Land: 20, NetWorth: 500, Score: 100, Protected: true},
			{Empire: "Open Realm", Land: 100, NetWorth: 5000, Score: 900},
		}})
	})
	f := &fakeSession{keys: []rune("1\r" + "1\r" + " ")}
	doTerrorOp(f, w, game.TerrorOpSpy)
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "Fresh Realm") {
		t.Fatalf("never reached a baron list holding the protected realm:\n%s", out)
	}
	if !strings.Contains(out, "New Realm Protection") {
		t.Errorf("spying on a protected baron was not refused:\n%s", out)
	}
	var queued int
	w.Read(func() {
		for _, p := range w.Outbox {
			queued += len(p.Terrors)
		}
	})
	if queued != 0 {
		t.Errorf("a refused interplanetary spy was queued anyway (%d)", queued)
	}
}
