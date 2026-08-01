package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Tests for the mid-session news notice: a player whose empire is hit by
// another node while they sit at a menu or prompt is told at their next
// action, exactly once, without stealing events from the turn-start recap.

// commitAttackOn commits one full-force attack by mallory against the named
// defender, as another BBS node would.
func commitAttackOn(t *testing.T, cfg game.Config, handle string) {
	t.Helper()
	commitOnFile(t, cfg, func(w *game.World) {
		a, d := w.FindByOwner("mallory"), w.FindByOwner(handle)
		w.Attack(a, d, game.FullForce(a), true)
	})
}

// addMallory commits the attacking node's realm with the given army. 40k wins
// against a 10k defender but wounds rather than crushes; 1M crushes a
// starting-size realm outright (the capture floor alone can take all its land).
func addMallory(t *testing.T, cfg game.Config, troopers int) {
	t.Helper()
	commitOnFile(t, cfg, func(w *game.World) {
		m := w.AddHuman("mallory", "Malloria")
		m.Protection = 0
		m.Troopers = troopers
	})
}

// TestMidSessionAttackNoticeShownOnce: node B's player idles at a menu, another
// node attacks. The next action's post-action check must print the notice with
// the attack event, exactly once — and consume the event so the next session's
// "since your last play" recap doesn't repeat it.
func TestMidSessionAttackNoticeShownOnce(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 10000
	})
	// Enough territory that losing a battle wounds instead of crushes — the
	// capture floor alone can wipe out a starting-size realm.
	commitOnFile(t, cfg, func(w *game.World) {
		e := w.FindByOwner("bob")
		w.GrantRegions(e, &e.Regions.Coastal, 300)
	})
	addMallory(t, cfg, 40_000)

	ticks := 0
	m := &Menu{Title: "Idle Test", Items: []Item{
		{Key: 'x', Label: "Tick", Do: func(s session.Session, g *ctx) Result {
			ticks++
			return Stay
		}},
	}}
	fb := &hookSession{
		// Three ticks: baseline is set at Run entry, the hook fires on the first
		// ReadKey (attack lands while the player "idles"), tick 1's check shows
		// the notice, ticks 2-3 prove it doesn't repeat.
		fakeSession: fakeSession{keys: []rune("xxx")},
		marker:      "Idle Test",
		hook:        func() { commitAttackOn(t, cfg, "bob") },
	}
	var err error
	func() {
		defer session.GuardEnd(&err)
		Run(fb, b, m)
	}()

	out := fb.out.String()
	if ticks != 3 {
		t.Fatalf("expected 3 ticks, got %d (session err %v) — did the attack crush the player?", ticks, err)
	}
	if !strings.Contains(out, "While you were at the menus") || !strings.Contains(out, "attacked you") {
		t.Fatalf("expected the mid-session attack notice, got: %q", out)
	}
	if strings.Count(out, "While you were at the menus") != 1 {
		t.Fatalf("the notice repeated: %q", out)
	}
	if evs := committedEmpire(t, cfg, "bob").Events; len(evs) != 0 {
		t.Fatalf("shown events must be consumed so the recap won't repeat them; still committed: %v", evs)
	}
}

// TestSessionBacklogNotShownAsNews: events that already existed when the
// session began belong to the turn-start recap — the mid-session notice must
// not print them, and must leave them committed for that recap.
func TestSessionBacklogNotShownAsNews(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, nil)
	commitOnFile(t, cfg, func(w *game.World) {
		e := w.FindByOwner("bob")
		e.Events = append(e.Events, game.Event{Text: "Ancient backlog item."})
	})

	m := &Menu{Title: "Idle Test", Items: []Item{
		{Key: 'x', Label: "Tick", Do: func(s session.Session, g *ctx) Result { return Stay }},
	}}
	fb := &fakeSession{keys: []rune("xx")}
	Run(fb, b, m)

	if strings.Contains(fb.out.String(), "Ancient backlog item") {
		t.Fatalf("pre-session backlog leaked into the mid-session notice: %q", fb.out.String())
	}
	if evs := committedEmpire(t, cfg, "bob").Events; len(evs) != 1 {
		t.Fatalf("backlog must stay committed for the turn-start recap, got %v", evs)
	}
}

