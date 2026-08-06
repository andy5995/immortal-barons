package game

import (
	"time"
)

// Travel times (BRE's "Average Turn Around Times to All BBSes"). The original
// keeps one average per node in DATA\TIMES.BR and maintains it with a ping it
// calls TIME_CHECK: a record carrying the sender's node, the target's node and
// the moment it left. The far side bounces the record back untouched; when it
// reaches home again the sender has a true round trip and folds it into the
// running average. The mechanic, the echo and the averaging weights below are
// binary-verified from the overlay routines at BRE.OVR 0x445B0-0x44770.
//
// What it measures is the sysop's transport rather than anything in the game,
// which is why it is worth a screen: a strike or a message only moves when the
// packets do.

// TimeCheck is one round-trip probe. From is the board that sent it and is the
// only board that ever reads the elapsed time; To is the board being measured,
// whose only job is to send the record straight back. Sent is RFC3339 so the
// two clocks involved can sit in different zones.
type TimeCheck struct {
	From string
	To   string
	Sent string
}

// timeNow is time.Now, indirected so a test can hold the clock still.
var timeNow = time.Now

// PingTravelTimes queues a TIME_CHECK to every other known board, once per game
// day. Without a fresh probe the screen would freeze at whatever the last
// exchange measured, and a transport that has since slowed down would go
// unnoticed.
func (w *World) PingTravelTimes() {
	if w.Config.BoardID == "" {
		return
	}
	// The game clock, or the real date on a board whose first maintenance has
	// not run yet — an empty date on both sides would match and never probe.
	today := w.LastMaintDate
	if today == "" {
		today = timeNow().Format(time.DateOnly)
	}
	if w.LastTravelPing == today {
		return
	}
	w.LastTravelPing = today
	sent := timeNow().Format(time.RFC3339)
	for _, board := range w.KnownBoards() {
		p := w.outboxFor(board)
		p.TimeChecks = append(p.TimeChecks, TimeCheck{From: w.Config.BoardID, To: board, Sent: sent})
	}
}

// applyTimeChecks handles both halves of the probe: a record naming us as the
// target goes straight back out unchanged, and one of our own coming home is
// measured. echo collects the ones to return.
func (w *World) applyTimeChecks(checks []TimeCheck) (echo []TimeCheck) {
	for _, tc := range checks {
		switch w.Config.BoardID {
		case tc.To:
			echo = append(echo, tc)
		case tc.From:
			w.recordTravelTime(tc)
		}
	}
	return echo
}

// recordTravelTime folds one completed round trip into the average for the
// board it went to: avg = (avg + 2*elapsed) / 3, weighted toward the newest
// sample as the original weights it.
func (w *World) recordTravelTime(tc TimeCheck) {
	sent, err := time.Parse(time.RFC3339, tc.Sent)
	if err != nil {
		return
	}
	elapsed := timeNow().Sub(sent).Hours() / 24
	if elapsed < 0 {
		return // a clock skewed backwards, not a measurement
	}
	if w.TravelTimes == nil {
		w.TravelTimes = map[string]float64{}
	}
	w.TravelTimes[tc.To] = (w.TravelTimes[tc.To] + TravelAvgNewWeight*elapsed) / TravelAvgDenom
}
