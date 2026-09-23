package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The original scopes its New Realm Protection refusals, one per routine
// (docs/dev/bre-screens.md), and IB words each as that routine does. Each test
// asserts its own refusal and that none of the others leaked onto its screen.
var protectionRefusals = []string{
	"That empire is in protection.",
	"You are in protection.",
	"Our empire is in protection, my lord.",
	"That realm is still in protection.",
	"Your dominion is still under protection.",
	"Sorry....You are under New Realm Protection!",
	"Sorry... You're under new realm protection.",
}

// assertOnlyRefusal fails unless out carries want and no other scoped refusal.
func assertOnlyRefusal(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("want the refusal %q, got:\n%s", want, out)
	}
	for _, other := range protectionRefusals {
		if other != want && strings.Contains(out, other) {
			t.Errorf("printed %q where only %q belongs:\n%s", other, want, out)
		}
	}
}

// A trade offer to a shielded realm is refused as soon as the realm is picked,
// before either basket is built — create_trade_offer's order.
func TestTradeOfferToShieldedRealmRefused(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Protection = 0
	target := recipients(w)[0]
	if target.Protection <= 0 {
		t.Fatal("the rival should start under New Realm Protection")
	}
	f := &fakeSession{keys: []rune(target.Letter() + " ")}
	sendTradeDeal(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Trade with:") || !strings.Contains(out, target.Name) {
		t.Fatalf("never reached the recipient picker and chose %s:\n%s", target.Name, out)
	}
	assertOnlyRefusal(t, out, "That realm is still in protection.")
	if strings.Contains(out, "Offer") {
		t.Errorf("went on to the baskets after refusing:\n%s", out)
	}
	if len(target.TradeDeals) != 0 {
		t.Errorf("a refused offer reached the target: %v", target.TradeDeals)
	}
}

// A shielded sender is refused before the picker is even drawn.
func TestTradeOfferFromShieldedRealmRefused(t *testing.T) {
	w := newWorld()
	if w.Player().Protection <= 0 {
		t.Fatal("the player should start under New Realm Protection")
	}
	f := &fakeSession{keys: []rune(" ")}
	sendTradeDeal(f, w)
	out := stripANSI(f.out.String())
	assertOnlyRefusal(t, out, "Your dominion is still under protection.")
	if strings.Contains(out, "Trade with:") {
		t.Errorf("drew the recipient picker for a shielded sender:\n%s", out)
	}
}

// Every InterPlanetary item that acts on another planet refuses a shielded
// caller with the menu's own words, run_interbbs_menu's, before its first
// prompt. Join Group Attack is the exception the menu gate skips; it refuses in
// its own words, after drawing what is forming.
func TestInterPlanetaryItemsRefuseAShieldedCaller(t *testing.T) {
	const ipGate = "Sorry....You are under New Realm Protection!"
	for _, tc := range []struct {
		name string
		do   func(session.Session, *ctx) Result
		// notReached is text the item prints only once past the gate.
		notReached string
	}{
		{"Gooie Kablooie Ops", gooieKablooie, "Gooie Kablooie"},
		{"Send Trade Deal", sendIPTradeDeal, "Ship to which planet?"},
		{"Create Group Attack", createGroupAttack, "planet"},
		{"Indiv. Attack Force", indivAttackForce, "planet"},
		{"Send SpyGuy", sendSpyGuy, "gold per day"},
		{"Terrorist Ops", terrorOp(game.TerrorOpSpy), "agents"},
		{"Special Operations", ipSpecialOp(game.OpBombFood), "Bombers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld()
			w.Config.IBBS = true
			w.Config.GooieKablooie = true
			w.turnPlayed = true
			if w.Player().Protection <= 0 {
				t.Fatal("the player should start under New Realm Protection")
			}
			f := &fakeSession{keys: []rune(" ")}
			if got := tc.do(f, w); got != Stay {
				t.Errorf("returned %v, want Stay", got)
			}
			out := stripANSI(f.out.String())
			assertOnlyRefusal(t, out, ipGate)
			if strings.Contains(out, tc.notReached) {
				t.Errorf("went past the gate:\n%s", out)
			}
		})
	}
}

func TestJoinGroupAttackRefusesAShieldedCaller(t *testing.T) {
	w := newWorld()
	w.turnPlayed = true
	w.With(func() {
		w.World.Config.IBBS = true
		p := w.Player()
		p.Protection = 0 // to form the party; the shield goes back on below
		p.Troopers = 10_000
		w.World.CreateGroupAttack(p, "Mars", "", game.GroupAttackHoursMax,
			game.AttackForce{Troopers: 1000})
		p.Protection = 5
	})
	f := &fakeSession{keys: []rune(" ")}
	joinGroupAttack(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Mars") {
		t.Fatalf("the table of forming parties was not drawn:\n%s", out)
	}
	assertOnlyRefusal(t, out, "Sorry... You're under new realm protection.")
	if strings.Contains(out, "Join which group?") {
		t.Errorf("asked which party to join while shielded:\n%s", out)
	}
}
