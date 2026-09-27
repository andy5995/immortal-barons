package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The desk counts down to the weapon's own instant, in the same short form the
// group-attack table uses. It read "in 0 hours" for the whole of launch day
// while the schedule was a game day, which is what sent us looking.
func TestGooieDeskCountsDownToTheInstant(t *testing.T) {
	for _, c := range []struct {
		name string
		at   time.Time
		want string
	}{
		{"hours out", time.Now().Add(2 * time.Hour), "in 2h"},
		{"minutes out", time.Now().Add(90 * time.Minute), "in 90m"},
		{"its hour has come", time.Now().Add(-time.Hour), "on the next run"},
	} {
		w := newWorld()
		w.With(func() {
			w.World.Config.GooieKablooie = true
			w.Player().Protection = 0
			w.World.Annihilator = &game.Annihilator{
				TargetBoard: "Mars", Funded: true, Intact: 100,
				CostMillion: 10, PaidMillion: 10, LaunchAt: c.at,
			}
		})
		f := &fakeSession{keys: []rune("\r\r")}
		gooieKablooie(f, w)

		out := stripANSI(f.out.String())
		if !strings.Contains(out, "The Gooie Kablooie is complete") {
			t.Fatalf("%s: never reached the funded branch:\n%s", c.name, out)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.want, out)
		}
		if strings.Contains(out, "in 0 hours") {
			t.Errorf("%s: the day-grained countdown is back:\n%s", c.name, out)
		}
	}
}

// The funding desk as the original draws it (cap/eots-ibbs-03.cap): a blank line
// before each status line, fifteen spaces before "Cost Left:", and a prompt
// capped by the gold in hand — 105 million here, against 327 still needed.
func TestGooieDeskMatchesTheOriginalsScreen(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.GooieKablooie = true
		p := w.Player()
		p.Protection, p.Gold = 0, 105_576_974
		// The record names the builder by handle; the screen shows the realm.
		builder := w.World.AddHuman("bran", "Bran The Warrior")
		w.World.Annihilator = &game.Annihilator{
			TargetBoard: "Starship Junkyard", Creator: builder.Owner, Intact: 100,
			CostMillion: 481, PaidMillion: 154,
		}
	})
	f := &fakeSession{keys: []rune("0\r")}
	gooieKablooie(f, w)

	want := "\nTarget:     Starship Junkyard\n" +
		"\nTotal Cost: 481 mil gold               Cost Left: 327 mil gold\n" +
		"\nCreator:    Bran The Warrior\n" +
		"\nHow many Million Gold do you wish to put in? (0; 105):"
	if out := stripANSI(f.out.String()); !strings.Contains(out, want) {
		t.Fatalf("got:\n%q\nwant:\n%q", out, want)
	}
}

// A payment that leaves the weapon short prints nothing, as in the original;
// the payment still lands.
func TestGooiePartialFundingIsSilent(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.GooieKablooie = true
		p := w.Player()
		p.Protection, p.Gold = 0, 50_000_000
		w.World.Annihilator = &game.Annihilator{
			TargetBoard: "Mars", Creator: "x", Intact: 100, CostMillion: 481, PaidMillion: 154,
		}
	})
	f := &fakeSession{keys: []rune("10\r")}
	gooieKablooie(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "(0; 50):") {
		t.Fatalf("never reached the funding prompt:\n%s", out)
	}
	if tail := out[strings.Index(out, "(0; 50):"):]; strings.Contains(tail, "million") || strings.Contains(tail, "complete") {
		t.Errorf("a partial payment should print nothing after the prompt:\n%s", tail)
	}
	w.Read(func() {
		if got := w.World.Annihilator.PaidMillion; got != 164 {
			t.Errorf("paid = %d, want 164", got)
		}
	})
}
