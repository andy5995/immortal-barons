package menu

import (
	"strings"
	"testing"
)

func TestAskYesNo(t *testing.T) {
	cases := []struct {
		keys string
		want bool
	}{
		{"y\r", true},
		{"\r", true},
		{"n\r", false},
		{"N\r", false},
	}
	for _, c := range cases {
		f := &fakeSession{keys: []rune(c.keys)}
		got := askYesNo(f, "Continue?")
		if got != c.want {
			t.Errorf("askYesNo(%q) = %v, want %v", c.keys, got, c.want)
		}
	}
}

func TestIncomeReportWritesNonEmpty(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	incomeReport(f, w, w.Active)
	if f.out.Len() == 0 {
		t.Error("expected incomeReport to write output")
	}
	if !strings.Contains(f.out.String(), "Income Report") {
		t.Error("expected income report heading")
	}
}

func TestEndOfTurnStatsWritesNonEmpty(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	endOfTurnStats(f, w, w.Active)
	if f.out.Len() == 0 {
		t.Error("expected endOfTurnStats to write output")
	}
	if !strings.Contains(f.out.String(), "Turns left") {
		t.Error("expected turns-left line")
	}
}

func TestRunTurnNoTurnsLeftReturnsImmediately(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")} // pause keypress inside ok(), then seeScores' pause
	w := newWorld()
	w.Active.TurnsLeft = 0
	runTurn(f, w)
	if !strings.Contains(f.out.String(), "used all of your turns today") {
		t.Error("expected the no-turns-left message")
	}
}

func TestIncomeReportSurfacesAndClearsPirateRaids(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // one key for the pause
	w := newWorld()
	p := w.Active
	p.PirateRaids = []string{"The Sharks pirates raided you, carrying off 40 troopers, 0 jets, and 5 tanks!"}

	incomeReport(f, w, p)

	if !strings.Contains(f.out.String(), "Sharks pirates raided you") {
		t.Error("income report should show the pirate raid notice")
	}
	if len(p.PirateRaids) != 0 {
		t.Error("income report should clear the raid notice after showing it")
	}
}

func TestPaymentStageAutoPays(t *testing.T) {
	f := &fakeSession{}
	w := newWorld()
	w.AutoPayMaint = true
	p := w.Active
	p.Gold = 100000
	before := p.Gold
	want := p.ForcesUpkeep() + p.RegionUpkeep()

	paymentStage(f, w, p)

	if p.Gold != before-want {
		t.Errorf("auto-pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
	if !strings.Contains(f.out.String(), "Maintenance paid") {
		t.Error("expected the auto-pay confirmation line")
	}
}

func TestPaymentStageManualFullPayNoDesertion(t *testing.T) {
	f := &fakeSession{keys: []rune("\r\r")} // accept suggested (full) for forces, then regions
	w := newWorld()
	w.AutoPayMaint = false
	p := w.Active
	p.Gold = 100000
	p.Support = 100 // skips the optional support-boost prompt
	beforeTroopers := p.Troopers
	want := p.ForcesUpkeep() + p.RegionUpkeep()
	before := p.Gold

	paymentStage(f, w, p)

	if p.Troopers != beforeTroopers {
		t.Errorf("full pay should not desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if p.Gold != before-want {
		t.Errorf("manual full pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
}

func TestPaymentStageManualUnderpayDeserts(t *testing.T) {
	f := &fakeSession{keys: []rune("0\r0\r")} // give 0 to forces, then 0 to regions
	w := newWorld()
	w.AutoPayMaint = false
	p := w.Active
	p.Gold = 100000
	p.Support = 100
	beforeTroopers := p.Troopers

	paymentStage(f, w, p)

	if p.Troopers >= beforeTroopers {
		t.Errorf("underpaying forces should desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if !strings.Contains(f.out.String(), "deserted") {
		t.Error("expected a desertion notice")
	}
}

// TestRunTurnConsumesATurn scripts a full pass through the pipeline for one
// turn: income/status pauses, Return ('0') out of Spending, Attack, Covert,
// and Trading, decline the message prompt, the end-of-turn pause, then
// decline "continue" to stop after one turn.
func TestRunTurnConsumesATurn(t *testing.T) {
	keys := "  0000n\r n\r"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.AutoPayMaint = true // pay maintenance silently; this test is about the turn loop
	left := w.Active.TurnsLeft
	runTurn(f, w)
	if w.Active.TurnsLeft != left-1 {
		t.Errorf("expected TurnsLeft %d, got %d", left-1, w.Active.TurnsLeft)
	}
}
