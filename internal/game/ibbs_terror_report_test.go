package game

import (
	"strings"
	"testing"
)

// The sender's report must account for every agent it paid for: a batch bigger
// than the target can absorb otherwise reads exactly like a batch that was the
// right size. The caught line and the success line are the original's shape —
// bare for one agent, counted for more (process_terrorist_report).
func TestTerrorOpReportAccountsForEveryAgent(t *testing.T) {
	for _, tc := range []struct {
		name              string
		sent, hit, caught int
		want              string
	}{
		{"batch outruns the target", 25, 7, 1, "One of your agents was caught by Victim's security.\n" +
			"Your agents sabotaged their headquarters 7 times.\n" +
			"17 of your agents got through and found nothing left to damage."},
		{"clean sweep", 4, 4, 0, "Your agents sabotaged their headquarters 4 times."},
		{"all stopped", 3, 0, 3, "3 of your agents were caught by Victim's security."},
		{"one of several lands", 3, 1, 2, "2 of your agents were caught by Victim's security.\n" +
			"One of your agents sabotaged their headquarters."},
		{"lone agent lands", 1, 1, 0, "Your agent sabotaged their headquarters."},
		{"lone agent caught", 1, 0, 1, "Your agent was caught by Victim's security."},
		{"lone agent wasted", 1, 0, 0, "Your agent got through and found nothing left to damage."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := terrorOpReport(TerrorOpSabotageHQ, "Victim", tc.sent, tc.hit, tc.caught)
			if got != tc.want {
				t.Errorf("terrorOpReport(%d, %d, %d):\n got %q\nwant %q", tc.sent, tc.hit, tc.caught, got, tc.want)
			}
		})
	}
}

// A terror op is told to the two realms involved and to nobody else (#285): the
// target on its recap, the sender on its own when the answer comes home, and no
// line in either planet's news. The original's resolver and its returning-report
// routine both file recap entries and neither calls the news writer.
func TestTerrorOpPostsNoNewsOnEitherBoard(t *testing.T) {
	for seed := int64(1); seed <= 6; seed++ {
		cfgA := DefaultConfig()
		cfgA.BoardID = "boardA"
		wA := NewWorldSeed(cfgA, seed)
		sender := wA.AddHuman("att", "Attacker")
		sender.Agents = 50

		cfgB := DefaultConfig()
		cfgB.BoardID = "boardB"
		wB := NewWorldSeed(cfgB, seed)
		target := wB.AddHuman("victim", "Victim")
		target.Protection, target.Morale, target.Agents = 0, 100, 1_000

		if _, err := wA.SendTerror(sender, "boardB", "Victim", 6, TerrorOpDemoralize); err != nil {
			t.Fatalf("SendTerror: %v", err)
		}
		newsA, newsB := len(wA.NewsToday), len(wB.NewsToday)
		targetEvents, senderEvents := len(target.Events), len(sender.Events)

		reply := wB.ApplyPacket(wA.Outbox[0])
		if len(reply.Results) != 1 || reply.Results[0].Kind != "terror" {
			t.Fatalf("seed %d: the op was not resolved: %+v", seed, reply.Results)
		}
		wA.ApplyPacket(reply)

		if got := wB.NewsToday[newsB:]; len(got) != 0 {
			t.Errorf("seed %d: the target's planet was told: %v", seed, got)
		}
		if got := wA.NewsToday[newsA:]; len(got) != 0 {
			t.Errorf("seed %d: the sender's planet was told: %v", seed, got)
		}
		if len(target.Events) == targetEvents {
			t.Errorf("seed %d: the target's recap has no entry", seed)
		}
		if len(sender.Events) != senderEvents+1 {
			t.Fatalf("seed %d: the sender's recap gained %d entries, want 1", seed, len(sender.Events)-senderEvents)
		}
		if got := sender.Events[len(sender.Events)-1].Text; !strings.HasPrefix(got, "Demoralize against Victim of boardB:\n") {
			t.Errorf("seed %d: sender's report = %q", seed, got)
		}
	}
}

