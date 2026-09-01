package game

import (
	"errors"
	"strings"
	"testing"
)

// A spy that gets through reports on the target and costs nothing. No agent
// count guarantees this — BRE's roll ignores both sides' agents (see
// covertRoll) — so the test spies until one lands.
func TestSendSpySuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0

	for i := 0; i < 50; i++ {
		before := a.Agents
		report, err := w.SendSpy(a, d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(report, d.Name) {
			continue // caught; try again
		}
		if a.Agents != before {
			t.Errorf("a spy that got through cost an agent: %d, want %d", a.Agents, before)
		}
		return
	}
	t.Fatal("50 spies in a row were all caught; the roll is not landing near 55%")
}

// covertTries bounds the retry loops in this file. EVERY local covert op is now
// resolved by BRE's own roll, which neither realm's agent count moves
// (covertRoll), so a test that needs one branch has to ask repeatedly rather
// than stack agents. The worst case is Bribery at 32.5%: 60 tries miss them all
// about once in 10^10 runs.
const covertTries = 60

// covertTestAgents is what runCovertUntil restocks the attacker to between
// attempts. Every effect op now spends an agent when it is QUEUED and hands it
// back only if it lands, so a loop of sixty tries would otherwise run the
// attacker dry and start returning ErrNoAgents.
const covertTestAgents = 10_000

// runCovert runs one operation the way a day of play does, and returns what the
// attacker is told. An EFFECT op only queues at the menu, so its report is the
// line the drain files on the attacker's recap; the two INFO ops resolve at the
// menu, so theirs is the value returned there. Which of the two happened is read
// off the queue rather than from a list of op names, so an op that changes side
// is followed rather than silently mis-read.
func runCovert(t *testing.T, w *World, a *Empire, run func() (string, error)) string {
	t.Helper()
	report, err := run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.CovertQueue) == 0 {
		return report
	}
	before := len(a.Events)
	w.resolveCovertQueue()
	if len(a.Events) != before+1 {
		t.Fatalf("a queued operation should file one line on the attacker's recap, got %d", len(a.Events)-before)
	}
	return a.Events[len(a.Events)-1].Text
}

// runCovertUntil repeats run until want accepts its report and returns that
// report, clearing the attacker's per-turn slots and restocking its purse and
// agents between attempts so only the roll varies.
func runCovertUntil(t *testing.T, w *World, a *Empire, run func() (string, error), want func(report string) bool) string {
	t.Helper()
	for i := 0; i < covertTries; i++ {
		a.TurnProgress = TurnProgress{}
		a.Gold = 1_000_000_000
		a.Agents = covertTestAgents
		if report := runCovert(t, w, a, run); want(report) {
			return report
		}
	}
	t.Fatalf("%d attempts never reached the branch under test", covertTries)
	return ""
}

// caught reports whether a covert report is one of the two the attacker gets
// when the agent is taken: the effect ops say the operation failed, the two info
// ops say the spy was caught.
func caught(report string) bool {
	return strings.Contains(report, "failed") || strings.Contains(report, "caught")
}

// TestCovertRollIgnoresTheTargetAndScalesWithDifficulty pins the two things
// BRE's one covert roll does that read as bugs. Both terms of the comparison are
// drawn from the ATTACKER, so neither realm's agents move the odds; and each op
// divides the attacker's own term by its own difficulty figure. The three rates
// are golden literals off BRE's formula — 0.1 + 0.9/(1+k) for k of 1, 2 and 3 —
// not the constants, so a retune has to bring new evidence with it. Several
// seeds, since one seed is one trajectory.
func TestCovertRollIgnoresTheTargetAndScalesWithDifficulty(t *testing.T) {
	const trials = 4000
	rate := func(seed int64, op CovertOp, attackerAgents, targetAgents int) float64 {
		w := NewWorldSeed(DefaultConfig(), seed)
		a := w.AddHuman("alice", "Alethia")
		d := w.AddHuman("bob", "Bobland")
		a.Agents, d.Agents = attackerAgents, targetAgents
		won := 0
		for i := 0; i < trials; i++ {
			if w.covertRoll(a, d, op) {
				won++
			}
		}
		return float64(won) / trials
	}
	cases := []struct {
		op   CovertOp
		want float64
	}{
		{OpSendSpy, 0.550},          // divisor 1
		{OpDemoralizeForces, 0.400}, // divisor 2
		{OpBribery, 0.325},          // divisor 3
	}
	for _, seed := range []int64{1, 7, 99, 2026} {
		for _, tc := range cases {
			for _, stock := range []struct {
				name             string
				attacker, target int
			}{
				{"a million against one", 1_000_000, 1},
				{"a million against a million", 1_000_000, 1_000_000},
			} {
				got := rate(seed, tc.op, stock.attacker, stock.target)
				if got < tc.want-0.05 || got > tc.want+0.05 {
					t.Errorf("seed %d, %s, %s: success rate %.3f, want %.3f",
						seed, tc.op, stock.name, got, tc.want)
				}
			}
		}
	}
}

