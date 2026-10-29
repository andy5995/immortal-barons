package game

import (
	"strings"
	"testing"
)

// TestPostInvestRateNewsOnMove posts one economic news line when the rate
// actually moves, and none when it doesn't.
func TestPostInvestRateNewsOnMove(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)

	w.InvestRate = 10
	w.postInvestRateNews(10) // no move
	if len(w.NewsToday) != 0 {
		t.Errorf("expected no news for an unchanged rate, got %v", w.NewsToday)
	}

	w.postInvestRateNews(8) // rose
	if len(w.NewsToday) != 1 {
		t.Fatalf("expected one news line for a rate rise, got %v", w.NewsToday)
	}

	w.NewsToday = nil
	w.InvestRate = 5
	w.postInvestRateNews(9) // fell
	if len(w.NewsToday) != 1 {
		t.Fatalf("expected one news line for a rate fall, got %v", w.NewsToday)
	}
}

// TestPostMasterNewsClaimAndRetain covers both branches: a new leader posts
// a claim line and updates CurrentMaster; an unchanged leader posts a retain
// line and leaves CurrentMaster alone. LastMaster (the crowned league
// champion) is untouched by postMasterNews throughout.
func TestPostMasterNewsClaimAndRetain(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	leader := w.AddHuman("leader", "Leaderland")
	leader.Land = 1000

	w.postMasterNews()
	if w.CurrentMaster != leader.Name {
		t.Fatalf("CurrentMaster = %q, want %q", w.CurrentMaster, leader.Name)
	}
	if len(w.NewsToday) != 1 {
		t.Fatalf("expected one claim line, got %v", w.NewsToday)
	}

	w.NewsToday = nil
	w.postMasterNews() // unchanged leader
	if w.CurrentMaster != leader.Name {
		t.Fatalf("CurrentMaster changed unexpectedly: %q", w.CurrentMaster)
	}
	if len(w.NewsToday) != 1 {
		t.Fatalf("expected one retain line, got %v", w.NewsToday)
	}

	// A new empire overtakes the old leader in net worth.
	w.NewsToday = nil
	challenger := w.AddHuman("challenger", "Challengeria")
	challenger.Land = 1_000_000
	w.postMasterNews()
	if w.CurrentMaster != challenger.Name {
		t.Fatalf("CurrentMaster = %q, want %q", w.CurrentMaster, challenger.Name)
	}
	if len(w.NewsToday) != 1 {
		t.Fatalf("expected one seized-title line, got %v", w.NewsToday)
	}

	if w.LastMaster != "" {
		t.Errorf("expected LastMaster to stay untouched by postMasterNews, got %q", w.LastMaster)
	}
}

// TestTitleIsClaimedFromADepartedMaster: once the Planetary Master abdicates or
// is crushed, the next leader claims the title rather than seizing it from a
// realm that is gone. A living master still has it seized.
func TestTitleIsClaimedFromADepartedMaster(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	old := w.AddHuman("old", "Oldmark")
	old.Land = 1000
	next := w.AddHuman("next", "Nextland")
	next.Land = 500
	w.postMasterNews()

	w.Abdicate(old)
	if got := w.NewsToday[len(w.NewsToday)-1].Text; got != "The baron of Oldmark has abdicated." {
		t.Errorf("abdication news = %q", got)
	}
	// Before the sweep the husk is still there, dead.
	w.NewsToday = nil
	w.postMasterNews()
	if len(w.NewsToday) != 1 || w.NewsToday[0].Text != "Nextland claims the title of Planetary Master!" {
		t.Fatalf("after the master abdicated, news = %v", w.NewsToday)
	}
	// Daily maintenance sweeps the husk before the title check, so there it
	// is gone altogether.
	w.CurrentMaster = old.Name
	w.GameDay++
	w.removeDeadHusks()
	if w.FindByName(old.Name) != nil {
		t.Fatal("the husk was not swept")
	}
	w.NewsToday = nil
	w.postMasterNews()
	if len(w.NewsToday) != 1 || w.NewsToday[0].Text != "Nextland claims the title of Planetary Master!" {
		t.Fatalf("after the husk was swept, news = %v", w.NewsToday)
	}

	w.NewsToday = nil
	rival := w.AddHuman("rival", "Rivalia")
	rival.Land = 2000
	w.postMasterNews()
	if len(w.NewsToday) != 1 || w.NewsToday[0].Text != "Rivalia has seized the title of Planetary Master from Nextland!" {
		t.Fatalf("a living master's title should be seized, news = %v", w.NewsToday)
	}
}

