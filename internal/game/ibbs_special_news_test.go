package game

import (
	"math/rand"
	"regexp"
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
// it posted, failing the test if it posted more than one. A missile that
// misfired posts none, and reads as "".
func resolveOneSpecial(t *testing.T, w *World, op RemoteSpecialOp) (AttackResult, string) {
	t.Helper()
	before := len(w.NewsToday)
	res := w.resolveRemoteSpecialOp(op)
	got := w.NewsToday[before:]
	switch {
	case len(got) == 0 && res.Outcome == OutcomeMisfire:
		return res, ""
	case len(got) != 1:
		t.Fatalf("%s posted %d news lines, want 1: %v", op.Op, len(got), got)
	}
	return res, got[0].Text
}

// The one line an arriving missile posts on the target's planet is chosen by how
// it ended, and agrees with the report the firer reads (#288). Until then every
// outcome posted "X struck Y", so a launch that broke up read as a hit on one
// planet and a failure on the other. A misfire posts nothing: the target reads
// it as a personal event. Each seed is classified by the outcome the target
// board settled, which the test does not construct, and every outcome the loop
// is meant to cover must be reached. {n} is the land a backfire opened.
func TestMissileNewsFollowsTheOutcome(t *testing.T) {
	const backfire = AttackOutcome("backfire")
	for _, tc := range []struct {
		op    SpecialOp
		label string
		want  map[AttackOutcome]string
		prep  func(*Empire)
	}{
		{OpNuclear, "Nuclear Assault", map[AttackOutcome]string{
			OutcomeMisfire:     "",
			OutcomeIntercepted: "Victim's SDI shot down the nuclear missile from Selby of Home.",
			OutcomeWon:         "The nuclear missile from Selby of Home hit Victim.",
		}, func(d *Empire) { d.SDI = 60 }},
		{OpSabre, "S3-Sabre", map[AttackOutcome]string{
			OutcomeMisfire: "",
			backfire:       "The S3-Sabre from Selby of Home backfired, expanding Victim's territory by {n}.",
			OutcomeWon:     "The S3-Sabre from Selby of Home hit Victim.",
		}, func(d *Empire) { d.SDI = 0; d.Troopers = 100 * SabreBackfireScale / 2 }},
		{OpChemical, "Chemical Bombing", map[AttackOutcome]string{
			OutcomeGuarded: "Victim's defenses brought down the chemical missile from Selby of Home.",
			OutcomeWon:     "The chemical missile from Selby of Home hit Victim.",
		}, func(d *Empire) { d.SDI = 0; d.Tanks = (d.Land + 1) * 25_000 }},
	} {
		seen := map[AttackOutcome]bool{}
		for seed := int64(1); seed <= 200 && len(seen) < len(tc.want); seed++ {
			w, d := specialNewsBoard(seed)
			tc.prep(d)
			res, news := resolveOneSpecial(t, w, RemoteSpecialOp{
				ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: "Victim", Op: tc.op, Dial: 5,
			})
			key := res.Outcome
			if res.Backfired {
				key = backfire
			}
			want, ok := tc.want[key]
			if !ok {
				continue
			}
			seen[key] = true
			// The land a backfire opened is parked on the target for its picker.
			want = strings.ReplaceAll(want, "{n}", regionCount(d.PendingRegions))
			if news != want {
				t.Errorf("%s seed %d, report %q:\n news %q\n want %q", tc.label, seed, res.Report, news, want)
			}
		}
		for o := range tc.want {
			if !seen[o] {
				t.Errorf("%s: no seed in 200 ended %q, so its news line went unchecked", tc.label, o)
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
	if res.Won || news != "Bombers from Selby of Home found nothing to destroy on the planet." {
		t.Errorf("bare market: won=%v news %q", res.Won, news)
	}

	w.FoodMarketSupply = 10_000
	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 2, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood})
	if !res.Won || news != "Bombers from Selby of Home burned the planet's food market." {
		t.Errorf("stocked market: won=%v news %q", res.Won, news)
	}
	if w.FoodMarketSupply < 100 || w.FoodMarketSupply > 8_000 {
		t.Errorf("stocked market: supply %d after the hit, want 1-80%% of 10000 left", w.FoodMarketSupply)
	}

	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 3, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombMarket})
	if res.Won || news != "Bombers from Selby of Home found nothing to destroy on the planet." {
		t.Errorf("empty trading market: won=%v news %q", res.Won, news)
	}

	landNextBombingRun(w)
	res, news = resolveOneSpecial(t, w, RemoteSpecialOp{ID: 4, FromBoard: "Home", FromEmpire: "Selby", Op: OpUndermine})
	if res.Won || news != "Bombers from Selby of Home found nothing to destroy on the planet." {
		t.Errorf("nothing invested: won=%v news %q", res.Won, news)
	}
}