// Covert ops charge their BRE gold cost per op (on top of the agent risk), and
// an op the attacker can't afford does nothing.
func TestCovertOpChargesGold(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0
	a.Gold = 10_000
	if _, err := w.SendSpy(a, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Gold != 10_000-CostSendSpy {
		t.Errorf("Send Spy should charge %d gold: Gold = %d, want %d", CostSendSpy, a.Gold, 10_000-CostSendSpy)
	}
}

// Spy on Relations charges the price its menu shows, which BRE does not: the
// original's report_spy_result serves both info ops and subtracts the slot-'1'
// fee — Send Spy's 5,000 — whichever one called it (BRE.OVR 0x016E73), while its
// menu advertises 100,000. IB charges the advertised figure DELIBERATELY; this
// test locks that divergence so it is not "corrected" to 5,000 by a later
// reading of the binary. It passes against the code as it stands, by design.
func TestSpyOnRelationsChargesTheFeeItAdvertises(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 50, 0

	a.Gold = 99_999 // one gold under the advertised fee
	if _, err := w.SpyOnRelations(a, d); !errors.Is(err, ErrCantAfford) {
		t.Fatalf("expected ErrCantAfford below the advertised 100,000, got %v", err)
	}
	if a.Gold != 99_999 {
		t.Errorf("a refused op must charge nothing: gold %d, want 99999", a.Gold)
	}

	a.Gold = 100_000
	if _, err := w.SpyOnRelations(a, d); err != nil {
		t.Fatal(err)
	}
	if a.Gold != 0 {
		t.Errorf("Spy on Relations should take the 100,000 it advertises: gold %d, want 0", a.Gold)
	}
}

func TestCovertOpTooPoor(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	a.Gold = CostBribery - 1 // can't cover Bribery's fee
	agentsBefore := a.Agents
	if _, err := w.Bribery(a, d); !errors.Is(err, ErrCantAfford) {
		t.Fatalf("expected ErrCantAfford when too poor, got %v", err)
	}
	if a.Gold != CostBribery-1 || a.Agents != agentsBefore {
		t.Error("a failed (unaffordable) op must charge no gold and lose no agent")
	}
}

// BRE's "Limit one try per turn!" is keyed PER OPERATION (the per-digit byte at
// record +0xFD), so a turn holds one try of each effect op rather than one op
// overall. The info ops (Send Spy, Spy on Relations) are exempt entirely. The
// slots clear next turn.
func TestCovertEffectOpCappedOncePerTurnPerOperation(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 50, 0
	a.Gold = 100_000_000

	if _, err := w.StirRevolts(a, d); err != nil {
		t.Fatalf("first try of an effect op should work: %v", err)
	}
	if _, err := w.StirRevolts(a, d); !errors.Is(err, ErrCovertCapReached) {
		t.Fatalf("a SECOND Stir Revolts should be capped, got %v", err)
	}
	// A different operation holds its own slot and is unaffected by the one above.
	if _, err := w.SetUp(a, d); err != nil {
		t.Errorf("a different effect op holds its own slot: %v", err)
	}
	if _, err := w.DemoralizeForces(a, d); err != nil {
		t.Errorf("a different effect op holds its own slot: %v", err)
	}
	if _, err := w.SendSpy(a, d); err != nil {
		t.Errorf("Send Spy is an info op, exempt from the cap: %v", err)
	}
	if _, err := w.SendSpy(a, d); err != nil {
		t.Errorf("an info op is exempt however often it is run: %v", err)
	}
	if _, err := w.SpyOnRelations(a, d); err != nil {
		t.Errorf("Spy on Relations is an info op, exempt from the cap: %v", err)
	}
	a.TurnProgress = TurnProgress{} // next turn
	if _, err := w.StirRevolts(a, d); err != nil {
		t.Errorf("the slots should reset next turn: %v", err)
	}
}

func TestSendSpyNoAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0

	_, err := w.SendSpy(a, d)
	if !errors.Is(err, ErrNoAgents) {
		t.Fatalf("expected ErrNoAgents, got %v", err)
	}
}

func TestSupportDissensionsSuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0
	d.Troopers = 1000
	beforeEvents := 0

	// A foiled attempt files a victim event of its own, so the count is taken
	// afresh for the attempt that lands.
	report := runCovertUntil(t, w, a, func() (string, error) {
		beforeEvents = len(d.Events)
		return w.SupportDissensions(a, d)
	}, func(r string) bool { return !caught(r) })
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	// Golden band from the LOCAL resolver (BRE.OVR 0x04C178): the share that
	// flees is Random(10)+10-Random(10) percent, so 1-19% of 1,000 goes and
	// 810-990 remain. The flat tenth asserted here before was IB's own.
	if d.Troopers < 810 || d.Troopers > 990 {
		t.Errorf("expected 810-990 troopers remaining, got %d", d.Troopers)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
}

func TestSupportDissensionsNoAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0

	_, err := w.SupportDissensions(a, d)
	if !errors.Is(err, ErrNoAgents) {
		t.Fatalf("expected ErrNoAgents, got %v", err)
	}
}

// A caught spy costs the agent and alerts the victim. Stacking agents cannot
// force the branch any more, so the test spies until one is caught.
func TestSendSpyFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 1000000

	for i := 0; i < 50; i++ {
		before, beforeEvents := a.Agents, len(d.Events)
		report, err := w.SendSpy(a, d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(report, "caught") {
			continue // got through; try again
		}
		if a.Agents != before-1 {
			t.Errorf("caught spy: attacker has %d agents, want %d", a.Agents, before-1)
		}
		if len(d.Events) != beforeEvents+1 {
			t.Fatalf("caught spy raised %d victim events, want 1", len(d.Events)-beforeEvents)
		}
		return
	}
	t.Fatal("50 spies in a row all got through; the roll is not landing near 55%")
}

// A caught agent is lost and the victim is alerted. No fixture can force the
// branch — the roll ignores both realms' agents — so the op runs until one is
// foiled, and the assertions are made against that attempt.
func TestSupportDissensionsFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 1000000
	d.Troopers = 1000

	agentsBefore, eventsBefore := 0, 0
	runCovertUntil(t, w, a, func() (string, error) {
		// An attempt that LANDS takes troopers, so the victim is restocked
		// before each one and the assertions below read the foiled attempt.
		d.Troopers = 1000
		agentsBefore, eventsBefore = a.Agents, len(d.Events)
		return w.SupportDissensions(a, d)
	}, caught)

	if d.Troopers != 1000 {
		t.Errorf("expected troopers unchanged at 1000, got %d", d.Troopers)
	}
	if a.Agents != agentsBefore-1 {
		t.Errorf("expected attacker to lose the caught agent, got %d agents, want %d", a.Agents, agentsBefore-1)
	}
	if len(d.Events) != eventsBefore+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-eventsBefore)
	}
}

// bombFoodEffect is now reached only from the interplanetary Special Operation;
// BRE's LOCAL Bomb Enemy Targets is one op with its own six-slot table.
func TestBombFoodDestroysReserve(t *testing.T) {
	_, _, d := newAttackerAndTarget(t)
	d.Food = 1000
	if lost := bombFoodEffect(d); lost != 500 || d.Food != 500 {
		t.Errorf("expected 500 lost and 500 left after a 50%% strike, got lost=%d left=%d", lost, d.Food)
	}
}

func TestStirRevoltsLowersSupport(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Support = 50, 0, 100
	runCovertUntil(t, w, a, func() (string, error) { return w.StirRevolts(a, d) },
		func(r string) bool { return !caught(r) })
	// Golden band from the LOCAL resolver (BRE.OVR 0x04C00C): support loses
	// Random(4)+5 POINTS, so 92-95 of 100. The x11/13 scaling asserted here
	// before came from the inter-BBS packet resolver, a different op table.
	if d.Support < 92 || d.Support > 95 {
		t.Errorf("expected support 92-95, got %d", d.Support)
	}
}

func TestCovertOpNeedsAnAgent(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0
	if _, err := w.DemoralizeForces(a, d); !errors.Is(err, ErrNoAgents) {
		t.Errorf("expected ErrNoAgents, got %v", err)
	}
}

func TestDemoralizeForcesLowersMorale(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Morale = 50, 0, 100
	runCovertUntil(t, w, a, func() (string, error) { return w.DemoralizeForces(a, d) },
		func(r string) bool { return !caught(r) })
	// Golden band from the LOCAL resolver (BRE.OVR 0x04C2BD): morale loses
	// Random(5)+5 POINTS, so 91-95 of 100. The x6/7 scaling asserted here before
	// came from the inter-BBS packet resolver, a different op table.
	if d.Morale < 91 || d.Morale > 95 {
		t.Errorf("expected morale 91-95, got %d", d.Morale)
	}
}

