package game

import (
	"fmt"
	"time"
)

// ibbs_annihilator.go — telling the league about this planet's Gooie
// Annihilator, and tracking one aimed at us. The weapon itself is annihilator.go.

// ExportAnnihilatorStatus tells the targeted planet about this one's weapon, whether
// it is still being funded or already in the air (#63).
func (w *World) ExportAnnihilatorStatus() {
	if w.Annihilator == nil {
		return
	}
	d := w.Annihilator
	// A weapon still on the ground is the builders' own business. The target
	// learns of it only through a SpyGuy posted here — BRE's "destined for our
	// planet is under construction at ..." belongs to show_gooie_arrival_time,
	// whose only caller is the SPY_GUY receiver, and the funding and dismantle
	// reports are gated on the target's spy counter as well. IB broadcast the
	// whole build for free until 2026-08-18, which left the watcher with nothing
	// to report that his planet did not already know.
	if !d.Launched {
		return
	}
	w.enqueueAnnihilator(d.TargetBoard, &AnnihilatorStatus{
		FromBoard:  w.Config.BoardID,
		Funded:     d.Funded,
		Launched:   d.Launched,
		ArrivesAt:  arrivalStamp(d.ArrivesAt),
		ArrivesDay: d.ArrivesDay,
		Intact:     d.Intact,
	})
}

// ExportAnnihilatorGone tells the targeted planet to stop watching, because the
// weapon aimed at it was dismantled.
func (w *World) ExportAnnihilatorGone(board string) {
	w.enqueueAnnihilator(board, &AnnihilatorStatus{FromBoard: w.Config.BoardID, Dismantled: true})
}

func (w *World) enqueueAnnihilator(board string, st *AnnihilatorStatus) {
	w.outboxFor(board).Annihilator = st
}

// applyAnnihilatorStatus takes in what another planet says about the weapon it is
// pointing at us, and posts the warning its barons need.
func (w *World) applyAnnihilatorStatus(st *AnnihilatorStatus) {
	if st.Dismantled {
		if d := w.IncomingFrom(st.FromBoard); d != nil {
			w.forgetIncoming(d)
			w.postNews(fmt.Sprintf("The Gooie Kablooie being built at %s has been dismantled.", st.FromBoard))
		}
		return
	}
	// A weapon we have already finished with, announced once more by a board
	// that had not yet retired the record. Without this the siege runs again.
	in := w.IncomingFrom(st.FromBoard)
	if in == nil {
		done, haveDone := ParseStamp(w.AnnihilatorDone[st.FromBoard])
		if at, ok := ParseStamp(st.ArrivesAt); ok && haveDone && at.Equal(done) {
			return
		}
	}
	first := in == nil
	// How far off the arrival is, measured on OUR clock. The packet carries an
	// instant for this reason: st.ArrivesDay counts from the sender's own first
	// maintenance and says nothing about our calendar, so comparing it against
	// w.GameDay dropped live weapons and invented countdowns of thousands of
	// hours. A packet with no instant is from a board that predates the field,
	// and falls back to the old, hazardous reading rather than to nothing.
	days := st.ArrivesDay - w.GameDay
	if at, ok := ParseStamp(st.ArrivesAt); ok {
		days = daysUntil(at, timeNow())
	}
	// Age decides only for a status with NO instant, where there is nothing else
	// to go on: such a packet's day number counts from the SENDER's first
	// maintenance and means nothing here, so an old one is likelier a weapon
	// already gone than a live one, and that is the best a legacy packet allows.
	//
	// With an instant, identity decides instead (AnnihilatorDone, above), and an
	// arrival already in the past must still LAND. A board that was down across
	// the arrival has to find its planet under siege when it comes back: the
	// original has no such problem, because it sends one attack packet at arrival
	// and the target applies it whenever it next reads inbound. Refusing a late
	// status here is the silent disappearance this field exists to end.
	if _, haveInstant := ParseStamp(st.ArrivesAt); first && st.Launched && !haveInstant && days < 0 {
		return
	}
	if first {
		in = &Annihilator{Creator: st.FromBoard, Intact: 100}
		w.Incoming = append(w.Incoming, in)
	}
	// The instant identifies the weapon, so once one from this board is on the
	// books a status carrying a DIFFERENT instant is about some other weapon of
	// theirs and must not rewrite this one's — otherwise the stamp filed when
	// this weapon ends records the wrong arrival and stops recognizing this
	// weapon's own late copies. The builder is free to start a second the day
	// after the first lands, so a second generation routinely reports during the
	// first's siege. The records are per builder board, so this is only ever the
	// same board's next weapon; a different board gets its own row.
	if at, ok := ParseStamp(st.ArrivesAt); ok && !first && !in.ArrivesAt.IsZero() && !at.Equal(in.ArrivesAt) {
		return
	}
	wasFlying := in.Launched
	in.TargetBoard = w.Config.BoardID
	// Translated onto OUR day count, so the siege tick and the countdown both
	// read a number that means something here.
	in.ArrivesDay = w.GameDay + days
	if at, ok := ParseStamp(st.ArrivesAt); ok {
		in.ArrivesAt = at
	}
	if st.Launched {
		in.Launched = true
	}
	if st.Intact > 0 && st.Intact < in.Intact {
		in.Intact = st.Intact
	}
	switch {
	case st.Launched && !wasFlying:
		hours := days * 24
		if hours < 0 {
			hours = 0
		}
		w.postNews(fmt.Sprintf("A Gooie Kablooie arrives from %s in %d hours.", st.FromBoard, hours))
	case first:
		w.postNews(fmt.Sprintf("A Gooie Kablooie destined for our planet is under construction at %s.", st.FromBoard))
	}
}