// TestBuyLandFlushesNewsMidLoop: Buy Regions is a loop the player can stay
// inside for many purchases, so the notice must surface between prompts, not
// only at the next menu redraw.
func TestBuyLandFlushesNewsMidLoop(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Gold = 10_000_000
		p.LandAvailable = 100
	})
	// Enough territory to be wounded, not crushed (see the notice test above).
	commitOnFile(t, cfg, func(w *game.World) {
		e := w.FindByOwner("bob")
		w.GrantRegions(e, &e.Regions.Coastal, 300)
	})
	addMallory(t, cfg, 40_000)
	postActionCheck(b) // stand in for the menu navigation that precedes buyLand

	fb := &hookSession{
		// Coastal, quantity 5, then quit the loop.
		fakeSession: fakeSession{keys: []rune("C5\r0")},
		marker:      "Buy how many Coastal regions?",
		hook:        func() { commitAttackOn(t, cfg, "bob") },
	}
	buyLand(fb, b)

	out := fb.out.String()
	if !strings.Contains(out, "While you were at the menus") || !strings.Contains(out, "attacked you") {
		t.Fatalf("expected the notice between Buy Regions prompts, got: %q", out)
	}
}

// TestRegularAttackReportsTrimmedForce: the player commits a force typed
// against pre-prompt holdings; a concurrent strike thins them before the
// launch. clampTo already sends only what remains — the player must be told.
func TestRegularAttackReportsTrimmedForce(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 10000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		v := w.AddHuman("victim", "Victimville")
		v.Protection = 0
	})

	fb := &hookSession{
		// Target (A), send 10000 troopers, defaults for the rest.
		fakeSession: fakeSession{keys: []rune("A10000\r\r\r\r")},
		marker:      "Send how many Troopers?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.FindByOwner("bob").Troopers = 100 })
		},
	}
	regularAttack(fb, b)

	if out := fb.out.String(); !strings.Contains(out, "only units still under your command were sent") {
		t.Fatalf("expected the trimmed-force notice, got: %q", out)
	}
}

// TestPirateRaidReportsTrimmedForce: RaidFaction clamps the committed force to
// fresh stock just like a Regular Attack does — the raider must be told too.
func TestPirateRaidReportsTrimmedForce(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 10000
	})

	fb := &hookSession{
		// Faction 1, commit 10000 troopers, defaults for jets/tanks.
		fakeSession: fakeSession{keys: []rune("110000\r\r\r")},
		marker:      "Commit how many Troopers?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.FindByOwner("bob").Troopers = 100 })
		},
	}
	attackPirates(fb, b)

	if out := fb.out.String(); !strings.Contains(out, "only units still under your command were sent") {
		t.Fatalf("expected the trimmed-force notice, got: %q", out)
	}
}

// TestBuyLandEndsCrushedSession: a player crushed to total conquest while
// inside the Buy Regions loop must be stopped at the next flush, not left
// shopping with the corpse's gold until they choose to leave.
func TestBuyLandEndsCrushedSession(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Gold = 10_000_000
		p.LandAvailable = 100
	})
	addMallory(t, cfg, 1_000_000)
	postActionCheck(b) // stand in for the menu navigation that precedes buyLand

	fb := &hookSession{
		// Coastal, quantity 5 — the crush lands mid-prompt; the post-purchase
		// flush must end the session before another prompt is offered.
		fakeSession: fakeSession{keys: []rune("C5\r0")},
		marker:      "Buy how many Coastal regions?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) {
				a, d := w.FindByOwner("mallory"), w.FindByOwner("bob")
				for i := 0; d.Alive && i < 200; i++ {
					w.Attack(a, d, game.FullForce(a), true)
				}
				if d.Alive {
					t.Fatal("could not crush the defender in 200 attacks — test setup is wrong")
				}
			})
		},
	}
	var err error
	func() {
		defer session.GuardEnd(&err)
		buyLand(fb, b)
	}()

	if err == nil {
		t.Fatalf("a crushed player must not keep buying; buyLand returned normally: %q", fb.out.String())
	}
	if !strings.Contains(fb.out.String(), "collapsed") {
		t.Errorf("expected the collapse notice, got %q", fb.out.String())
	}
}

// TestSellReportsActualSold: the game-level sell clamps to current stock; the
// confirmation must report what was really sold, and say why it differs.
func TestSellReportsActualSold(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Troopers = 10000
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("5000\r")},
		marker:      "Sell Troopers (half price)?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.FindByOwner("bob").Troopers = 100 })
		},
	}
	sell := sellUnit("Sell Troopers", func(e *game.Empire) int { return e.Troopers }, (*game.World).SellTroopers)
	sell(fb, b)

	out := fb.out.String()
	if !strings.Contains(out, "Sold 100.") {
		t.Fatalf("confirmation must report the ACTUAL count sold, got: %q", out)
	}
	if !strings.Contains(out, "only what remained was sold") {
		t.Fatalf("expected the holdings-changed note, got: %q", out)
	}
}