// Both stats stop at CovertStatFloor however often they are ground at
// (BRE.OVR 0x04C02F for support, 0x04C2E0 for morale). Golden literal 5.
func TestCovertOpsFloorMoraleAndSupport(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 50, 0
	d.Morale, d.Support = 6, 6
	// Twice the passes the deterministic version needed: only about half of them
	// land now, and both stats have to be ground all the way down.
	for i := 0; i < 60; i++ {
		// A foiled attempt costs an agent, and about half of these are foiled.
		a.Gold, a.Agents = 10_000_000, 50
		a.TurnProgress = TurnProgress{}
		if _, err := w.DemoralizeForces(a, d); err != nil {
			t.Fatal(err)
		}
		a.TurnProgress = TurnProgress{}
		if _, err := w.StirRevolts(a, d); err != nil {
			t.Fatal(err)
		}
		w.resolveCovertQueue() // both agents land at the day's maintenance
	}
	if d.Morale != 5 || d.Support != 5 {
		t.Errorf("expected both stats held at 5, got morale %d support %d", d.Morale, d.Support)
	}
}

// BRE's local Bomb Enemy Targets picks ONE of six holdings at random and takes a
// slice of it. The six bands are golden literals from the resolver's own dispatch
// (BRE.OVR 0x04C39E, 0x04C427, 0x04C4B0, 0x04C539, 0x04C5C2, 0x04C64B), and over
// many strikes every one of the six has to be reached — a table wired to the
// wrong field would leave a holding untouched forever.
func TestBombEnemyTargetsHitsEverySlotInBand(t *testing.T) {
	bands := map[string][2]int{
		"People": {5, 14}, "Troopers": {5, 9}, "Agents": {5, 9},
		"Tanks": {5, 7}, "Jets": {5, 7}, "Food": {20, 89},
	}
	seen := map[string]bool{}
	for seed := int64(1); seed <= 3; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		a := w.AddHuman("a", "Alpha")
		d := w.AddHuman("d", "Delta")
		pastProtection(w)
		for i := 0; i < 200; i++ {
			a.Gold, a.Agents = 10_000_000, 10_000_000
			a.TurnProgress = TurnProgress{}
			d.Agents, d.People, d.Troopers = 100_000, 100_000, 100_000
			d.Tanks, d.Jets, d.Food = 100_000, 100_000, 100_000
			before := map[string]int{
				"People": d.People, "Troopers": d.Troopers, "Agents": d.Agents,
				"Tanks": d.Tanks, "Jets": d.Jets, "Food": d.Food,
			}
			if _, err := w.BombEnemyTargets(a, d); err != nil {
				t.Fatal(err)
			}
			w.resolveCovertQueue() // the agent only bombs at maintenance
			after := map[string]int{
				"People": d.People, "Troopers": d.Troopers, "Agents": d.Agents,
				"Tanks": d.Tanks, "Jets": d.Jets, "Food": d.Food,
			}
			hits := 0
			for name, was := range before {
				if after[name] == was {
					continue
				}
				hits++
				seen[name] = true
				lost, band := was-after[name], bands[name]
				if pct := lost * 100 / was; pct < band[0] || pct > band[1] {
					t.Fatalf("seed %d: %s lost %d%% of %d, want %d-%d%%", seed, name, pct, was, band[0], band[1])
				}
			}
			if hits > 1 {
				t.Fatalf("seed %d: one strike damaged %d holdings, want at most 1", seed, hits)
			}
		}
	}
	for name := range bands {
		if !seen[name] {
			t.Errorf("600 strikes never picked %s — is the six-slot table wired to it?", name)
		}
	}
}

func TestSetUpVoidsAlliance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 1_000_000
	d := w.AddHuman("d", "Delta")
	d.Agents = 0
	partner := w.AddHuman("p", "PartnerLand")
	w.ProposeTreaty(d, partner, "Full Defense Alliance")
	w.AcceptTreaty(partner, d.Name, "Full Defense Alliance")

	// Set Up rolls once against each court, so it lands about 30% of the time.
	report := runCovertUntil(t, w, a, func() (string, error) { return w.SetUp(a, d) },
		func(r string) bool { return !caught(r) })
	if !strings.Contains(report, partner.Name) {
		t.Errorf("expected report to name the tricked ally %q, got %q", partner.Name, report)
	}
	if w.HasTreaty(d, partner, "Full Defense Alliance") {
		t.Error("expected the Full Defense Alliance to be voided")
	}
}