// A batch caught to the last agent struck at nothing, so the target is told
// about the agents it caught and not that terrorists "achieved nothing" — a
// line that says they reached the realm.
func TestTerrorCaughtBatchIsNotAlsoTheAchievedNothingLine(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	for seed := int64(1); seed <= 6; seed++ {
		w := NewWorldSeed(cfg, seed)
		target := w.AddHuman("victim", "Victim")
		target.Protection, target.Agents, target.Morale = 0, 1_000_000, 100
		res := w.resolveRemoteTerror(RemoteTerror{
			ID: 1, FromBoard: "boardA", FromEmpire: "Selby", TargetEmpire: "Victim",
			Agents: 3, Op: TerrorOpDemoralize, Strength: 1,
		})
		if !strings.HasPrefix(res.Report, "3 of your agents were caught") {
			// The one-in-a-hundred automatic success let one through; this seed
			// does not exercise the all-caught path.
			continue
		}
		var text []string
		for _, ev := range target.Events {
			text = append(text, ev.Text)
		}
		all := strings.Join(text, "\n")
		if all != "Your security caught 3 agents from Selby of boardA." {
			t.Errorf("seed %d: target recap = %q", seed, all)
		}
		return
	}
	t.Fatal("no seed produced an all-caught batch; the test proves nothing")
}

// A realm under New Realm Protection is told nothing about a terror op aimed at
// it: the original jumps from the protection test straight to the sender's
// result (BRE.OVR 0x04a96b +0x37c). IB named the sending board until 2026-10-01.
func TestProtectedTerrorTargetIsToldNothing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	w := NewWorldSeed(cfg, 1)
	target := w.AddHuman("victim", "Victim")
	target.Protection = 3
	before := len(target.Events)
	res := w.resolveRemoteTerror(RemoteTerror{
		ID: 1, FromBoard: "boardA", FromEmpire: "Selby", TargetEmpire: "Victim",
		Agents: 4, Op: TerrorOpDemoralize, Strength: 1_000_000,
	})
	if res.Outcome != OutcomeProtected {
		t.Fatalf("outcome = %v, want protected", res.Outcome)
	}
	if got := target.Events[before:]; len(got) != 0 {
		t.Errorf("the protected target was told: %v", got)
	}
}

// The first spy that gets in ends the mission, and the realm it got into is
// told nothing — not the spy, and not the agents caught before it. The
// original writes the spy report and returns (BRE.OVR 0x04a96b +0x560),
// before the caught count and the per-operation line are filed. Seed
// independent: every seed that lands a spy must show it.
func TestASpyThatGetsInEndsTheBatchUnseen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	landed := 0
	for seed := int64(1); seed <= 8; seed++ {
		w := NewWorldSeed(cfg, seed)
		target := w.AddHuman("victim", "Victim")
		target.Protection, target.Agents = 0, 0
		before := len(target.Events)
		res := w.resolveRemoteTerror(RemoteTerror{
			ID: 1, FromBoard: "boardA", FromEmpire: "Selby", TargetEmpire: "Victim",
			Agents: 5, Op: TerrorOpSpy, Strength: 1_000_000,
		})
		if !res.Won {
			continue
		}
		landed++
		if got := target.Events[before:]; len(got) != 0 {
			t.Errorf("seed %d: the target of a spy that got in was told: %v", seed, got)
		}
		if strings.Contains(res.Report, "nothing left to damage") || strings.Contains(res.Report, "times") {
			t.Errorf("seed %d: the report counts agents that never went in: %q", seed, res.Report)
		}
		if !strings.Contains(res.Report, "One of your agents went through their files.") {
			t.Errorf("seed %d: report = %q, want one spy in", seed, res.Report)
		}
	}
	if landed == 0 {
		t.Fatal("no seed landed a spy; the test proves nothing")
	}
}
