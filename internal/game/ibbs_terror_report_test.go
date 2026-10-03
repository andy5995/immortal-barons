package game

import (
	"strings"
	"testing"
)

// The sender's report must account for every agent it paid for: a batch bigger
// than the target can absorb otherwise reads exactly like a batch that was the
// right size. It names the target and its board, since the sender prints it
// with no heading, and each part is a sentence of its own so each translates
// on its own (#297). The caught-agent sentence is the pool entry handed in,
// fixed here so the wording is exact.
func TestTerrorOpReportAccountsForEveryAgent(t *testing.T) {
	fate := agentCaughtPool[0]
	for _, tc := range []struct {
		name              string
		sent, hit, caught int
		want              string
	}{
		{"batch outruns the target", 25, 7, 1, "Your agents sabotaged Victim's headquarters on boardB 7 times. " +
			"One of them didn't make it home. 17 more found nothing left to damage."},
		{"clean sweep", 4, 4, 0, "Your agents sabotaged Victim's headquarters on boardB 4 times."},
		{"all stopped", 3, 0, 3, "Victim's security on boardB caught every one of your 3 agents."},
		{"one of several lands", 3, 1, 2, "Your agents sabotaged Victim's headquarters on boardB once. 2 of them didn't make it home."},
		{"none land, some caught", 3, 0, 1, "Your agents reached Victim's headquarters on boardB and found nothing left to damage. " +
			"One of them didn't make it home."},
		{"lone agent lands", 1, 1, 0, "Your agent sabotaged Victim's headquarters on boardB once."},
		{"lone agent caught", 1, 0, 1, "Victim's security on boardB caught your agent."},
		{"lone agent wasted", 1, 0, 0, "Your agent reached Victim's headquarters on boardB and found nothing left to damage."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := terrorOpReport(TerrorOpSabotageHQ, "Victim", "boardB", tc.sent, tc.hit, tc.caught, fate)
			if got.English() != tc.want {
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

		reply := wB.receive(wA.Outbox[0])
		if len(reply.Results) != 1 || reply.Results[0].Kind != "terror" {
			t.Fatalf("seed %d: the op was not resolved: %+v", seed, reply.Results)
		}
		wA.receive(reply)

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
		// One sentence from the target's board, naming the target and the board,
		// printed with no heading.
		if got := sender.Events[len(sender.Events)-1].Text; strings.Contains(got, "\n") ||
			!strings.Contains(got, "Victim") || !strings.Contains(got, "boardB") {
			t.Errorf("seed %d: sender's report = %q", seed, got)
		}
	}
}

// A batch caught to the last agent struck at nothing, so the target is told
// about the agents it caught and not that terrorists "got nowhere" — a
// line that says they reached the realm.
func TestTerrorCaughtBatchIsNotAlsoTheGotNowhereLine(t *testing.T) {
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
		if text(res.Report) != "Victim's security on boardB caught every one of your 3 agents." {
			// The one-in-a-hundred automatic success let one through; this seed
			// does not exercise the all-caught path.
			continue
		}
		var text []string
		for _, ev := range target.Events {
			text = append(text, ev.Text)
		}
		all := strings.Join(text, "\n")
		found := false
		for i := range agentCaughtPool {
			found = found || all == caughtEvent(i, 3, "Selby of boardA")
		}
		if !found {
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
		if strings.Contains(text(res.Report), "nothing left to damage") || strings.Contains(text(res.Report), "times") {
			t.Errorf("seed %d: the report counts agents that never went in: %q", seed, text(res.Report))
		}
		if !strings.HasPrefix(text(res.Report), "Your spy slipped into Victim's files on boardB and came home with a full report") {
			t.Errorf("seed %d: report = %q, want one spy in", seed, text(res.Report))
		}
	}
	if landed == 0 {
		t.Fatal("no seed landed a spy; the test proves nothing")
	}
}