// Expose Enemy Ops shields against ONE realm — the one your bribed agent sits
// inside — and blocks nine of its attempts in ten rather than all of them
// (BRE.OVR 0x01701B writes the expiry per pair; 0x04BA48 reads it and lets one
// attempt in ten through). Both golden literals here are BRE's, not IB's: the
// exposed realm lands about 1 op in 20 (a tenth of the 55% an easy op would
// otherwise make), and an unexposed one is untouched at 55%.
func TestExposeEnemyOpsShieldsAgainstOneRealmOnly(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold, a.Agents = 100_000_000, 1_000_000
	exposed := w.AddHuman("d", "Delta")
	other := w.AddHuman("o", "Omega")
	exposed.Agents, other.Agents = 1_000_000, 1_000_000

	// The shield needs a bribed agent already inside the realm it shields
	// against; without one there is nothing to aim it at.
	if _, err := w.ExposeEnemyOps(a, exposed); !errors.Is(err, ErrNoBribedAgent) {
		t.Fatalf("expected ErrNoBribedAgent with no agent inside %s, got %v", exposed.Name, err)
	}
	a.Bribed = append(a.Bribed, exposed.Name)

	goldBefore, agentsBefore := a.Gold, a.Agents
	if _, err := w.ExposeEnemyOps(a, exposed); err != nil {
		t.Fatal(err)
	}
	// It spends no agent and takes no per-turn slot: BRE's menu dispatches it
	// before it reaches either.
	if a.Agents != agentsBefore {
		t.Errorf("Expose Enemy Ops should spend no agent, %d -> %d", agentsBefore, a.Agents)
	}
	if a.Gold != goldBefore-CostExposeEnemyOps {
		t.Errorf("gold %d, want %d", a.Gold, goldBefore-CostExposeEnemyOps)
	}
	if len(a.TurnProgress.CovertOpsUsed) != 0 {
		t.Errorf("Expose Enemy Ops should take no per-turn slot, got %v", a.TurnProgress.CovertOpsUsed)
	}

	const trials = 4000
	rate := func(from *Empire) float64 {
		won := 0
		for i := 0; i < trials; i++ {
			if w.covertRoll(from, a, OpSupportDissensions) {
				won++
			}
		}
		return float64(won) / trials
	}
	if got := rate(exposed); got < 0.02 || got > 0.09 {
		t.Errorf("the exposed realm landed %.3f of its ops, want near 0.055", got)
	}
	if got := rate(other); got < 0.50 || got > 0.60 {
		t.Errorf("an unexposed realm landed %.3f of its ops, want near 0.55 — the shield is not per realm", got)
	}
}

// bombRoutesFixture builds an attacker, a victim, the victim's trade partner and
// a realm the victim holds Protective Trade with, on the given seed. The
// attacker's covert roll is made certain (all the agents against none) so what
// varies is the strike's own dice, not whether the agent got in.
func bombRoutesFixture(t *testing.T, seed int64) (w *World, a, d, partner, guarded *Empire) {
	t.Helper()
	w = NewWorldSeed(DefaultConfig(), seed)
	a = w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000
	a.Agents = 1_000_000
	d = w.AddHuman("d", "Delta")
	d.Agents = 0
	partner = w.AddHuman("p", "PartnerLand")
	partner.Agents = 0
	guarded = w.AddHuman("g", "GuardedLand")
	guarded.Agents = 0
	pastProtection(w)
	return w, a, d, partner, guarded
}

// pact puts ttype between two realms through the offer/accept path.
func pact(t *testing.T, w *World, from, to *Empire, ttype string) {
	t.Helper()
	w.ProposeTreaty(from, to, ttype)
	if !w.AcceptTreaty(to, from.Name, ttype) {
		t.Fatalf("AcceptTreaty %s between %s and %s failed", ttype, from.Name, to.Name)
	}
}

// bombDealQty is the round quantity every good in a test deal carries, so the
// 5-9% a bombed deal keeps is the golden 50-90.
const bombDealQty = 1000

// bombRoutesTrials is how many strikes a test throws at a standing deal. Each
// has a 1-in-9 chance of reaching it (a 1-in-3 landing roll and a 1-in-3 per-deal
// roll), so 200 leaves a miss at about 4e-11 — the tests below assert what
// happens when a strike lands, not that any single one does.
const bombRoutesTrials = 200

