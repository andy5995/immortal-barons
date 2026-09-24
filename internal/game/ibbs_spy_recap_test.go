package game

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// spyBoards is a sender on boardA and an unguarded target on boardB.
func spyBoards(seed int64, targetAgents int) (wA, wB *World, sender, target *Empire) {
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA = NewWorldSeed(cfgA, seed)
	sender = wA.AddHuman("spy", "Spymaster")
	sender.Agents = 50

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB = NewWorldSeed(cfgB, seed)
	target = wB.AddHuman("victim", "Victim")
	target.Protection, target.Agents = 0, targetAgents
	return
}

// A Send Spy that gets in shows its sender the figures on the recap, and they
// are the spy's own report even when a sweep's report on the same realm, taken
// earlier, rides home in the same packet.
func TestSendSpyRecapCarriesTheFigures(t *testing.T) {
	wA, wB, sender, target := spyBoards(1, 0)
	if err := wA.SendTerror(sender, "boardB", "Victim", 1, TerrorOpSpy); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	reply := wB.ApplyPacket(wA.Outbox[0])
	if len(reply.Results) != 1 || !reply.Results[0].Won {
		t.Fatalf("the spy did not get in: %+v", reply.Results)
	}
	sweep := SpyReport{Board: "boardB", Empire: "Victim", Land: 1, Offense: 1, Defense: 1, Gold: 1}
	reply.ReconReports = append([]SpyReport{sweep}, reply.ReconReports...)
	before := len(sender.Events)

	wA.ApplyPacket(reply)

	if len(sender.Events) != before+1 {
		t.Fatalf("sender's recap gained %d entries, want 1", len(sender.Events)-before)
	}
	got := sender.Events[len(sender.Events)-1].Text
	want := fmt.Sprintf("Land %s  Off %s  Def %s  Gold %s",
		numfmt.Comma(target.Land), numfmt.Comma(target.Offense()),
		numfmt.Comma(target.Defense()), numfmt.Comma(target.Gold))
	if !strings.HasPrefix(got, "Send Spy against Victim of boardB:\n") || !strings.HasSuffix(got, "\n"+want) {
		t.Errorf("sender's report = %q, want it to end with %q", got, want)
	}
}

// A caught spy is still reported to its sender, but it went through no files,
// so the report has no figures.
func TestCaughtSpyRecapHasNoFigures(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		wA, wB, sender, _ := spyBoards(seed, 1_000_000)
		if err := wA.SendTerror(sender, "boardB", "Victim", 1, TerrorOpSpy); err != nil {
			t.Fatalf("SendTerror: %v", err)
		}
		reply := wB.ApplyPacket(wA.Outbox[0])
		if reply.Results[0].Won {
			continue // the automatic success let this one through
		}
		wA.ApplyPacket(reply)
		got := sender.Events[len(sender.Events)-1].Text
		if got != "Send Spy against Victim of boardB:\nYour agent was caught by Victim's security." {
			t.Errorf("seed %d: a caught spy's report = %q, want the caught line and no figures", seed, got)
		}
		return
	}
	t.Fatal("no seed produced a caught spy; the test proves nothing")
}

// The Spy Database keeps the newest reports on each realm up to the cap, leaves
// other realms' alone, and stamps each with its arrival.
func TestSpyDatabaseKeepsTheNewestPerRealm(t *testing.T) {
	now := time.Date(2026, 9, 24, 5, 31, 0, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return now }
	defer func() { timeNow = restore }()

	w := NewWorldSeed(DefaultConfig(), 1)
	w.fileSpyReport(SpyReport{Board: "Far", Empire: "Other", Land: 99})
	for land := 1; land <= 7; land++ {
		w.fileSpyReport(SpyReport{Board: "Far", Empire: "Ruritania", Land: land})
	}

	var lands []int
	others := 0
	for _, e := range w.SpyDatabase {
		if !e.Filed.Equal(now) {
			t.Errorf("entry %+v not stamped with its arrival", e)
		}
		switch e.Empire {
		case "Ruritania":
			lands = append(lands, e.Land)
		case "Other":
			others++
		}
	}
	if fmt.Sprint(lands) != "[3 4 5 6 7]" || others != 1 {
		t.Errorf("kept Ruritania %v and %d of Other, want [3 4 5 6 7] and 1", lands, others)
	}
}

// Only a Send Spy that got in brings intel home, as in the original: a caught
// spy, a protected target and every other operation send back none.
func TestOnlyASpyThatGotInSendsIntel(t *testing.T) {
	intel := func(seed int64, agents int, protected bool, op TerrorOpType) (Packet, bool) {
		wA, wB, sender, target := spyBoards(seed, agents)
		if protected {
			target.Protection = 5
		}
		sender.Agents = 50
		if err := wA.SendTerror(sender, "boardB", "Victim", 1, op); err != nil {
			t.Fatalf("SendTerror: %v", err)
		}
		reply := wB.ApplyPacket(wA.Outbox[0])
		return reply, reply.Results[0].Won
	}
	if reply, won := intel(1, 0, false, TerrorOpSpy); !won || len(reply.ReconReports) != 1 {
		t.Errorf("a spy that got in sent %d intel records (won=%v), want 1", len(reply.ReconReports), won)
	}
	if reply, _ := intel(1, 0, true, TerrorOpSpy); len(reply.ReconReports) != 0 {
		t.Errorf("a protected target sent %d intel records, want 0", len(reply.ReconReports))
	}
	if reply, won := intel(1, 0, false, TerrorOpDemoralize); !won || len(reply.ReconReports) != 0 {
		t.Errorf("a Demoralize that got in sent %d intel records (won=%v), want 0", len(reply.ReconReports), won)
	}
	for seed := int64(1); seed <= 12; seed++ {
		reply, won := intel(seed, 1_000_000, false, TerrorOpSpy)
		if won {
			continue // the automatic success let this one through
		}
		if len(reply.ReconReports) != 0 {
			t.Errorf("seed %d: a caught spy sent %d intel records, want 0", seed, len(reply.ReconReports))
		}
		return
	}
	t.Fatal("no seed produced a caught spy; the test proves nothing")
}
