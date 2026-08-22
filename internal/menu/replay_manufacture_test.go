package menu

import "testing"

// Reproduction for Andy's live report: DC mid-turn (at the Spending menu, having
// listed units on the market), come back, and a turn's worth of production
// (~Manufacture) appears in inventory — as if the turn advanced. This asserts a
// replay does NOT run Manufacture a second time and does NOT advance the turn.
func TestReplayDoesNotDoubleManufactureOrAdvanceTurn(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Regions.Industrial = 246
	p.Land = p.Regions.Total()
	p.ProdTroopers, p.ProdJets, p.ProdTurrets = 0, 0, 0
	p.ProdBombers, p.ProdTanks, p.ProdCarriers = 0, 100, 0
	p.Specialized = ""
	p.ProdInitialized = true
	p.Food = 1_000_000_000         // fed: no food-market prompt
	p.Gold = 2_000_000_000         // rich enough for any purchase, and within a 32-bit int
	p.Support, p.Morale = 100, 100 // no support/morale boost prompts
	p.Agents = 0                   // skip the covert stage
	p.TurnsLeft = 5
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.AutoFeed = true

	turnsBefore := p.TurnsLeft

	// boot runs one runTurn pass that ends in an idle-boot (session.End panic),
	// which we recover the way GameLoop's GuardEnd does.
	boot := func(keys string) {
		defer func() { _ = recover() }()
		f := &fakeSession{keys: []rune(keys), boot: true}
		runTurn(f, w)
	}

	// First pass: pauses for income/status/food-consumed, then boot at Spending.
	boot("      ")
	firstTanks := p.Tanks
	if firstTanks == 0 {
		t.Fatalf("expected Manufacture to produce tanks on the first pass, got 0")
	}
	if !p.TurnProgress.IncomeCollected {
		t.Fatalf("expected IncomeCollected set after the first pass")
	}

	// Replay: intro/payment/feed are skipped, landing straight at Spending, where
	// we boot again immediately. Must not produce again, must not advance the turn.
	boot("")
	if p.Tanks != firstTanks {
		t.Errorf("replay double-produced tanks (turn seemed to advance): %d -> %d", firstTanks, p.Tanks)
	}
	if p.TurnsLeft != turnsBefore {
		t.Errorf("replay advanced the turn: TurnsLeft %d -> %d", turnsBefore, p.TurnsLeft)
	}
}