// pendingDeal records a deal from `from` on `to` directly. SendTradeDeal's pact,
// carrier and escrow requirements would put a second relation between the
// parties, which is the one thing these tests are measuring.
func pendingDeal(from, to *Empire) {
	to.TradeDeals = append(to.TradeDeals, TradeDeal{
		From:   from.Name,
		Send:   TradeBasket{Troopers: bombDealQty, Food: bombDealQty, Gold: bombDealQty},
		Demand: TradeBasket{Tanks: bombDealQty},
	})
}

// dealUntouched reports whether a deal pendingDeal recorded still carries
// everything it was given.
func dealUntouched(deal TradeDeal) bool {
	return deal.Send == (TradeBasket{Troopers: bombDealQty, Food: bombDealQty, Gold: bombDealQty}) &&
		deal.Demand == (TradeBasket{Tanks: bombDealQty})
}

// bombRoutes runs one Bomb Trade Routes strike against d through the helper the
// interplanetary op calls, skipping the fee and the one-effect-op-per-turn cap so
// a test can repeat it.
func bombRoutes(w *World, d *Empire) {
	if w.bombRoutesLands() {
		w.bombRoutesEffect(d)
	}
}

// The Protective Trade guard belongs to the DEAL being bombed, not to the
// attacker: a deal whose own two parties hold the pact survives a strike from an
// unrelated third realm, while an unguarded deal standing beside it on the same
// realm is wrecked (BRE.OVR 0x051077). Both halves are properties rather than one
// trajectory, so several seeds.
func TestBombTradeRoutesSparesTheDealsOwnProtectivePartners(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		w, _, d, partner, guarded := bombRoutesFixture(t, seed)
		pact(t, w, d, guarded, "Protective Trade")
		pendingDeal(guarded, d) // index 0: its own two parties hold the pact
		pendingDeal(partner, d) // index 1: its parties hold nothing

		wrecked := false
		for i := 0; i < bombRoutesTrials; i++ {
			bombRoutes(w, d)
			if !dealUntouched(d.TradeDeals[0]) {
				t.Fatalf("seed %d: a deal between Protective Trade partners was bombed", seed)
			}
			if !dealUntouched(d.TradeDeals[1]) {
				wrecked = true
			}
		}
		if !wrecked {
			t.Errorf("seed %d: %d strikes never touched the unguarded deal", seed, bombRoutesTrials)
		}
	}
}

// The attacker's own relations are never read. A realm holding Protective Trade
// with the VICTIM still wrecks a deal whose own two parties hold nothing, and the
// strike is neither refused nor refunded. Standing treaties are no longer touched
// at all — the op damages deals in transit, not the agreements behind them.
func TestBombTradeRoutesIgnoresTheAttackersOwnPact(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		w, a, d, partner, _ := bombRoutesFixture(t, seed)
		pact(t, w, d, partner, "Free Trade Agreement")
		pact(t, w, a, d, "Protective Trade")
		pendingDeal(partner, d)

		wrecked := false
		for i := 0; i < bombRoutesTrials; i++ {
			bombRoutes(w, d)
			if !dealUntouched(d.TradeDeals[0]) {
				wrecked = true
			}
		}
		if !wrecked {
			t.Errorf("seed %d: the attacker's own pact shielded a deal it is not a party to", seed)
		}
		if !w.HasTreaty(d, partner, "Free Trade Agreement") {
			t.Errorf("seed %d: the op should damage deals, not sever standing agreements", seed)
		}
	}
}

// A bombed deal keeps 5-9% of every good and loses the rest (BRE.OVR 0x051077,
// trunc(qty x (random(5)+5) / 100)). The band is asserted as the golden 50-90 out
// of 1,000 rather than through the constants, so a retune fails here.
func TestBombTradeRoutesLeavesFiveToNinePercent(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		w, _, d, partner, _ := bombRoutesFixture(t, seed)
		landed := 0
		for i := 0; i < bombRoutesTrials; i++ {
			d.TradeDeals = nil
			pendingDeal(partner, d)
			bombRoutes(w, d)
			deal := d.TradeDeals[0]
			if dealUntouched(deal) {
				continue
			}
			landed++
			for name, got := range map[string]int{
				"Troopers": deal.Send.Troopers,
				"Food":     deal.Send.Food,
				"Gold":     deal.Send.Gold,
				"Tanks":    deal.Demand.Tanks,
			} {
				if got < 50 || got > 90 {
					t.Fatalf("seed %d: %s: expected 50-90 of %d left after a strike, got %d", seed, name, bombDealQty, got)
				}
			}
		}
		if landed == 0 {
			t.Errorf("seed %d: %d strikes landed on nothing", seed, bombRoutesTrials)
		}
	}
}

