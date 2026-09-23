package game

import (
	"strings"
	"testing"
)

// specialNewsBoard is a board holding one realm for Special Operations to land
// on, with enough of everything that a hit always destroys something.
func specialNewsBoard(seed int64) (*World, *Empire) {
	cfg := DefaultConfig()
	cfg.BoardID = "Far"
	w := NewWorldSeed(cfg, seed)
	d := w.AddHuman("victim", "Victim")
	d.Protection = 0
	return w, d
}

// resolveOneSpecial lands op on w and returns the result and the one news line
// it posted, failing the test if it posted any other number of lines.
func resolveOneSpecial(t *testing.T, w *World, op RemoteSpecialOp) (AttackResult, string) {
	t.Helper()
	before := len(w.NewsToday)
	res := w.resolveRemoteSpecialOp(op)
	got := w.NewsToday[before:]
	if len(got) != 1 {
		t.Fatalf("%s posted %d news lines, want 1: %v", op.Op, len(got), got)
	}
	return res, got[0].Text
}

// The one line an arriving missile posts on the target's planet is chosen by how
// it ended, and agrees with the report the firer reads (#288). Until then every
// outcome posted "X struck Y", so a launch that broke up read as a hit on one
// planet and a failure on the other. Each seed is classified by the report the
// firer gets, which the test does not construct, and every outcome the loop is
// meant to cover must be reached.
func TestMissileNewsFollowsTheOutcome(t *testing.T) {
	for _, tc := range []struct {
		op    SpecialOp
		label string
		want  map[string]string // report marker -> news line
		prep  func(*Empire)
	}{
		{OpNuclear, "Nuclear Assault", map[string]string{
			"misfired and never reached": "The Nuclear Assault from Selby of Home misfired on its way to Victim.",
			"SDI intercepted":            "Victim's SDI shot down the Nuclear Assault from Selby of Home.",
			"Nuclear strike!":            "The Nuclear Assault from Selby of Home hit Victim.",
		}, func(d *Empire) { d.SDI = 60 }},
		{OpSabre, "S3-Sabre", map[string]string{
			"misfired and never reached": "The S3-Sabre from Selby of Home misfired on its way to Victim.",
			"broke up over":              "The S3-Sabre from Selby of Home broke up over Victim.",
			"Your S3-Sabre hit":          "The S3-Sabre from Selby of Home hit Victim.",
		}, func(d *Empire) { d.SDI = 0; d.Troopers = 100 * SabreBackfireScale / 2 }},
	} {
		seen := map[string]bool{}
		for seed := int64(1); seed <= 200 && len(seen) < len(tc.want); seed++ {
			w, d := specialNewsBoard(seed)
			tc.prep(d)
			res, news := resolveOneSpecial(t, w, RemoteSpecialOp{
				ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: "Victim", Op: tc.op, Dial: 5,
			})
			for marker, want := range tc.want {
				if !strings.Contains(res.Report, marker) {
					continue
				}
				seen[marker] = true
				if news != want {
					t.Errorf("%s seed %d, report %q:\n news %q\n want %q", tc.label, seed, res.Report, news, want)
				}
			}
		}
		for marker := range tc.want {
			if !seen[marker] {
				t.Errorf("%s: no seed in 200 produced %q, so its news line went unchecked", tc.label, marker)
			}
		}
	}
}

// A bombing run on a planet that holds nothing for it posts a line saying so,
// not that it struck — and a run that lands on something says it did (#288).
func TestPlanetOpNewsFollowsTheOutcome(t *testing.T) {
	w, _ := specialNewsBoard(1)
	w.FoodMarketSupply = 0
	res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood})
	if res.Won || news != "Bombers from Selby of Home hit the planet's food market and found it bare." {
		t.Errorf("bare market: won=%v news %q", res.Won, news)
	}

	w.FoodMarketSupply = 10_000
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 2, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood})
	if !res.Won || news != "Bombers from Selby of Home hit the planet's food market." {
		t.Errorf("stocked market: won=%v news %q", res.Won, news)
	}
	if w.FoodMarketSupply != 5_000 {
		t.Errorf("stocked market: supply %d after the hit, want 5000", w.FoodMarketSupply)
	}

	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 3, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombMarket})
	if res.Won || news != "Bombers from Selby of Home hit the planet's trading market and found nothing listed there." {
		t.Errorf("empty trading market: won=%v news %q", res.Won, news)
	}

	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 4, FromBoard: "Home", FromEmpire: "Selby", Op: OpUndermine})
	if res.Won || news != "Agents from Selby of Home found nothing invested in the planet's bank to undermine." {
		t.Errorf("nothing invested: won=%v news %q", res.Won, news)
	}
}

// Bomb Trade Routes has two ways to come to nothing, and they are reported
// apart: the landing roll that turns most runs back, and routes with nothing on
// them. Both must be reached for the test to mean anything.
func TestTradeRouteBombingNewsSeparatesTheTwoFailures(t *testing.T) {
	want := map[string]string{
		"driven off":            "Bombers from Selby of Home were driven off before they reached the planet's trade routes.",
		"Nothing worth hitting": "Bombers from Selby of Home found nothing moving on the planet's trade routes.",
	}
	seen := map[string]bool{}
	for seed := int64(1); seed <= 50 && len(seen) < len(want); seed++ {
		w, _ := specialNewsBoard(seed)
		res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombRoutes})
		for marker, line := range want {
			if strings.Contains(res.Report, marker) {
				seen[marker] = true
				if news != line {
					t.Errorf("seed %d, report %q: news %q, want %q", seed, res.Report, news, line)
				}
			}
		}
	}
	if len(seen) != len(want) {
		t.Errorf("reached %v of the two outcomes in 50 seeds", seen)
	}
}

// No realm of that name: nothing happened on this planet, and nothing is posted.
func TestSpecialOpAgainstNoSuchRealmPostsNothing(t *testing.T) {
	w, _ := specialNewsBoard(1)
	before := len(w.NewsToday)
	res := w.resolveRemoteSpecialOp(RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: "Nobody", Op: OpNuclear})
	if res.Outcome != OutcomeNotFound {
		t.Fatalf("outcome %v, want not found", res.Outcome)
	}
	if got := w.NewsToday[before:]; len(got) != 0 {
		t.Errorf("posted %v", got)
	}
}
