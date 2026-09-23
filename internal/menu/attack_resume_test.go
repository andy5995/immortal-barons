package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// runTurnUntilEnd plays runTurn the way GameLoop does, catching the unwind a
// boot or a dropped connection makes through session.End.
func runTurnUntilEnd(s session.Session, w *ctx) (err error) {
	defer session.GuardEnd(&err)
	runTurn(s, w)
	return nil
}

// One local attack a turn must hold even when the session ends straight after
// the strike, at the report's pause or the captured-region picker: the strike
// is saved there, but the turn is not over. The stage has to be charged in the
// same save as the strike, or the next login resumes the turn at the Attack
// menu and offers a second attack on the same turn.
func TestAttackCannotBeRepeatedByLeavingAfterIt(t *testing.T) {
	// "    00" is the four pauses (Queen's refund, income, status, maintenance
	// paid), then Quit Bank and Quit Spending: the Attack menu is next.
	const toAttack = "    00"
	cases := []struct {
		name     string
		troopers int    // 0 keeps the starting army
		keys     string // from the Attack menu to the strike
		report   string // printed only once the strike has been applied
		endsAt   string // the prompt the session ends at, after the report
	}{
		// target A, full force, confirm
		{"regular loss", 0, "RA\r\r\r\r\r", "Defeat!", "Paused<«─"},
		// a win ends at the captured-region picker instead of the pause
		{"regular win", 100_000, "RA\r\r\r\r\r", "You captured", "Regions left] Your choice?"},
		{"nuclear", 0, "NAy", "Nuclear strike!", "Paused<«─"}, // target A, buy the missile
		{"chemical", 0, "CAy", "Chemical strike!", "Paused<«─"},
		{"biological", 0, "BAy", "Biological strike!", "Paused<«─"},
		// faction 1, 100 troopers, no jets or tanks
		{"pirates", 0, "P1100\r\r\r", "Your raid on the Humans", "Paused<«─"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld()
			// A second rival, so a crushing win still leaves a realm to target
			// and the menu an attack to offer.
			w.AddHuman("victim", "Victimville")
			for _, e := range w.Empires {
				e.Protection = 0
			}
			p := w.Player()
			p.Prefs.AutoPayMaint = true
			p.Gold = 2_000_000_000
			if tc.troopers > 0 {
				p.Troopers = tc.troopers
			}
			left := p.TurnsLeft

			f := &fakeSession{keys: []rune(toAttack + tc.keys), boot: true}
			if err := runTurnUntilEnd(f, w); err == nil {
				t.Fatal("the first session should have ended at the boot")
			}
			// The script must have REACHED the strike, and the session must have
			// ended after its report rather than at some earlier prompt.
			out1 := stripANSI(f.out.String())
			at := strings.Index(out1, tc.report)
			if at < 0 {
				t.Fatalf("the strike was never applied; got:\n%s", out1)
			}
			if !strings.HasSuffix(strings.TrimSpace(out1[at:]), tc.endsAt) {
				t.Fatalf("the session should end at %q after the report; got:\n%s", tc.endsAt, out1[at:])
			}
			if p.TurnsLeft != left {
				t.Fatalf("the interrupted turn should not be charged yet: TurnsLeft %d -> %d", left, p.TurnsLeft)
			}

			// A fresh login on the same world resumes the interrupted turn. It must
			// go past the Attack menu to the end of the turn.
			// The strike's news was filed in the same save as the strike; the
			// resumed turn must not file it again.
			news1 := map[string]bool{}
			for _, n := range w.NewsToday {
				news1[n.Text] = true
			}
			w2 := &ctx{World: w.World, handle: w.handle, Term: w.Term}
			f2 := &fakeSession{keys: []rune(" n"), boot: true} // status pause, decline the next turn
			runTurnUntilEnd(f2, w2)
			out := stripANSI(f2.out.String())
			if strings.Contains(out, "[Attack]") {
				t.Errorf("a second attack was offered on the same turn:\n%s", out)
			}
			if !strings.Contains(out, "Continue to your next turn?") {
				t.Errorf("the resumed turn never reached its end:\n%s", out)
			}
			if got := w2.Player().TurnsLeft; got != left-1 {
				t.Errorf("the resumed turn should be charged once: TurnsLeft %d -> %d", left, got)
			}
			seen := map[string]int{}
			for _, n := range w.NewsToday {
				if news1[n.Text] {
					seen[n.Text]++
				}
			}
			for text, n := range seen {
				if n > 1 {
					t.Errorf("news filed %d times after the resume: %q", n, text)
				}
			}
		})
	}
}

// A strike that never happened must not charge the stage: backing out of an
// attack at any of its prompts leaves the Attack menu open for the turn.
func TestCancelledAttackLeavesTheStageOpen(t *testing.T) {
	cases := []struct{ keys, reached string }{
		{"N\r", "Choose a target"},           // no target picked
		{"NAn", "Buy it?"},                   // missile declined
		{"R\r", "Attack which realm?"},       // no target picked
		{"RA\r\r\r\rn", "Send this Attack?"}, // attack not sent
		{"P0", "Nightjackals"},               // no faction picked
	}
	for _, tc := range cases {
		w := newWorld()
		for _, e := range w.Empires {
			e.Protection = 0
		}
		w.Player().Gold = 2_000_000_000
		f := &fakeSession{keys: []rune(tc.keys)}
		Run(f, w, BuildMenus().Attack)
		if !strings.Contains(stripANSI(f.out.String()), tc.reached) {
			t.Fatalf("%q: never reached %q; got:\n%s", tc.keys, tc.reached, f.out.String())
		}
		if w.Player().TurnProgress.AttackDone {
			t.Errorf("%q: an attack that was called off marked the stage done", tc.keys)
		}
	}
}