// Two strikes in three come to nothing before a single deal is looked at
// (BRE.OVR 0x04a09a), on top of the one-in-three each deal has of escaping. With
// one deal standing that is 1 strike in 9 landing, against the 1 in 3 it would be
// with no void roll. A rate is a property of the dice, not of a seed, so it is
// measured over many trials on several seeds.
func TestBombTradeRoutesVoidsTwoStrikesInThree(t *testing.T) {
	const trials = 900
	for seed := int64(1); seed <= 3; seed++ {
		w, _, d, partner, _ := bombRoutesFixture(t, seed)
		landed := 0
		for i := 0; i < trials; i++ {
			d.TradeDeals = nil
			pendingDeal(partner, d)
			bombRoutes(w, d)
			if !dealUntouched(d.TradeDeals[0]) {
				landed++
			}
		}
		// 1 in 9 of 900 is 100, with a standard deviation of 9.4; 1 in 3 is 300.
		if landed < 60 || landed > 145 {
			t.Errorf("seed %d: %d of %d strikes landed, expected about 100 (1 in 9)", seed, landed, trials)
		}
	}
}

// BRE's market bombing reads no relation at all, so Protective Trade between
// attacker and victim does not shield a listing. The 25% loss is asserted as the
// exact computed figure, which stays deterministic across seeds.
func TestBombTradingMarketIsNotGatedByAnyRelation(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		w, a, d, _, _ := bombRoutesFixture(t, seed)
		pact(t, w, a, d, "Protective Trade")
		d.Tanks = 20
		if err := w.SetMarketListing(d, "Tank", 20, 1000); err != nil {
			t.Fatalf("seed %d: list: %v", seed, err)
		}
		w.bombMarketPosition(d, BombMarketLossPct)
		if got := w.MarketForSale(d.Name, "Tank"); got != 15 {
			t.Errorf("seed %d: expected 15 tanks left after a 25%% strike, got %d", seed, got)
		}
	}
}

func TestUndermineInvestmentsReducesPrincipal(t *testing.T) {
	_, _, d := newAttackerAndTarget(t)
	d.Investments = []Investment{{Amount: 1000, Return: 1100, MaturesDay: 5}}
	if lost := undermineEffect(d); lost != 250 {
		t.Errorf("expected 250 principal destroyed, got %d", lost)
	}
	if d.Investments[0].Amount != 750 {
		t.Errorf("expected 750 principal remaining, got %d", d.Investments[0].Amount)
	}
}

func TestSabreStrikeDamagesTarget(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	// sabreEffect is the target-side half and rolls no covert attempt, so
	// no agents are needed here. No Troopers on the target so it can never
	// backfire, and a stock of every strikeable resource. Only ~3 in 10 launches
	// land, so fire many.
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Jets, d.Turrets, d.Tanks, d.Food, d.People = 1000, 1000, 1000, 1000, 1_000_000
	before := d.Jets + d.Turrets + d.Tanks + d.Food + d.People
	for i := 0; i < 100; i++ {
		w.sabreEffect(d, a.Name, w.rng.Intn(SabreDialMax+1))
	}
	after := d.Jets + d.Turrets + d.Tanks + d.Food + d.People
	if after >= before {
		t.Errorf("expected S3-Sabre strikes to reduce the target's resources, before=%d after=%d", before, after)
	}
}

// The dial AIMS the missile — this is the whole of what was wrong before
// 2026-08-31, when IB treated it as a bluff that changed nothing. Dial 5 maps to
// airbases, which hit jets and nothing else, so a realm holding jets and tanks
// in equal number must lose jets and keep its tanks. Asserting the asymmetry
// rather than "something was destroyed" is what makes this fail if the mapping
// is ignored again.
func TestSabreDialAimsTheMissile(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Jets, d.Tanks, d.Food = 100_000, 100_000, 100_000
	for i := 0; i < 400; i++ {
		w.sabreEffect(d, a.Name, 5)
	}
	if d.Jets >= 100_000 {
		t.Errorf("dial 5 is airbases and must cost the target jets; still %d", d.Jets)
	}
	// Tanks and food are reachable only through the ±1 jitter (dial 4, military
	// bases) and the 1-in-10 wild roll, so they may move — but far less than the
	// jets the dial actually aimed at.
	if 100_000-d.Tanks >= 100_000-d.Jets {
		t.Errorf("dial 5 cost more tanks (%d) than jets (%d); the mapping is not being read",
			100_000-d.Tanks, 100_000-d.Jets)
	}
}

