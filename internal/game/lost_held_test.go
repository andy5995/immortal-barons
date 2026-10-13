package game

import (
	"reflect"
	"testing"
)

// recoveryDay runs ReturnLostForces once a game day, from the launch day for
// up to limit days, with board held on the days in [heldFrom, heldUntil), and
// returns how many days after launch the first recovery came — or -1 if none did.
func recoveryDay(t *testing.T, w *World, board string, heldFrom, heldUntil, limit int) int {
	t.Helper()
	start := w.GameDay
	for d := 0; d <= limit; d++ {
		w.GameDay = start + d
		var held map[string]bool
		if d >= heldFrom && d < heldUntil {
			held = map[string]bool{board: true}
		}
		if n := w.ReturnLostForces(held); n > 0 {
			return d
		}
		var want []string
		if held != nil && w.Config.LostForcesDays > 0 && w.InFlight[0].TargetBoard == board {
			want = []string{board}
		}
		if got := w.PausedRecovery(); !reflect.DeepEqual(got, want) {
			t.Errorf("day %d: PausedRecovery = %v, want %v", d, got, want)
		}
	}
	return -1
}

// launchedStrike sends a group strike at "faraway" and returns its owner.
func launchedStrike(t *testing.T, lostDays int) (*World, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = lostDays
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers = 1000
	if _, err := w.CreateGroupAttack(e, "faraway", "Rome", GroupAttackHoursMin, AttackForce{Troopers: 600}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	w.LaunchDueGroupAttacksAt(afterDeparture())
	if len(w.InFlight) != 1 || w.InFlight[0].LaunchedDay != w.GameDay {
		t.Fatalf("in flight = %+v, want one strike launched today", w.InFlight)
	}
	return w, e
}

// A strike aimed at a board whose packets are held is not given up at
// LostForcesDays: its answer may be one of the held packets (#190).
func TestLostForcesPauseWhileTheTargetIsHeld(t *testing.T) {
	w, e := launchedStrike(t, 3)
	// Held from launch until day 5. The launch day is not counted twice, so
	// days 1-4 are the held ones, then the full three.
	if got := recoveryDay(t, w, "faraway", 0, 5, 40); got != 8 {
		t.Errorf("recovered on day %d, want 8 (launch day + 4 held + 3 days' wait)", got)
	}
	if e.Troopers != 1000 {
		t.Errorf("troopers = %d, want all 1000 home", e.Troopers)
	}
}

// A hold in the middle of the window extends it by the held span: the held
// days do not count, and the wait resumes when it clears.
func TestLostForcesHoldMidWindowExtendsIt(t *testing.T) {
	w, _ := launchedStrike(t, 3)
	// Day 0 is the launch day, which never counts; held days 1-3; the wait runs
	// from day 4.
	if got := recoveryDay(t, w, "faraway", 1, 4, 40); got != 7 {
		t.Errorf("recovered on day %d, want 7 (launch day + 3 days' wait + 3 held)", got)
	}
}

// A hold on some other board changes nothing.
func TestLostForcesHoldOnAnotherBoardDoesNotPause(t *testing.T) {
	w, _ := launchedStrike(t, 3)
	if got := recoveryDay(t, w, "elsewhere", 0, 99, 40); got != 4 {
		t.Errorf("recovered on day %d, want 4 (launch day + 3 days' wait)", got)
	}
}

// A hold that never clears still gives up at the backstop, counted from launch.
func TestLostForcesBackstopFiresWhileHeld(t *testing.T) {
	w, e := launchedStrike(t, 3)
	if got := recoveryDay(t, w, "faraway", 0, 999, 40); got != 16 {
		t.Errorf("recovered on day %d, want 16 (launch day + 5 x 3)", got)
	}
	if e.Troopers != 1000 {
		t.Errorf("troopers = %d, want all 1000 home", e.Troopers)
	}
}

// LostForcesDays 0 means never, held or not, and nothing is reported paused.
func TestLostForcesNeverWhenOff(t *testing.T) {
	w, _ := launchedStrike(t, 0)
	if got := recoveryDay(t, w, "faraway", 5, 10, 200); got != -1 {
		t.Errorf("recovered on day %d with the recovery off", got)
	}
	w.ReturnLostForces(map[string]bool{"faraway": true})
	if got := w.PausedRecovery(); got != nil {
		t.Errorf("PausedRecovery = %v with the recovery off, want none", got)
	}
}

// A terror op's agents wait out the hold the same way a strike's forces do.
func TestLostTerrorOpPausesWhileHeld(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = 3
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Agents = 20
	if _, err := w.SendTerror(e, "faraway", "Rome", 5, TerrorOpDemoralize); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	if got := recoveryDay(t, w, "faraway", 0, 4, 40); got != 7 {
		t.Errorf("recovered on day %d, want 7 (launch day + 3 held + 3 days' wait)", got)
	}
	if e.Agents != 20 {
		t.Errorf("agents = %d, want all 20 home", e.Agents)
	}
}

// A trade bid's escrow waits out the hold the same way, and the backstop
// refunds it when the hold never clears.
func TestLostTradeBidPausesWhileHeld(t *testing.T) {
	bw, buyer, _, _ := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	if got := recoveryDay(t, bw, "Bravo", 0, 2, 40); got != 5 {
		t.Errorf("refunded on day %d, want 5 (launch day + 1 held + 3 days' wait)", got)
	}
	if buyer.Gold != goldBefore {
		t.Errorf("gold = %d, want the full %d returned", buyer.Gold, goldBefore)
	}

	bw, buyer, _, _ = twoTradingBoards(t, 100, 500)
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	if got := recoveryDay(t, bw, "Bravo", 0, 999, 40); got != 16 {
		t.Errorf("refunded on day %d under a hold that never clears, want 16", got)
	}
}

// A wait of 1 is a full day: a strike sent on day 0 is not given up at the next
// day's run, before its result could make the round trip (#306).
func TestLostForcesWaitOfOneIsAFullDay(t *testing.T) {
	w, e := launchedStrike(t, 1)
	if got := recoveryDay(t, w, "elsewhere", 0, 0, 40); got != 2 {
		t.Errorf("recovered on day %d, want 2 (launch day + 1 day's wait)", got)
	}
	if e.Troopers != 1000 {
		t.Errorf("troopers = %d, want all 1000 home", e.Troopers)
	}
}
