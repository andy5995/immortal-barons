package game

import (
	"math"
	"testing"
	"time"
)

// holdClock freezes timeNow at base and returns a func the test advances it
// with, plus a cleanup that restores the real clock.
func holdClock(t *testing.T, base time.Time) func(time.Duration) {
	t.Helper()
	now := base
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = time.Now })
	return func(d time.Duration) { now = now.Add(d) }
}

func travelWorld(board string) *World {
	w := NewWorldSeed(Config{BoardID: board}, 1)
	w.LastMaintDate = "2026-08-06"
	return w
}

// TestTravelTimeRoundTrip walks the whole probe: a ping goes out, the far board
// bounces it back untouched, and the sender measures the round trip. The far
// board must record nothing — only the originator ever holds a figure.
func TestTravelTimeRoundTrip(t *testing.T) {
	advance := holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := travelWorld("Nova Hub")
	here.LeagueNodes = []LeagueNode{{Number: 2, Name: "The Eclipse"}}
	there := travelWorld("The Eclipse")

	here.PingTravelTimes()
	if len(here.Outbox) != 1 || len(here.Outbox[0].TimeChecks) != 1 {
		t.Fatalf("expected one probe queued, got outbox %+v", here.Outbox)
	}
	ping := here.Outbox[0]

	advance(3 * time.Hour)
	echo := there.ApplyPacket(ping)
	if len(echo.TimeChecks) != 1 || echo.TimeChecks[0].Sent != ping.TimeChecks[0].Sent {
		t.Fatalf("the far board must echo the probe unchanged, got %+v", echo.TimeChecks)
	}
	if len(there.TravelTimes) != 0 {
		t.Errorf("the far board recorded a time it never measured: %v", there.TravelTimes)
	}

	advance(3 * time.Hour)
	here.ApplyPacket(echo)
	// Six hours out and back is 0.25 days; the first sample folds into an empty
	// average as (0 + 2*0.25)/3.
	if got, want := here.TravelTimes["The Eclipse"], 0.5/3; math.Abs(got-want) > 1e-9 {
		t.Errorf("round trip recorded as %v days, want %v", got, want)
	}
}

// TestTravelTimeAverageWeightsNewest pins BRE's averaging: avg = (avg + 2*new)/3.
func TestTravelTimeAverageWeightsNewest(t *testing.T) {
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	advance := holdClock(t, base)
	w := travelWorld("Nova Hub")
	w.TravelTimes = map[string]float64{"The Eclipse": 1}

	advance(24 * time.Hour) // a probe sent a day ago comes home now: elapsed 1 day
	w.recordTravelTime(TimeCheck{From: "Nova Hub", To: "The Eclipse", Sent: base.Format(time.RFC3339)})
	if got, want := w.TravelTimes["The Eclipse"], 1.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("a sample equal to the average moved it to %v, want %v", got, want)
	}

	// A four-day trip against a one-day average: (1 + 2*4)/3 = 3.
	advance(4 * 24 * time.Hour)
	sent := base.Add(24 * time.Hour).Format(time.RFC3339)
	w.recordTravelTime(TimeCheck{From: "Nova Hub", To: "The Eclipse", Sent: sent})
	if got, want := w.TravelTimes["The Eclipse"], 3.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("average is %v days, want %v", got, want)
	}
}

// TestTravelPingIsOncePerDay checks the probe does not go out again until the
// game day turns over — a run several times a day would otherwise flood the
// league with probes.
func TestTravelPingIsOncePerDay(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	w := travelWorld("Nova Hub")
	w.LeagueNodes = []LeagueNode{{Number: 2, Name: "The Eclipse"}}
	w.PingTravelTimes()
	w.Outbox = nil
	w.PingTravelTimes()
	if len(w.Outbox) != 0 {
		t.Errorf("a second run the same day queued another probe: %+v", w.Outbox)
	}
	w.LastMaintDate = "2026-08-07"
	w.PingTravelTimes()
	if len(w.Outbox) != 1 {
		t.Errorf("the next day queued %d packets, want 1", len(w.Outbox))
	}
}

// TestEchoOnlyPacketHasPayload guards the transport check: a reply that is
// nothing but an echoed probe still has to be sent, or no round trip ever
// completes and the screen stays at "No Data" forever.
func TestEchoOnlyPacketHasPayload(t *testing.T) {
	p := Packet{TimeChecks: []TimeCheck{{From: "a", To: "b", Sent: "x"}}}
	if !p.HasPayload() {
		t.Error("an echo-only packet reports no payload")
	}
	if (Packet{}).HasPayload() {
		t.Error("an empty packet reports a payload")
	}
}
