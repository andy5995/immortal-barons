package game

import (
	"strings"
	"testing"
)

// spyWorlds builds two boards: ours, which pays for a watcher, and theirs,
// which will be watched.
func spyWorlds(t *testing.T) (ours, theirs *World, payer *Empire) {
	t.Helper()
	cfgA := DefaultConfig()
	cfgA.IBBS = true
	cfgA.AICount = 0
	cfgA.BoardID = "Wildside"
	ours = NewWorldSeed(cfgA, 1)
	payer = ours.AddHuman("alice", "Alethia")
	payer.Protection = 0
	payer.Gold = 1_000_000_000

	cfgB := DefaultConfig()
	cfgB.IBBS = true
	cfgB.AICount = 0
	cfgB.BoardID = "Nova Hub"
	theirs = NewWorldSeed(cfgB, 2)
	return ours, theirs, payer
}

// The price is per DAY and drawn from the SENDER's own planet: total regions
// times SpyGuyGoldPerRegion, and the whole stay is charged up front.
// BINARY-VERIFIED against run_bombing_operations_menu.
func TestSpyGuyCostsGoldPerDayByOurOwnSize(t *testing.T) {
	ours, _, payer := spyWorlds(t)
	var regions int
	for _, e := range ours.Empires {
		if e.Alive {
			regions += e.Land
		}
	}
	want := int64(regions) * SpyGuyGoldPerRegion
	if got := ours.SpyGuyCostPerDay(); got != want {
		t.Fatalf("cost per day %d, want %d", got, want)
	}

	before := payer.Gold
	if err := ours.SendSpyGuy(payer, "Nova Hub", 4); err != nil {
		t.Fatalf("SendSpyGuy: %v", err)
	}
	if spent := before - payer.Gold; spent != want*4 {
		t.Errorf("charged %d for four days, want %d", spent, want*4)
	}
	if len(ours.Outbox) != 1 || len(ours.Outbox[0].SpyGuys) != 1 {
		t.Fatalf("no watcher was dispatched: %+v", ours.Outbox)
	}
	// No agent is spent: he is not a covert agent.
	if payer.Agents != ours.Empires[0].Agents {
		t.Error("sending a SpyGuy took an agent")
	}
}

// The stay is bounded by SpyGuyMaxDays and by what the caller can pay for.
func TestSpyGuyStayIsBounded(t *testing.T) {
	ours, _, payer := spyWorlds(t)
	if got := ours.SpyGuyDaysAffordable(payer); got != SpyGuyMaxDays {
		t.Errorf("a rich baron may stay %d days, want the %d-day ceiling", got, SpyGuyMaxDays)
	}
	payer.Gold = ours.SpyGuyCostPerDay() * 2
	if got := ours.SpyGuyDaysAffordable(payer); got != 2 {
		t.Errorf("two days' gold buys %d days", got)
	}
	if err := ours.SendSpyGuy(payer, "Nova Hub", 3); err != ErrCantAfford {
		t.Errorf("a stay longer than the gold allows: %v", err)
	}
	if err := ours.SendSpyGuy(payer, "Nova Hub", SpyGuyMaxDays+1); err != ErrSpyGuyDays {
		t.Errorf("a stay past the ceiling was accepted: %v", err)
	}
}

// The watched board keeps the LONGER of a stay already paid for and a new one,
// as the original's receiver does, and forgets him a day at a time.
func TestSpyGuyStayKeepsTheLongerAndExpires(t *testing.T) {
	_, theirs, _ := spyWorlds(t)
	theirs.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 5})
	theirs.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 2})
	if got := theirs.SpyGuys["Wildside"]; got != 5 {
		t.Fatalf("stay is %d days, want the longer 5", got)
	}
	for i := 0; i < 4; i++ {
		theirs.expireSpyGuys()
	}
	if got := theirs.SpyGuys["Wildside"]; got != 1 {
		t.Fatalf("after four days the stay is %d, want 1", got)
	}
	theirs.expireSpyGuys()
	if theirs.watching("Wildside") {
		t.Error("the watcher outstayed what was paid for")
	}
}

// What he is for: a group attack assembled against his planet is reported home,
// and it arrives as PLANET NEWS there rather than as mail — BRE sends it as a
// NEWS_DATA record.
func TestSpyGuyReportsAGroupAttackAsNewsAtHome(t *testing.T) {
	ours, theirs, _ := spyWorlds(t)
	theirs.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 3})
	raider := theirs.AddHuman("bob", "Rome")
	raider.Protection = 0
	raider.Troopers = 5000

	if _, err := theirs.CreateGroupAttack(raider, "Wildside", "", 24, AttackForce{Troopers: 100}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	var carried []string
	for _, p := range theirs.Outbox {
		if p.ToBoard == "Wildside" {
			carried = append(carried, p.News...)
		}
	}
	if len(carried) != 1 {
		t.Fatalf("the watcher sent %d reports, want 1: %+v", len(carried), theirs.Outbox)
	}
	if !strings.Contains(carried[0], "Nova Hub") || !strings.Contains(carried[0], "hours") {
		t.Errorf("the report should name the planet and the hours, got %q", carried[0])
	}

	// It lands in the paying planet's news, where every baron there reads it.
	ours.ApplyPacket(Packet{FromBoard: "Nova Hub", ToBoard: "Wildside", News: carried})
	if news := strings.Join(ours.NewsToday, "\n"); !strings.Contains(news, "Nova Hub") {
		t.Errorf("the report never reached the planet news: %q", news)
	}
}

