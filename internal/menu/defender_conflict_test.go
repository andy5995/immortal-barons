package menu

import (
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// These tests cover the VICTIM's side of a cross-node conflict: the other
// conflict tests prove an acting node's writes are safe, these prove a node
// that is attacked (or bled as an ally) while its player sits at a prompt
// carries on consistently — the battle losses persist, and the victim's next
// action applies on top of the post-battle state instead of resurrecting the
// pre-battle numbers it had on screen.

// TestAttackedWhileOnlineDefenderBuysConsistently: node B's player is at the
// buy-Troopers prompt when another node's attack lands (units and regions
// lost, committed to the file). B then completes the purchase. The purchase
// must apply to the post-attack empire — committed troopers are the reduced
// count plus the buy, and the captured regions stay captured.
func TestAttackedWhileOnlineDefenderBuysConsistently(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 10000
		p.Gold = game.PriceTrooper * 1000
	})
	addMallory(t, cfg, 1_000_000)
	startLand := committedEmpire(t, cfg, "bob").Land

	var postAttackTroopers, postAttackLand int
	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("10\r")},
		marker:      "How many?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) {
				a, d := w.FindByOwner("mallory"), w.FindByOwner("bob")
				w.Attack(a, d, game.FullForce(a), true)
			})
			e := committedEmpire(t, cfg, "bob")
			postAttackTroopers, postAttackLand = e.Troopers, e.Land
		},
	}
	buyTroopers := buyUnit("Troopers", false, func(w *ctx) int { return w.TrooperPrice(w.Player()) }, (*game.World).Recruit)
	buyTroopers(fb, b)

	if postAttackTroopers >= 10000 || postAttackLand >= startLand {
		t.Fatalf("the mid-prompt attack cost nothing (troopers %d, land %d/%d) — test setup is wrong", postAttackTroopers, postAttackLand, startLand)
	}
	e := committedEmpire(t, cfg, "bob")
	if e.Troopers != postAttackTroopers+10 {
		t.Fatalf("committed troopers = %d, want post-attack %d + 10 bought — the defender's session resurrected pre-attack forces", e.Troopers, postAttackTroopers)
	}
	if e.Land != postAttackLand {
		t.Fatalf("committed land = %d, want the post-attack %d — the defender's session resurrected captured regions", e.Land, postAttackLand)
	}
}

// TestAllyBledWhileOnlineBuysConsistently: node B's player is allied to a realm
// that gets attacked while B sits at a prompt. bleedAllies deducts B's committed
// 30% share on the attacking node; B's purchase must land on the bled count.
func TestAllyBledWhileOnlineBuysConsistently(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "carol", "Carolia", nil, func(p *game.Empire) {
		p.Troopers = 10000
		p.Gold = game.PriceTrooper * 1000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("bob", "Defendia")
		d.Protection = 0
		w.Treaties = append(w.Treaties, game.Treaty{Type: "Full Defense Alliance", A: "Carolia", B: "Defendia"})
	})
	addMallory(t, cfg, 1_000_000)

	var postBleedTroopers int
	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("10\r")},
		marker:      "How many?",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) {
				a, d := w.FindByOwner("mallory"), w.FindByOwner("bob")
				w.Attack(a, d, game.FullForce(a), true)
			})
			postBleedTroopers = committedEmpire(t, cfg, "carol").Troopers
		},
	}
	buyTroopers := buyUnit("Troopers", false, func(w *ctx) int { return w.TrooperPrice(w.Player()) }, (*game.World).Recruit)
	buyTroopers(fb, b)

	if postBleedTroopers >= 10000 {
		t.Fatalf("the ally was never bled (troopers %d) — test setup is wrong", postBleedTroopers)
	}
	if e := committedEmpire(t, cfg, "carol"); e.Troopers != postBleedTroopers+10 {
		t.Fatalf("committed troopers = %d, want post-bleed %d + 10 bought — the ally's session resurrected bled forces", e.Troopers, postBleedTroopers)
	}
}

// TestCrushedWhileOnlineEndsSessionFileBacked: the file-backed twin of
// TestMidSessionDeathEndsSession. Node B's player is browsing a menu while
// another node grinds the realm to total conquest on disk. B's next menu
// iteration must reload, see the empire dead, print the collapse notice, and
// end the session rather than let the player keep driving a corpse.
func TestCrushedWhileOnlineEndsSessionFileBacked(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "bob", "Defendia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 0
	})
	addMallory(t, cfg, 1_000_000)

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("x")},
		marker:      "Doomed Menu",
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
	m := &Menu{Title: "Doomed Menu", Items: []Item{
		{Key: 'x', Label: "Wait", Do: func(s session.Session, g *ctx) Result { return Stay }},
	}}
	var err error
	func() {
		defer session.GuardEnd(&err)
		err = Run(fb, b, m)
	}()
	if err != io.EOF {
		t.Fatalf("crushed player should end the session via session.End (io.EOF), got %v", err)
	}
	if !strings.Contains(fb.out.String(), "collapsed") {
		t.Errorf("expected the collapse notice, got %q", fb.out.String())
	}
}