// TestStarvationMakesNoNewsButCivilWarDoes: running out of food posts no
// planet news of its own (the original's news file has no section for it), but
// the civil war it lights does. A well-fed empire posts nothing.
func TestStarvationMakesNoNewsButCivilWarDoes(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)

	starving := w.AddHuman("starving", "Famineburg")
	starving.People = 100
	starving.Food = 0
	starving.Regions = RegionMix{} // no food production
	starving.Alive = true

	w.processEconomy(starving)
	// One line: the civil war the shortfall lights (BRE files a civil war
	// whenever the people got under 65% of their food); none for the famine.
	if len(w.NewsToday) != 1 || !strings.Contains(w.NewsToday[0].Text, "Famineburg") {
		t.Fatalf("expected only the civil-war news line, got %v", w.NewsToday)
	}

	w.NewsToday = nil
	fed := w.AddHuman("fed", "Plentyville")
	fed.People = 100
	fed.Food = 1_000_000
	fed.Alive = true

	w.processEconomy(fed)
	if len(w.NewsToday) != 0 {
		t.Errorf("expected no news for a well-fed empire, got %v", w.NewsToday)
	}
}

// TestMasterIsPaidFromTheQueensPurse holds the daily award to its golden
// figures: 1% of the purse, taken back out of the purse, credited to the
// Master alone and announced on their own recap. An empty purse pays nothing
// and files nothing.
func TestMasterIsPaidFromTheQueensPurse(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	leader := w.AddHuman("leader", "Leaderland")
	leader.Land = 1_000_000
	other := w.AddHuman("other", "Otherland")

	w.RefundPool = 500_000
	leader.Gold, other.Gold = 0, 0
	leader.Events, other.Events = nil, nil
	w.postMasterNews()

	if leader.Gold != 5_000 {
		t.Errorf("Master's gold = %d, want 5000", leader.Gold)
	}
	if w.RefundPool != 495_000 {
		t.Errorf("purse = %d, want 495000", w.RefundPool)
	}
	if other.Gold != 0 || len(other.Events) != 0 {
		t.Errorf("a non-Master was paid: gold %d, events %v", other.Gold, other.Events)
	}
	if len(leader.Events) != 1 {
		t.Fatalf("expected one award notice on the Master's recap, got %v", leader.Events)
	}
	if !strings.Contains(leader.Events[0].Text, "5,000") {
		t.Errorf("award notice does not name the amount: %q", leader.Events[0].Text)
	}

	// An empty purse is silent.
	w.RefundPool = 0
	leader.Gold, leader.Events = 0, nil
	w.postMasterNews()
	if leader.Gold != 0 || len(leader.Events) != 0 {
		t.Errorf("empty purse still paid: gold %d, events %v", leader.Gold, leader.Events)
	}
}

// The Planetary Master is the living realm with the most regions, whatever its
// net worth: update_planet_title (BRE.OVR 0x007aeb) compares total_regions.
// Reported from play: a realm with 13,168 regions and less net worth was
// passed over for one with 10,020. A tie stays with the earlier letter.
func TestPlanetaryMasterGoesToTheMostRegions(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	rich := w.AddHuman("rich", "Richland")
	wide := w.AddHuman("wide", "Wideland")
	rich.Regions = RegionMix{}
	rich.Regions.Coastal = 10_020
	rich.syncLand()
	rich.Tanks = 5_000_000
	wide.Regions = RegionMix{}
	wide.Regions.Coastal = 13_168
	wide.syncLand()
	if w.NetWorth(rich) <= w.NetWorth(wide) {
		t.Fatalf("fixture: want the smaller realm richer, got %d vs %d", w.NetWorth(rich), w.NetWorth(wide))
	}
	w.postMasterNews()
	if w.CurrentMaster != wide.Name {
		t.Errorf("CurrentMaster = %q, want %q (more regions, less net worth)", w.CurrentMaster, wide.Name)
	}

	wide.Regions.Coastal = 10_020
	wide.syncLand()
	if got := w.planetMaster(); got != rich {
		t.Errorf("tie on regions went to %s, want the earlier letter %s", w.EmpireLetter(got), w.EmpireLetter(rich))
	}
}
