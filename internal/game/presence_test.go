package game

import (
	"encoding/json"
	"testing"
	"time"
)

// TestOnlineWindow walks the stamp's whole life: never played, just acted,
// still inside the window, past it, and cleared by a clean logoff.
func TestOnlineWindow(t *testing.T) {
	advance := holdClock(t, time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC))
	e := &Empire{Name: "Testland"}

	if e.Online() {
		t.Error("a realm that has never played must not read as online")
	}

	e.MarkActive()
	if !e.Online() {
		t.Error("a realm that just acted must read as online")
	}

	// Golden literals, not OnlineWindowSecs arithmetic: expressing the bounds in
	// terms of the constant would follow a retune silently, and the point of the
	// test is that the window is a decision.
	advance(299 * time.Second)
	if !e.Online() {
		t.Error("299s after the last action is still inside the 300s window")
	}
	advance(1 * time.Second)
	if e.Online() {
		t.Error("300s after the last action is outside the window")
	}

	e.MarkActive()
	e.MarkOffline()
	if e.Online() {
		t.Error("a clean logoff must drop the realm off the roster at once")
	}
	if e.LastActive != 0 {
		t.Errorf("MarkOffline left LastActive = %d, want 0", e.LastActive)
	}
}

// TestOnlineSurvivesSave pins the stamp as persisted state. The indicator is
// read by OTHER nodes out of world.json, so a json:"-" here would make every
// baron permanently offline to everyone but themselves — and nothing else in
// the game reads the field, so no other test would notice.
func TestOnlineSurvivesSave(t *testing.T) {
	advance := holdClock(t, time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC))
	w := NewWorldSeed(Config{}, 1)
	e := w.AddHuman("tester", "Testland")
	e.MarkActive()

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var reloaded World
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := reloaded.FindByOwner("tester")
	if got == nil {
		t.Fatal("empire missing after round trip")
	}
	if got.LastActive != e.LastActive {
		t.Errorf("LastActive = %d after round trip, want %d", got.LastActive, e.LastActive)
	}
	if !got.Online() {
		t.Error("a stamp made just now must survive the save as online")
	}
	advance(OnlineWindowSecs * time.Second)
	if got.Online() {
		t.Error("the reloaded stamp must age out like any other")
	}
}
