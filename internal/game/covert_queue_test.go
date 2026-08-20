package game

import (
	"strings"
	"testing"
)

// effectOps is every local covert operation BRE's menu QUEUES: the six digits
// its resolver has a case for (2, 3, 4, 5, 7 and 8 at BRE.OVR 0x04BFD4 onward).
// Send Spy and Spy on Relations resolve at the menu, and Expose Enemy Ops is
// dispatched before the queue is reached.
var effectOps = []struct {
	op  CovertOp
	run func(*World, *Empire, *Empire) (string, error)
}{
	{OpStirRevolts, (*World).StirRevolts},
	{OpSetUp, (*World).SetUp},
	{OpSupportDissensions, (*World).SupportDissensions},
	{OpDemoralizeForces, (*World).DemoralizeForces},
	{OpBombEnemyTargets, (*World).BombEnemyTargets},
	{OpBribery, (*World).Bribery},
}

// An effect operation does nothing at the menu but take the fee, the agent and
// the turn slot: the target is untouched and learns nothing until the queue is
// drained. This is the whole of the change, so it is asserted for all six
// operations rather than the one that is easiest to measure.
func TestEffectOpsOnlyQueueAtTheMenu(t *testing.T) {
	for _, tc := range effectOps {
		w, a, d := newAttackerAndTarget(t)
		a.Gold, a.Agents = 1_000_000_000, 500
		d.Agents, d.Morale, d.Support, d.Troopers = 0, 100, 100, 100_000
		d.People, d.Tanks, d.Jets, d.Food = 100_000, 100_000, 100_000, 100_000

		report, err := tc.run(w, a, d)
		if err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		if !strings.Contains(report, d.Name) {
			t.Errorf("%s: the acknowledgement should name the target: %q", tc.op, report)
		}
		if len(w.CovertQueue) != 1 || w.CovertQueue[0].Op != tc.op || w.CovertQueue[0].Target != d.Name {
			t.Fatalf("%s: queue holds %+v", tc.op, w.CovertQueue)
		}
		if a.Agents != 499 {
			t.Errorf("%s: the agent is spent when the record is queued, got %d agents", tc.op, a.Agents)
		}
		if len(d.Events) != 0 {
			t.Errorf("%s: the target learned something before the agent arrived: %v", tc.op, d.Events)
		}
		if d.Morale != 100 || d.Support != 100 || d.Troopers != 100_000 || a.hasBribed(d.Name) {
			t.Errorf("%s: the operation took effect at the menu", tc.op)
		}
		if len(a.Events) != 0 {
			t.Errorf("%s: the attacker was given a result at the menu: %v", tc.op, a.Events)
		}
	}
}

// The other half: draining the queue resolves the operation and files the result
// on the ATTACKER's recap, which is where an asynchronous player reads it.
func TestDrainingTheQueueResolvesAndReports(t *testing.T) {
	for _, tc := range effectOps {
		w, a, d := newAttackerAndTarget(t)
		a.Gold, a.Agents = 1_000_000_000, 500
		d.Agents, d.Morale, d.Support = 0, 100, 100

		if _, err := tc.run(w, a, d); err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		w.resolveCovertQueue()

		if len(w.CovertQueue) != 0 {
			t.Errorf("%s: the queue still holds %d records", tc.op, len(w.CovertQueue))
		}
		if len(a.Events) != 1 {
			t.Fatalf("%s: filed %d lines on the attacker's recap, want 1", tc.op, len(a.Events))
		}
		// The agent comes back only if the operation landed; either way exactly
		// one of the two outcomes is on the recap.
		text := a.Events[0].Text
		if strings.Contains(text, "failed") {
			if a.Agents != 499 {
				t.Errorf("%s: a failed operation keeps the spent agent, got %d", tc.op, a.Agents)
			}
		} else if a.Agents != 500 {
			t.Errorf("%s: a successful operation hands the agent back, got %d", tc.op, a.Agents)
		}
	}
}

// A realm that dies between the agent leaving and the agent arriving cannot be
// struck. BRE re-checks the target's stored ID against the realm in that slot
// and abandons the record; nothing is refunded either way. The attacker is told,
// so 200,000 gold does not vanish without explanation.
func TestQueuedOpAgainstADeadRealmIsAbandoned(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold, a.Agents = 1_000_000_000, 500
	goldAfterFee := int64(0)

	if _, err := w.Bribery(a, d); err != nil {
		t.Fatal(err)
	}
	goldAfterFee = a.Gold
	d.Alive = false

	w.resolveCovertQueue()

	if len(a.Events) != 1 || !strings.Contains(a.Events[0].Text, d.Name) {
		t.Fatalf("the attacker should be told the realm was gone, got %v", a.Events)
	}
	if a.hasBribed(d.Name) {
		t.Error("an abandoned operation must not take effect")
	}
	if a.Gold != goldAfterFee || a.Agents != 499 {
		t.Error("an abandoned operation refunds nothing, as the original refunds nothing")
	}
}

// Daily maintenance is the hook. A queued operation left by yesterday's play
// resolves on the next day's run, which is what makes the recap the place the
// result shows up.
func TestDailyMaintenanceDrainsTheCovertQueue(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	d := w.AddHuman("d", "Delta")
	a.Gold, a.Agents = 1_000_000_000, 500
	d.Agents, d.Support = 0, 100
	pastProtection(w)
	w.LastMaintDate = "2026-07-31"

	if _, err := w.StirRevolts(a, d); err != nil {
		t.Fatal(err)
	}
	if len(w.CovertQueue) != 1 {
		t.Fatalf("precondition: the operation should be queued, got %d records", len(w.CovertQueue))
	}

	w.DailyMaintenance("2026-08-01")

	if len(w.CovertQueue) != 0 {
		t.Errorf("maintenance left %d records queued", len(w.CovertQueue))
	}
	if len(a.Events) == 0 {
		t.Error("maintenance should have filed the operation's result on the attacker's recap")
	}
}

// The two info operations are not queued: BRE's manual singles Send Spy out as
// taking place immediately, and its menu resolves both through report_spy_result
// (BRE.OVR 0x016D67) without ever building a record. Expose Enemy Ops is
// dispatched ahead of the queue too.
func TestInfoOpsAndExposeAreNotQueued(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold, a.Agents = 1_000_000_000, 500
	a.Bribed = []string{d.Name}

	for _, run := range []func() (string, error){
		func() (string, error) { return w.SendSpy(a, d) },
		func() (string, error) { return w.SpyOnRelations(a, d) },
		func() (string, error) { return w.ExposeEnemyOps(a, d) },
	} {
		if _, err := run(); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.CovertQueue) != 0 {
		t.Errorf("an immediate operation was queued: %+v", w.CovertQueue)
	}
}

// A queued record naming an op this build does not handle still owes its sender
// a report: the agent and the fee are gone either way, and an empty string would
// be filed as a blank line on the recap.
func TestUnknownQueuedOpStillReports(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	before := len(a.Events)
	w.CovertQueue = []QueuedCovertOp{{Attacker: a.Name, Target: d.Name, Op: CovertOp("Steal Sheep")}}

	w.resolveCovertQueue()

	if len(a.Events) != before+1 {
		t.Fatalf("events = %d, want one report filed", len(a.Events)-before)
	}
	if got := a.Events[len(a.Events)-1]; got.Text == "" {
		t.Error("the sender was filed a blank recap line")
	}
}