// Every dial setting resolves to a defined effect. The mapper's table is the
// binary's, so an out-of-range dial must be clamped rather than indexing past it.
func TestSabreAimCoversEveryDial(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	seen := map[SabreEffect]bool{}
	for dial := SabreDialMin; dial <= SabreDialMax; dial++ {
		for i := 0; i < 200; i++ {
			seen[w.SabreAim(dial)] = true
		}
	}
	// Every row but the last, which no dial can reach (the input is taken mod 11).
	for eff := SabreHitHQ; eff < SabreDevelopRegions; eff++ {
		if !seen[eff] {
			t.Errorf("effect %d is unreachable from any dial", eff)
		}
	}
}

// Gold is NOT one of the S3-Sabre's targets, and neither are agents, bombers or
// carriers. IB used to roll a target across every good it held, which is what
// the dial mapping replaced: the original's effect switch writes back exactly
// the HQ, population, food, jets, and the four military counts, and nothing else.
func TestSabreLeavesGoldAlone(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Gold, d.Bombers, d.Carriers = 1_000_000, 1000, 1000
	for i := 0; i < 300; i++ {
		w.sabreEffect(d, a.Name, w.rng.Intn(SabreDialMax+1))
	}
	if d.Gold != 1_000_000 || d.Bombers != 1000 || d.Carriers != 1000 {
		t.Errorf("the missile reached an asset no effect names: gold %d, bombers %d, carriers %d",
			d.Gold, d.Bombers, d.Carriers)
	}
}

// BINARY-VERIFIED: a caught agent gives the attacker away; an operation that
// succeeds does not (BRE.OVR 0x016d67 for the local spy report, 0x04a96b and
// 0x04a09a for the two interplanetary paths). The whole family has to carry it,
// so this walks every covert op rather than the one that was changed first — a
// sibling left on the old wording would still pass a single-op test.
func TestFoiledCovertOpsNameTheAttacker(t *testing.T) {
	ops := []struct {
		name string
		run  func(*World, *Empire, *Empire) (string, error)
	}{
		{"SendSpy", (*World).SendSpy},
		{"SupportDissensions", (*World).SupportDissensions},
		{"DemoralizeForces", (*World).DemoralizeForces},
		{"SetUp", (*World).SetUp},
		{"SpyOnRelations", (*World).SpyOnRelations},
		{"Bribery", (*World).Bribery},
		{"StirRevolts", (*World).StirRevolts},
		{"BombEnemyTargets", (*World).BombEnemyTargets},
	}
	for _, op := range ops {
		w, a, d := newAttackerAndTarget(t)
		a.Gold, a.Agents = 1_000_000_000, 10_000
		// Exposing the attacker turns nine rolls in ten, so the foiled branch
		// comes up quickly; nothing can force it outright.
		d.ExposedFrom = map[string]int{a.Name: w.GameDay + ExposeOpsShieldDays}
		before := 0

		runCovertUntil(t, w, a, func() (string, error) {
			// A Bribery that lands would refuse the next attempt outright, and
			// would double a's odds from then on.
			a.Bribed = nil
			before = len(d.Events)
			return op.run(w, a, d)
		}, caught)

		if len(d.Events) != before+1 {
			t.Fatalf("%s: filed %d victim events, want 1", op.name, len(d.Events)-before)
		}
		if text := d.Events[before].Text; !strings.Contains(text, a.Name) {
			t.Errorf("%s: foiled event does not name the attacker %q: %q", op.name, a.Name, text)
		}
	}
}

// The other half of the same rule: an operation that lands leaves the target
// guessing. Naming the attacker there would make Expose Enemy Ops and Bribery
// pointless, since the name is the only thing they buy.
func TestSuccessfulCovertOpsStayAnonymous(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold, a.Agents = 1_000_000_000, 1_000
	d.Agents = 0
	before := 0

	runCovertUntil(t, w, a, func() (string, error) {
		d.Troopers, before = 10_000, len(d.Events)
		return w.SupportDissensions(a, d)
	}, func(r string) bool { return !caught(r) })

	if d.Troopers == 10_000 {
		t.Fatal("the op did not land, so this proves nothing about a success")
	}
	if len(d.Events) != before+1 {
		t.Fatalf("filed %d victim events, want 1", len(d.Events)-before)
	}
	if text := d.Events[before].Text; strings.Contains(text, a.Name) {
		t.Errorf("a successful covert op named the attacker: %q", text)
	}
}
