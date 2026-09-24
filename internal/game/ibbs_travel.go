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

// PingTravelTimes queues a TIME_CHECK to every other known board on every
// planetary run. Without a fresh probe the screen would freeze at whatever the
// last exchange measured, and a transport that has since slowed down would go
// unnoticed.
//
// This cadence is a deliberate divergence (#287). The original probes once per
// game day, from daily maintenance (write_interbbs_time_check_packet <-
// ovr_044601_entry_02a8 <- run_daily_maintenance, one caller at each step,
// verified 2026-09-21), and IB did the same until #287. A daily probe cost too
// much: a probe lost to a dead link got no replacement until the next game day,
// so a link that came back stayed marked stale for up to a day, and a board
// whose game day stopped moving sent no probes at all (#289). Probing on every
// run makes the screen follow the transport at the rate the transport moves.
// The price is comparability: a board that runs more often folds more samples
// into its averages, so its figures react faster than a slower board's.
//
// It adds at most one small packet per board per run, and none for a board
// that already has something addressed to it in the same run; each probe then
// draws an echo back. -full is a planetary run too, on every caller's launch, so
// a board using it probes once per caller. That is kept on purpose: -full exists
// to exchange packets when a caller enters, and a probe is part of that
// exchange. Every run already broadcasts scores, so the extra traffic is small.
func (w *World) PingTravelTimes() {
	if w.Config.BoardID == "" {
		return
	}
	sent := timeNow().Format(time.RFC3339)
	for _, board := range w.KnownBoards() {
		// A board the roster cannot place gets no probe: the packet would only
		// circle the league and be destroyed, on every run, forever.
		if !w.Routable(board) {
			continue
		}
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
	if w.TravelSeen == nil {
		w.TravelSeen = map[string]string{}
	}
	w.TravelTimes[tc.To] = (w.TravelTimes[tc.To] + TravelAvgNewWeight*elapsed) / TravelAvgDenom
	// Stamped with the arrival, not with tc.Sent: the question the screen has to
	// answer is how long ago this board last heard back, and on a link that has
	// stopped those two are days apart.
	w.TravelSeen[tc.To] = timeNow().Format(time.RFC3339)
}

// TravelAge is how long ago the last completed round trip to `board` came home,
// and whether that is known at all. A board measured before the stamp was kept
// reports ok=false rather than an invented age.
func (w *World) TravelAge(board string) (age time.Duration, ok bool) {
	seen, found := w.TravelSeen[board]
	if !found {
		return 0, false
	}
	at, err := time.Parse(time.RFC3339, seen)
	if err != nil {
		return 0, false
	}
	if d := timeNow().Sub(at); d > 0 {
		return d, true
	}
	return 0, true
}