// ArriveAnnihilator lands an incoming weapon whose flight is over. It does not
// go off: it settles on the planet and begins its siege, and the damage is the
// daily tick's (#112). Called from the planetary step, so the warning has had
// every day of the flight to reach the barons.
func (w *World) ArriveAnnihilator() {
	for _, d := range w.Incoming {
		if !d.Launched || d.DaysLeft > 0 || w.GameDay < d.ArrivesDay {
			continue
		}
		d.DaysLeft = AnnihilatorSiegeDays
	}
}

// daysUntil is how many whole days from now to at, rounded UP so a weapon due in
// any part of a day still reads as that day rather than as already landed. The
// division truncates toward zero, so an arrival less than a day ago reads as 0
// and only one further back reads negative.
func daysUntil(at, now time.Time) int {
	d := at.Sub(now)
	days := int(d / (24 * time.Hour))
	if d > 0 && d%(24*time.Hour) != 0 {
		days++
	}
	return days
}

// arrivalStamp is Recorded, except that a zero time sends NOTHING. Recorded
// renders a zero time as "01/01/0001 ...", which ParseStamp happily accepts and
// daysUntil then reads as 106,751 days in the past — so the receiver drops the
// weapon as stale, which is the exact disappearance this field exists to stop.
// A weapon launched under a build that predated the field has no instant, and
// its first status after an upgrade is precisely that case.
func arrivalStamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return Recorded(at)
}

// markAnnihilatorDone records that this planet has finished with in, so a copy
// of its status arriving afterwards is recognized as a weapon already dealt
// with rather than as a new one. See World.AnnihilatorDone. A weapon with no
// arrival instant — from a board that predates the field — cannot be recorded,
// and is left to the days < 0 test as before.
func (w *World) markAnnihilatorDone(in *Annihilator) {
	if in == nil || in.Creator == "" || in.ArrivesAt.IsZero() {
		return
	}
	if w.AnnihilatorDone == nil {
		w.AnnihilatorDone = map[string]string{}
	}
	w.AnnihilatorDone[in.Creator] = Recorded(in.ArrivesAt)
}