// A planet with no watcher here is told nothing at all.
func TestNoWatcherNoReport(t *testing.T) {
	_, theirs, _ := spyWorlds(t)
	raider := theirs.AddHuman("bob", "Rome")
	raider.Protection = 0
	raider.Troopers = 5000

	if _, err := theirs.CreateGroupAttack(raider, "Wildside", "", 24, AttackForce{Troopers: 100}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	for _, p := range theirs.Outbox {
		if len(p.News) != 0 {
			t.Errorf("a strike was reported to a planet with no watcher here: %+v", p.News)
		}
	}
}

// A watcher arriving after the fact is caught up at once: the original answers
// a SPY_GUY packet with the gooie and group-attack state it already holds.
func TestAnArrivingSpyGuyIsToldWhatIsAlreadyAimedAtHim(t *testing.T) {
	_, theirs, _ := spyWorlds(t)
	theirs.Config.ClingyAnnihilator = true
	theirs.RemoteBoards = []RemoteBoard{{BoardID: "Wildside"}}
	builder := theirs.AddHuman("bob", "Rome")
	builder.Protection = 0
	if err := theirs.StartAnnihilator(builder, "Wildside"); err != nil {
		t.Fatalf("StartAnnihilator: %v", err)
	}
	// Nothing goes out while no one is watching.
	for _, p := range theirs.Outbox {
		if len(p.News) != 0 {
			t.Fatalf("reported to a planet with no watcher: %+v", p.News)
		}
	}

	theirs.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 2})
	var carried []string
	for _, p := range theirs.Outbox {
		if p.ToBoard == "Wildside" {
			carried = append(carried, p.News...)
		}
	}
	if len(carried) != 1 || !strings.Contains(carried[0], "Clingy Annihilator") {
		t.Fatalf("the arriving watcher was not caught up: %+v", carried)
	}
}

// And the watched planet is never told it is being watched — no news, no event.
// BRE has no discovery for this man at all: its "found and executed" belongs to
// investigate_traitors and "was found spying on you" to the LOCAL Send Spy.
func TestBeingWatchedIsNeverNoticed(t *testing.T) {
	_, theirs, _ := spyWorlds(t)
	local := theirs.AddHuman("bob", "Rome")
	theirs.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 3})
	theirs.expireSpyGuys()

	if len(theirs.NewsToday) != 0 {
		t.Errorf("the watched planet was told: %q", theirs.NewsToday)
	}
	if len(local.Events) != 0 {
		t.Errorf("a baron on the watched planet was told: %q", local.Events)
	}
}

// A weapon still being built is not announced to its target: knowing early is
// exactly what the watcher is for. Once it flies, the target is told whatever
// happens, since its jets are the only answer to it.
func TestConstructionIsSecretButFlightIsNot(t *testing.T) {
	_, theirs, _ := spyWorlds(t)
	theirs.Config.ClingyAnnihilator = true
	theirs.RemoteBoards = []RemoteBoard{{BoardID: "Wildside"}}
	builder := theirs.AddHuman("bob", "Rome")
	builder.Protection = 0
	builder.Gold = 1_000_000_000_000
	if err := theirs.StartAnnihilator(builder, "Wildside"); err != nil {
		t.Fatalf("StartAnnihilator: %v", err)
	}

	theirs.ExportAnnihilatorStatus()
	for _, p := range theirs.Outbox {
		if p.Annihilator != nil {
			t.Fatalf("a weapon under construction was announced to its target: %+v", p.Annihilator)
		}
	}

	// Fund it and launch it; now the target must be told.
	if _, err := theirs.FundAnnihilator(builder, theirs.Annihilator.CostMillion); err != nil {
		t.Fatalf("FundAnnihilator: %v", err)
	}
	if err := theirs.LaunchAnnihilator(builder); err != nil {
		t.Fatalf("LaunchAnnihilator: %v", err)
	}
	theirs.ExportAnnihilatorStatus()
	var told bool
	for _, p := range theirs.Outbox {
		if p.ToBoard == "Wildside" && p.Annihilator != nil && p.Annihilator.Launched {
			told = true
		}
	}
	if !told {
		t.Error("a weapon in flight was never announced to its target")
	}
}
