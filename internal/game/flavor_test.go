package game

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// misfireEntryFor is the index of the missile-pool entry whose firer half reads
// report, filled for missile, target and board.
func misfireEntryFor(report, missile, target, board string) (int, bool) {
	for i, m := range missileMisfirePool {
		if report == fill(m.Yours, "missile", missile, "target", target, "board", board) {
			return i, true
		}
	}
	return 0, false
}

// agentEntryFor is the index of the agent-pool entry whose sender half is in
// report: its Yours line locally, or one of its counted sentences on a
// Terrorist Ops report.
func agentEntryFor(report string) (int, bool) {
	for i, a := range agentCaughtPool {
		for _, line := range []string{a.Yours, a.Caught.one, a.Caught.many, a.Other.one, a.Other.many} {
			if strings.Contains(report, strings.TrimPrefix(line, "{n}")) {
				return i, true
			}
		}
	}
	return 0, false
}

// A misfire picks ONE entry of the missile pool and both sides read it: the
// target's event and the report the firer gets home. Which entry a seed picks
// is not assumed; across seeds more than one must turn up.
func TestMissileMisfirePairsBothHalves(t *testing.T) {
	picked := map[int]bool{}
	for seed := int64(1); seed <= 400; seed++ {
		w, d := specialNewsBoard(seed)
		d.SDI, d.Turrets = 0, 0
		res := w.resolveRemoteSpecialOp(RemoteSpecialOp{
			ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: "Victim", Op: OpNuclear,
		})
		if res.Outcome != OutcomeMisfire {
			continue
		}
		i, ok := misfireEntryFor(text(res.Report), "nuclear missile", "Victim", "Far")
		if !ok {
			t.Fatalf("seed %d: misfire report %q is not from the pool", seed, text(res.Report))
		}
		want := fill(missileMisfirePool[i].Theirs, "missile", "nuclear missile", "from", "Selby of Home")
		if got := d.Events[len(d.Events)-1].Text; got != want {
			t.Errorf("seed %d: target read %q, want the paired half %q", seed, got, want)
		}
		picked[i] = true
	}
	if len(picked) < 2 {
		t.Errorf("400 seeds picked %d distinct misfire lines, want the pool used", len(picked))
	}
}

// A driven-off bombing run's target-planet news is one of the bomber pool's
// lines, and the report that rides home is empty: a bombing run files no
// firer event. Which line a seed picks is not assumed.
func TestBomberDrivenOffNewsComesFromThePool(t *testing.T) {
	picked := map[string]bool{}
	for seed := int64(1); seed <= 200; seed++ {
		w, _ := specialNewsBoard(seed)
		res, news := resolveOneSpecial(t, w, RemoteSpecialOp{
			ID: 1, FromBoard: "Home", FromEmpire: "Selby", Op: OpBombFood,
		})
		if res.Outcome != OutcomeDrivenOff {
			continue
		}
		if !drivenOffNews(news, "Selby of Home", "food market") {
			t.Fatalf("seed %d: driven-off news %q is not from the pool", seed, news)
		}
		if res.Report != nil {
			t.Errorf("seed %d: a driven-off run reported %q, want nothing", seed, text(res.Report))
		}
		picked[news] = true
	}
	if len(picked) < 2 {
		t.Errorf("200 seeds picked %d distinct driven-off lines, want the pool used", len(picked))
	}
}

// A caught agent picks ONE entry of the agent pool, on both paths that catch
// one: an interplanetary Terrorist Ops send and a local covert op.
func TestAgentCaughtPairsBothHalves(t *testing.T) {
	picked := map[int]bool{}
	for seed := int64(1); seed <= 200; seed++ {
		w, d := specialNewsBoard(seed)
		d.Agents = 5_000
		res := w.resolveRemoteTerror(RemoteTerror{
			ID: 1, FromBoard: "Home", FromEmpire: "Selby", TargetEmpire: "Victim",
			Agents: 6, Op: TerrorOpBombAirBases, Strength: 3_000,
		})
		caught := 0
		for _, e := range d.Events {
			if strings.Contains(e.Text, "sent by Selby of Home") {
				caught++
				n := caughtInReport(text(res.Report), 6)
				// A batch caught to the last agent gets the plain all-caught
				// sentence, with no pool clause to pair; the target's line is
				// still one of the pool's, with the same count.
				if strings.HasPrefix(text(res.Report), "Victim's security on Far caught") {
					found := false
					for i := range agentCaughtPool {
						found = found || e.Text == caughtEvent(i, n, "Selby of Home")
					}
					if !found {
						t.Errorf("seed %d: target read %q, not a pool line for %d agents", seed, e.Text, n)
					}
					continue
				}
				i, ok := agentEntryFor(text(res.Report))
				if !ok {
					t.Fatalf("seed %d: report %q carries no caught-agent line", seed, text(res.Report))
				}
				picked[i] = true
				if want := caughtEvent(i, n, "Selby of Home"); e.Text != want {
					t.Errorf("seed %d: target read %q, want the half paired with report %q: %q", seed, e.Text, text(res.Report), want)
				}
			}
		}
		if caught > 1 {
			t.Errorf("seed %d: %d caught lines for one send", seed, caught)
		}
	}

	for seed := int64(1); seed <= 200; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		a := w.AddHuman("a", "Attacker")
		d := w.AddHuman("d", "Defendia")
		a.Agents, d.Agents = 1, 1_000_000
		got := w.resolveStirRevolts(a, d)
		if !strings.HasPrefix(got.English(), "Your agent ") {
			continue
		}
		i, ok := agentEntryFor(got.English())
		if !ok {
			t.Fatalf("seed %d: local report %q is not from the pool", seed, got)
		}
		picked[i] = true
		want := fill(agentCaughtPool[i].Theirs, "who", "an agent behind an agitation attempt, in Attacker's pay")
		if ev := d.Events[len(d.Events)-1].Text; ev != want {
			t.Errorf("seed %d: target read %q, want the paired half %q", seed, ev, want)
		}
	}
	if len(picked) < 2 {
		t.Errorf("picked %d distinct caught-agent lines, want the pool used", len(picked))
	}
}

// drivenOffNews reports whether news is one of the bomber pool's lines, for a
// run from from sent at object.
func drivenOffNews(news, from, object string) bool {
	for _, line := range bomberDrivenOffPool {
		if news == fill(line, "from", from, "target", object) {
			return true
		}
	}
	return false
}

// caughtEvent is the target's line from agent-pool entry i for n agents sent
// by from.
func caughtEvent(i, n int, from string) string {
	who := fmt.Sprintf("%d agents sent by %s", n, from)
	if n == 1 {
		who = "1 agent sent by " + from
	}
	return fill(agentCaughtPool[i].Theirs, "who", who)
}

// caughtInReport is the number of agents a Terrorist Ops report says were
// caught, out of sent.
func caughtInReport(report string, sent int) int {
	switch {
	case strings.Contains(report, "caught every one of your"):
		return sent
	case strings.Contains(report, "caught your agent"), strings.Contains(report, "One of them "),
		strings.Contains(report, "One of your other agents "):
		return 1
	}
	if m := regexp.MustCompile(`(\d+) of (them|your other agents) `).FindStringSubmatch(report); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}
