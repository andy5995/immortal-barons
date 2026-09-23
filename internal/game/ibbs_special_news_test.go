package game

import (
	"math/rand"
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
	landNextBombingRun(w)
	res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood})
	if res.Won || news != "Bombers from Selby of Home hit the planet's food market and found it bare." {
		t.Errorf("bare market: won=%v news %q", res.Won, news)
	}

	w.FoodMarketSupply = 10_000
	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 2, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood})
	if !res.Won || news != "Bombers from Selby of Home hit the planet's food market." {
		t.Errorf("stocked market: won=%v news %q", res.Won, news)
	}
	if w.FoodMarketSupply < 100 || w.FoodMarketSupply > 8_000 {
		t.Errorf("stocked market: supply %d after the hit, want 1-80%% of 10000 left", w.FoodMarketSupply)
	}

	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 3, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombMarket})
	if res.Won || news != "Bombers from Selby of Home hit the planet's trading market and found nothing listed there." {
		t.Errorf("empty trading market: won=%v news %q", res.Won, news)
	}

	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 4, FromBoard: "Home", FromEmpire: "Selby", Op: OpUndermine})
	if res.Won || news != "Bombers from Selby of Home found nothing invested in the planet's bank to undermine." {
		t.Errorf("nothing invested: won=%v news %q", res.Won, news)
	}
}

// A bombing run has two ways to come to nothing, and they are reported apart:
// the landing roll that turns most runs back, and a target with nothing in it.
// Both must be reached for the test to mean anything.
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

// Every one of the four bombing ops meets the landing roll, not Bomb Trade Routes
// alone: the original rolls Random(3) ahead of its switch on the op type
// (resolve_received_bombing, BRE.OVR 0x04a09a +0x11b). A run that fails it
// leaves the planet exactly as it was and posts the driven-off line, naming the
// sender. Over many seeds, about two runs in three must fail — a property of
// the roll, so it holds on any run of seeds, not one trajectory.
func TestEveryBombingOpMeetsTheLandingRoll(t *testing.T) {
	const runs = 300
	for _, tc := range []struct {
		op     SpecialOp
		object string
	}{
		{OpBombFood, "the planet's food market"},
		{OpBombMarket, "the planet's trading market"},
		{OpBombRoutes, "the planet's trade routes"},
		{OpUndermine, "the planet's bank"},
	} {
		drivenOff := 0
		for seed := int64(1); seed <= runs; seed++ {
			w, d := specialNewsBoard(seed)
			w.FoodMarketSupply = 10_000
			d.Investments = []Investment{{Amount: 1000, Return: 1200, MaturesDay: w.GameDay + 3}}
			res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: tc.op})
			if !strings.Contains(res.Report, "driven off") {
				continue
			}
			drivenOff++
			if res.Won {
				t.Errorf("%s seed %d: a run driven off reported a win", tc.op, seed)
			}
			if want := "Bombers from Selby of Home were driven off before they reached " + tc.object + "."; news != want {
				t.Errorf("%s seed %d: news %q, want %q", tc.op, seed, news, want)
			}
			if w.FoodMarketSupply != 10_000 || d.Investments[0].Amount != 1000 {
				t.Errorf("%s seed %d: a run driven off still did damage", tc.op, seed)
			}
		}
		// 2/3 of 300 is 200, with a standard deviation near 8.
		if drivenOff < 160 || drivenOff > 240 {
			t.Errorf("%s: %d of %d runs driven off, want about two in three", tc.op, drivenOff, runs)
		}
	}
}