// A bombing run has two ways to come to nothing, and they are reported apart:
// the landing roll that turns most runs back, and a target with nothing in it.
// Both must be reached for the test to mean anything.
func TestTradeRouteBombingNewsSeparatesTheTwoFailures(t *testing.T) {
	want := map[AttackOutcome]string{
		OutcomeDrivenOff: "", // one of the bomber pool's lines; see drivenOffNews
		OutcomeNothing:   "Bombers from Selby of Home found nothing to destroy on the planet.",
	}
	seen := map[AttackOutcome]bool{}
	for seed := int64(1); seed <= 50 && len(seen) < len(want); seed++ {
		w, _ := specialNewsBoard(seed)
		res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombRoutes})
		if line, ok := want[res.Outcome]; ok {
			seen[res.Outcome] = true
			if res.Outcome == OutcomeDrivenOff {
				if !drivenOffNews(news, "Selby of Home", "trade routes") {
					t.Errorf("seed %d: driven-off news %q is not from the bomber pool", seed, news)
				}
				continue
			}
			if news != line {
				t.Errorf("seed %d, outcome %q: news %q, want %q", seed, res.Outcome, news, line)
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
		{OpBombFood, "food market"},
		{OpBombMarket, "trading market"},
		{OpBombRoutes, "trade routes"},
		{OpUndermine, "bank"},
	} {
		drivenOff := 0
		for seed := int64(1); seed <= runs; seed++ {
			w, d := specialNewsBoard(seed)
			w.FoodMarketSupply = 10_000
			d.Investments = []Investment{{Amount: 1000, Return: 1200, MaturesDay: w.GameDay + 3}}
			res, news := resolveOneSpecial(t, w, RemoteSpecialOp{ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: tc.op})
			if res.Outcome != OutcomeDrivenOff {
				continue
			}
			drivenOff++
			if res.Won {
				t.Errorf("%s seed %d: a run driven off reported a win", tc.op, seed)
			}
			if !drivenOffNews(news, "Selby of Home", tc.object) {
				t.Errorf("%s seed %d: news %q is not from the bomber pool", tc.op, seed, news)
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
// classified by the outcome the TARGET board settled, which the test does not
// construct, and every outcome must be reached. The firer's backfire line gives
// no count: the land is settled on the target's board.
func TestFiringPlanetReadsEveryMissileOutcome(t *testing.T) {
	const backfire = AttackOutcome("backfire")
	for _, tc := range []struct {
		op   SpecialOp
		prep func(*Empire)
		want map[AttackOutcome]string // outcome -> firer planet's line
	}{
		{OpNuclear, func(d *Empire) { d.SDI = 60 }, map[AttackOutcome]string{
			OutcomeMisfire:     "Alpha Baron's nuclear missile vanished on the way to Bravo BBS.",
			OutcomeIntercepted: "Bravo Hold's SDI shot down Alpha Baron's nuclear missile.",
			OutcomeWon:         "Alpha Baron's nuclear missile hit Bravo Hold of Bravo BBS.",
		}},
		{OpNuclear, func(d *Empire) { d.SDI = 0; d.Turrets = (d.Land + 1) * 25_000 }, map[AttackOutcome]string{
			OutcomeGuarded: "Bravo Hold's defenses brought down Alpha Baron's nuclear missile.",
		}},
		{OpSabre, func(d *Empire) { d.SDI = 0; d.Troopers = 100 * SabreBackfireScale / 2 }, map[AttackOutcome]string{
			backfire:          "Alpha Baron's S3-Sabre backfired on Bravo Hold of Bravo BBS.",
			OutcomeNegligible: "Alpha Baron's S3-Sabre barely scratched Bravo Hold of Bravo BBS.",
			OutcomeWon:        "Alpha Baron's S3-Sabre hit Bravo Hold of Bravo BBS.",
		}},
	} {
		seen := map[AttackOutcome]bool{}
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
			res := answer.Results[0]
			before := len(from.NewsToday)
			from.applyAttackResult(res, nil)
			here := from.NewsToday[before:]
			key := res.Outcome
			if res.Backfired {
				key = backfire
			}
			want, known := tc.want[key]
			if !known {
				continue
			}
			seen[key] = true
			if len(here) != 1 || here[0].Text != want {
				t.Errorf("%s seed %d, outcome %q:\n firer read %v\n want %q", tc.op, seed, key, here, want)
			}
		}
		for o := range tc.want {
			if !seen[o] {
				t.Errorf("%s: no seed in 300 ended %q, so the firer's line for it went unchecked", tc.op, o)
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
	from.applyAttackResult(answer.Results[0], nil)
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "New Realm Protection turned aside Alpha Baron's chemical missile." {
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
	}, nil)
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "Alpha Baron's nuclear missile against Bravo Hold of Bravo BBS failed." {
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
		drivenOff = drivenOff || res.Outcome == OutcomeDrivenOff
		if !strings.HasPrefix(news, "Bombers from Selby of Home ") &&
			!(res.Outcome == OutcomeDrivenOff && drivenOffNews(news, "Selby of Home", "bank")) {
			t.Errorf("seed %d: news %q", seed, news)
		}
		// The planet reads it in the news and nowhere else: the original's
		// receiver never calls the per-realm event writer.
		if got := d.Events[before:]; len(got) != 0 {
			t.Errorf("seed %d: a realm was told in person: %v", seed, got)
		}
	}
	if !landed || !drivenOff {
		t.Errorf("30 seeds reached landed=%v, driven off=%v; both lines must be checked", landed, drivenOff)
	}
}

// The firer's planet reads a line for every outcome of a bombing run it sent,
// agreeing with the target's line (#288): the original's return path reaches
// its news call on every path (process_bombing_results, BRE.OVR 0x04a4a6
// +0x0622). Each seed is classified by the outcome the TARGET board settled,
// and a stocked and an empty planet between them must reach all three outcomes
// for every op. A driven-off run's target line is one of the bomber pool's.
func TestFiringPlanetReadsEveryBombingOutcome(t *testing.T) {
	shareSuffix := regexp.MustCompile(`^ \d+% of (the supply burned|every listing was destroyed|the investments coming due was lost)\.$`)
	for _, tc := range []struct {
		op                SpecialOp
		hitThere, hitHere string
		object            string
	}{
		{OpBombFood,
			"Bombers from Alpha Baron of Alpha BBS burned the planet's food market.", "Alpha Baron's bombers burned Bravo BBS's food market.",
			"food market"},
		{OpBombMarket,
			"Bombers from Alpha Baron of Alpha BBS wrecked the planet's trading market.", "Alpha Baron's bombers wrecked Bravo BBS's trading market.",
			"trading market"},
		{OpBombRoutes,
			"Bombers from Alpha Baron of Alpha BBS hit trade routes across the planet.", "Alpha Baron's bombers hit trade routes across Bravo BBS.",
			"trade routes"},
		{OpUndermine,
			"Bombers from Alpha Baron of Alpha BBS undermined the planet's bank.", "Alpha Baron's bombers undermined Bravo BBS's bank.",
			"bank"},
	} {
		want := map[AttackOutcome][2]string{
			OutcomeWon:       {tc.hitThere, tc.hitHere},
			OutcomeNothing:   {"Bombers from Alpha Baron of Alpha BBS found nothing to destroy on the planet.", "Alpha Baron's bombers found nothing to destroy on Bravo BBS."},
			OutcomeDrivenOff: {"", "Alpha Baron's bombers were turned back before they reached Bravo BBS."},
		}
		seen := map[AttackOutcome]bool{}
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
			res := answer.Results[0]
			there := to.NewsToday[len(to.NewsToday)-1].Text
			before := len(from.NewsToday)
			from.applyAttackResult(res, nil)
			here := from.NewsToday[before:]
			lines, known := want[res.Outcome]
			if !known {
				t.Fatalf("%s seed %d: unexpected outcome %q", tc.op, seed, res.Outcome)
			}
			seen[res.Outcome] = true
			if res.Outcome == OutcomeDrivenOff {
				if !drivenOffNews(there, "Alpha Baron of Alpha BBS", tc.object) {
					t.Errorf("%s seed %d: target read %q, not a bomber-pool line", tc.op, seed, there)
				}
			} else if there != lines[0] {
				t.Errorf("%s seed %d: target read %q, want %q", tc.op, seed, there, lines[0])
			}
			// A landed run's line carries the share the target reported, as the
			// original's success line names it; the trade routes name none.
			if res.Outcome == OutcomeWon && len(here) == 1 && tc.op != OpBombRoutes {
				if !shareSuffix.MatchString(strings.TrimPrefix(here[0].Text, lines[1])) {
					t.Errorf("%s seed %d: firer read %q, want %q and the share", tc.op, seed, here[0].Text, lines[1])
				}
				continue
			}
			if len(here) != 1 || here[0].Text != lines[1] {
				t.Errorf("%s seed %d, outcome %q:\n firer read %v\n want %q", tc.op, seed, res.Outcome, here, lines[1])
			}
		}
		for o := range want {
			if !seen[o] {
				t.Errorf("%s: no seed reached %q", tc.op, o)
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
	}, nil)
	if got := from.NewsToday[before:]; len(got) != 1 ||
		got[0].Text != "Alpha Baron's Bomb Trading Market against Bravo BBS came to nothing." {
		t.Errorf("legacy failure: firer read %v", got)
	}
}

// Every outcome a Special Operation can end in has its own line on BOTH planets:
// the target's (missileNews, planetOpNews) and the firer's once the answer comes
// home (missileReturnNews, bombingReturnNews). The two sides are separate
// switches in separate files, so an outcome handled on one side only would fall
// through to the other's catch-all line and the planets would disagree. The
// seeded tests above reach only the outcomes their seeds happen to produce; this
// one walks every value.
func TestEveryOutcomeHasANewsLineOnBothPlanets(t *testing.T) {
	missile := map[specialOutcome]bool{specialHit: true, specialNothing: true,
		specialMisfire: true, specialIntercepted: true, specialBackfire: true, specialGuarded: true}
	planet := map[specialOutcome]bool{specialHit: true, specialNothing: true, specialDrivenOff: true}
	for o := specialOutcome(0); o < specialOutcomeCount; o++ {
		if !missile[o] && !planet[o] {
			t.Errorf("outcome %d is in neither family; say which ops can end in it", o)
		}
	}

	legacy := AttackResult{Outcome: OutcomeRepelled}
	for _, op := range []SpecialOp{OpNuclear, OpChemical, OpSabre} {
		missileName := missileNoun(op)
		there, here := map[string]specialOutcome{}, map[string]specialOutcome{}
		fallback := missileReturnNews("F", missileName, "T", "B", legacy)
		for o := range missile {
			res := AttackResult{Outcome: missileOutcome(o), Backfired: o == specialBackfire}
			a, b := missileNews(op, "F", "T", o, 3), missileReturnNews("F", missileName, "T", "B", res)
			// A misfire is the target's personal event, not its planet's news.
			if o == specialMisfire && a == "" {
				a = "(misfire: no line)"
			}
			if a == "" || b == "" {
				t.Errorf("%s outcome %d: target %q, firer %q", op, o, a, b)
			}
			if b == fallback {
				t.Errorf("%s outcome %d: the firer reads the legacy failure line %q", op, o, b)
			}
			if prev, dup := there[a]; dup {
				t.Errorf("%s outcomes %d and %d share the target's line %q", op, prev, o, a)
			}
			if prev, dup := here[b]; dup {
				t.Errorf("%s outcomes %d and %d share the firer's line %q", op, prev, o, b)
			}
			there[a], here[b] = o, o
		}
	}
	for _, op := range []SpecialOp{OpBombFood, OpBombMarket, OpBombRoutes, OpUndermine} {
		there, here := map[string]specialOutcome{}, map[string]specialOutcome{}
		fallback := bombingReturnNews("F", op, "B", legacy)
		for o := range planet {
			a := planetOpNews(op, "F", o, func() string { return bomberDrivenOffPool[0] })
			b := bombingReturnNews("F", op, "B", AttackResult{Outcome: planetOpOutcome(o)})
			if a == "" || b == "" {
				t.Errorf("%s outcome %d: target %q, firer %q", op, o, a, b)
			}
			if b == fallback {
				t.Errorf("%s outcome %d: the firer reads the legacy failure line %q", op, o, b)
			}
			if prev, dup := there[a]; dup {
				t.Errorf("%s outcomes %d and %d share the target's line %q", op, prev, o, a)
			}
			if prev, dup := here[b]; dup {
				t.Errorf("%s outcomes %d and %d share the firer's line %q", op, prev, o, b)
			}
			there[a], here[b] = o, o
		}
	}
}

// Who hears of a missile in person, outcome by outcome, as the original's two
// routines decide it. On the target's board (resolve_received_sabre_strike
// +0x0d1f..+0x0d8a) every outcome reaches the news, and the realm is told in
// person of everything but an SDI interception. On the firer's board
// (process_sabre_return +0x1056) a failed S3-Sabre is news alone, while a
// nuclear or chemical strike, or a Sabre that landed, is told in person too.
// Seeds are classified by outcome and every outcome must be reached.
func TestMissileOutcomesReachTheOriginalsChannels(t *testing.T) {
	for _, op := range []SpecialOp{OpNuclear, OpSabre} {
		seen := map[AttackOutcome]bool{}
		for seed := int64(1); seed <= 2000 && len(seen) < 4; seed++ {
			from, to, attacker, target := specialOpWorlds(t)
			to.rng = rand.New(rand.NewSource(seed))
			target.SDI = 60
			target.Turrets = (target.Land + 1) * 25_000
			if op == OpSabre {
				// Troopers are both the Sabre's garrison and IB's backfire
				// trigger, so a garrison big enough to stop it would make every
				// launch that gets past it backfire. A small realm with a
				// modest army reaches both.
				target.Regions = RegionMix{Desert: 9}
				target.syncLand()
				target.Troopers = 19_000
			}
			if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, op, 6); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			targetEvents, newsThere := len(target.Events), len(to.NewsToday)
			answer := to.ApplyPacket(from.Outbox[0])
			res := answer.Results[0]
			o := res.outcome()
			if res.Backfired {
				continue
			}
			switch o {
			case OutcomeWon, OutcomeNegligible, OutcomeMisfire, OutcomeIntercepted, OutcomeGuarded:
			default:
				continue
			}
			if o == OutcomeWon || o == OutcomeNegligible {
				o = OutcomeWon // the two landed outcomes share their channels
			}
			seen[o] = true
			// A misfire is the target's personal event alone.
			wantThere := 1
			if o == OutcomeMisfire {
				wantThere = 0
			}
			if len(to.NewsToday) != newsThere+wantThere {
				t.Errorf("%s %s: the target's planet read %d lines, want %d", op, o, len(to.NewsToday)-newsThere, wantThere)
			}
			told := len(target.Events) > targetEvents
			if want := o != OutcomeIntercepted; told != want {
				t.Errorf("%s %s: target told in person = %v, want %v", op, o, told, want)
			}
			newsHere := len(from.NewsToday)
			from.applyAttackResult(res, nil)
			if len(from.NewsToday) != newsHere+1 {
				t.Errorf("%s %s: the firer's planet read %d lines, want 1", op, o, len(from.NewsToday)-newsHere)
			}
			told = len(attacker.Events) > 0
			if want := op != OpSabre || o == OutcomeWon; told != want {
				t.Errorf("%s %s: firer told in person = %v, want %v", op, o, told, want)
			}
		}
		if len(seen) < 4 {
			t.Errorf("%s: reached only %v", op, seen)
		}
	}
}