// The firer's planet reads a line for every outcome of a missile it sent, and
// the line agrees with the one the target's planet read (#288). The original's
// return path posts on both of its branches (process_sabre_return, BRE.OVR
// 0x046045 +0x1051 and +0x1107); IB posted for a backfire alone. Each seed is
// classified by the TARGET's line, which the test does not construct, and every
// outcome must be reached.
func TestFiringPlanetReadsEveryMissileOutcome(t *testing.T) {
	for _, tc := range []struct {
		op   SpecialOp
		prep func(*Empire)
		want map[string]string // target planet's line -> firer planet's line
	}{
		{OpNuclear, func(d *Empire) { d.SDI = 60 }, map[string]string{
			"The Nuclear Assault from Alpha Baron of Alpha BBS misfired on its way to Bravo Hold.": "Alpha Baron's Nuclear Assault misfired on its way to Bravo Hold of Bravo BBS.",
			"Bravo Hold's SDI shot down the Nuclear Assault from Alpha Baron of Alpha BBS.":        "The SDI of Bravo Hold of Bravo BBS shot down Alpha Baron's Nuclear Assault.",
			"The Nuclear Assault from Alpha Baron of Alpha BBS hit Bravo Hold.":                    "Alpha Baron's Nuclear Assault hit Bravo Hold of Bravo BBS.",
		}},
		{OpSabre, func(d *Empire) { d.SDI = 0; d.Troopers = 100 * SabreBackfireScale / 2 }, map[string]string{
			"The S3-Sabre from Alpha Baron of Alpha BBS broke up over Bravo Hold.":               "Alpha Baron's S3-Sabre broke up over Bravo Hold of Bravo BBS.",
			"The S3-Sabre from Alpha Baron of Alpha BBS reached Bravo Hold and did little harm.": "Alpha Baron's S3-Sabre reached Bravo Hold of Bravo BBS and did little harm.",
			"The S3-Sabre from Alpha Baron of Alpha BBS hit Bravo Hold.":                         "Alpha Baron's S3-Sabre hit Bravo Hold of Bravo BBS.",
		}},
	} {
		seen := map[string]bool{}
		for seed := int64(1); seed <= 300 && len(seen) < len(tc.want); seed++ {
			from, to, attacker, target := specialOpWorlds(t)
			to.rng = rand.New(rand.NewSource(seed))
			tc.prep(target)
			// Dial 6 aims at the airbases, and a realm with no jets there loses
			// nothing, so a Sabre that lands and misses reaches negligible damage.
			if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, tc.op, 6); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			answer := to.ApplyPacket(from.Outbox[0])
			there := to.NewsToday[len(to.NewsToday)-1].Text
			before := len(from.NewsToday)
			from.applyAttackResult(answer.Results[0])
			here := from.NewsToday[before:]
			want, known := tc.want[there]
			if !known {
				continue
			}
			seen[there] = true
			if len(here) != 1 || here[0].Text != want {
				t.Errorf("%s seed %d, target read %q:\n firer read %v\n want %q", tc.op, seed, there, here, want)
			}
		}
		for line := range tc.want {
			if !seen[line] {
				t.Errorf("%s: no seed in 300 produced %q, so the firer's line for it went unchecked", tc.op, line)
			}
		}
	}
}

// A missile turned aside by New Realm Protection is news on the firer's planet
// too, and a result from a board that predates the narrower verdicts gets a line
// that claims no more than "failure".
func TestFiringPlanetReadsProtectionAndOldFailures(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	target.Protection = 5
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpChemical, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	answer := to.ApplyPacket(from.Outbox[0])
	before := len(from.NewsToday)
	from.applyAttackResult(answer.Results[0])
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "New Realm Protection turned aside Alpha Baron's Chemical Bombing against Bravo Hold of Bravo BBS." {
		t.Errorf("protected: firer read %v", got)
	}

	from, _, attacker, target = specialOpWorlds(t)
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpNuclear, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	before = len(from.NewsToday)
	from.applyAttackResult(AttackResult{
		ID: from.InFlight[0].ID, TargetBoard: "Bravo BBS", TargetEmpire: target.Name,
		Kind: string(OpNuclear), Outcome: OutcomeRepelled, Report: "It failed.",
	})
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "Alpha Baron's Nuclear Assault against Bravo Hold of Bravo BBS failed." {
		t.Errorf("legacy failure: firer read %v", got)
	}
}

// A bombing op never reaches the per-realm resolver, even from a packet that
// names a realm: TargetsPlanet routes every op that is not a missile down the
// planet path first, which is why applySpecialOp no longer carries branches for
// them. The named realm is protected, so a per-realm path would have answered
// "protected"; the planet path ignores both the name and the shield.
func TestBombingOpsNeverReachTheRealmResolver(t *testing.T) {
	for _, op := range []SpecialOp{OpBombFood, OpBombMarket, OpBombRoutes, OpUndermine} {
		w, d := specialNewsBoard(1)
		d.Protection = 99
		res, news := resolveOneSpecial(t, w, RemoteSpecialOp{
			ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: d.Name, Op: op,
		})
		if res.Outcome == OutcomeProtected || res.Outcome == OutcomeNotFound {
			t.Errorf("%s answered %q: it went looking for a realm", op, res.Outcome)
		}
		if !strings.HasPrefix(news, "Bombers from Selby of Home ") {
			t.Errorf("%s posted %q, which is not a planet-wide line", op, news)
		}
	}
}

// Every line a bombing op posts names the same carrier, whichever op and however
// it ended: the original's one failure line for all four has forces caught
// planting bombs, and its Undermine success line names forces too
// (game/ipreport.dat, ^BOMBING_HITS). IB's Undermine lines said "Agents" while
// its failure line said "Bombers", so one op read as two different raids.
func TestBombingNewsNamesOneCarrier(t *testing.T) {
	landed, drivenOff := false, false
	for seed := int64(1); seed <= 30; seed++ {
		w, d := specialNewsBoard(seed)
		d.Investments = []Investment{{Amount: 1000, Return: 1200, MaturesDay: w.GameDay + 3}}
		before := len(d.Events)
		res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpUndermine})
		landed = landed || res.Won
		drivenOff = drivenOff || strings.Contains(res.Report, "driven off")
		if !strings.HasPrefix(news, "Bombers from Selby of Home ") {
			t.Errorf("seed %d: news %q", seed, news)
		}
		for _, ev := range d.Events[before:] {
			if !strings.HasPrefix(ev.Text, "Bombers from Selby of Home ") {
				t.Errorf("seed %d: event %q", seed, ev.Text)
			}
		}
	}
	if !landed || !drivenOff {
		t.Errorf("30 seeds reached landed=%v, driven off=%v; both lines must be checked", landed, drivenOff)
	}
}

// The firer's planet reads a line for every outcome of a bombing run it sent,
// agreeing with the target's line (#288): the original's return path reaches
// its news call on every path (process_bombing_results, BRE.OVR 0x04a4a6
// +0x0622). Each seed is classified by the TARGET's line, and a stocked and an
// empty planet between them must reach all three outcomes for every op.
func TestFiringPlanetReadsEveryBombingOutcome(t *testing.T) {
	for _, tc := range []struct {
		op                    SpecialOp
		hitThere, hitHere     string
		emptyThere, emptyHere string
		object                string
	}{
		{OpBombFood,
			"Bombers from Alpha Baron of Alpha BBS hit the planet's food market.", "Alpha Baron's bombers hit Bravo BBS's food market.",
			"Bombers from Alpha Baron of Alpha BBS hit the planet's food market and found it bare.", "Alpha Baron's bombers found nothing to destroy at Bravo BBS's food market.",
			"food market"},
		{OpBombMarket,
			"Bombers from Alpha Baron of Alpha BBS wrecked the planet's trading market.", "Alpha Baron's bombers wrecked Bravo BBS's trading market.",
			"Bombers from Alpha Baron of Alpha BBS hit the planet's trading market and found nothing listed there.", "Alpha Baron's bombers found nothing to destroy at Bravo BBS's trading market.",
			"trading market"},
		{OpBombRoutes,
			"Bombers from Alpha Baron of Alpha BBS hit trade routes across the planet.", "Alpha Baron's bombers hit trade routes across Bravo BBS.",
			"Bombers from Alpha Baron of Alpha BBS found nothing moving on the planet's trade routes.", "Alpha Baron's bombers found nothing to destroy at Bravo BBS's trade routes.",
			"trade routes"},
		{OpUndermine,
			"Bombers from Alpha Baron of Alpha BBS undermined investments across the planet.", "Alpha Baron's bombers undermined investments across Bravo BBS.",
			"Bombers from Alpha Baron of Alpha BBS found nothing invested in the planet's bank to undermine.", "Alpha Baron's bombers found nothing to destroy at Bravo BBS's bank.",
			"bank"},
	} {
		drivenThere := "Bombers from Alpha Baron of Alpha BBS were driven off before they reached the planet's " + tc.object + "."
		want := map[string]string{
			tc.hitThere:   tc.hitHere,
			tc.emptyThere: tc.emptyHere,
			drivenThere:   "Alpha Baron's bombers were driven off before they reached Bravo BBS's " + tc.object + ".",
		}
		seen := map[string]bool{}
		for seed := int64(1); seed <= 400 && len(seen) < len(want); seed++ {
			from, to, attacker, target := specialOpWorlds(t)
			to.rng = rand.New(rand.NewSource(seed))
			if seed%2 == 0 { // stocked: something for every op to find
				to.FoodMarketSupply = 10_000
				target.Tanks = 100
				if err := to.SetMarketListing(target, "Tank", 100, 1000); err != nil {
					t.Fatalf("list: %v", err)
				}
				target.Investments = []Investment{{Amount: 1000, Return: 1200, MaturesDay: to.GameDay + 3}}
				partner := to.AddHuman("p", "Partner")
				for range 12 {
					partner.TradeDeals = append(partner.TradeDeals, TradeDeal{From: target.Name, Send: TradeBasket{Troopers: 1000}})
				}
			} else {
				to.FoodMarketSupply = 0
			}
			if err := from.SendSpecialOp(attacker, "Bravo BBS", "", tc.op, 0); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			answer := to.ApplyPacket(from.Outbox[0])
			there := to.NewsToday[len(to.NewsToday)-1].Text
			before := len(from.NewsToday)
			from.applyAttackResult(answer.Results[0])
			here := from.NewsToday[before:]
			line, known := want[there]
			if !known {
				t.Fatalf("%s seed %d: unexpected target line %q", tc.op, seed, there)
			}
			seen[there] = true
			if len(here) != 1 || here[0].Text != line {
				t.Errorf("%s seed %d, target read %q:\n firer read %v\n want %q", tc.op, seed, there, here, line)
			}
		}
		for line := range want {
			if !seen[line] {
				t.Errorf("%s: no seed reached %q", tc.op, line)
			}
		}
	}
}

// A bombing result from a board that predates the narrower verdicts carries a
// plain "failure", and the firer's line claims no more than that the run came
// to nothing.
func TestFiringPlanetReadsAnOldBombingFailure(t *testing.T) {
	from, _, attacker, _ := specialOpWorlds(t)
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "", OpBombMarket, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	before := len(from.NewsToday)
	from.applyAttackResult(AttackResult{
		ID: from.InFlight[0].ID, TargetBoard: "Bravo BBS",
		Kind: string(OpBombMarket), Outcome: OutcomeRepelled, Report: "It failed.",
	})
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "Alpha Baron's Bomb Trading Market against Bravo BBS came to nothing." {
		t.Errorf("legacy failure: firer read %v", got)
	}
}
