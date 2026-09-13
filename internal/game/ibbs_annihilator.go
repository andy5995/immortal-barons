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
		if w.Incoming != nil && w.Incoming.Creator == st.FromBoard {
			w.markAnnihilatorDone(w.Incoming)
			w.Incoming = nil
			w.postNews(fmt.Sprintf("The Gooie Kablooie being built at %s has been dismantled.", st.FromBoard))
		}
		return
	}
	// A weapon we have already finished with, announced once more by a board
	// that had not yet retired the record. Without this the siege runs again.
	if w.Incoming == nil {
		done, haveDone := ParseStamp(w.AnnihilatorDone[st.FromBoard])
		if at, ok := ParseStamp(st.ArrivesAt); ok && haveDone && at.Equal(done) {
			return
		}
	}
	first := w.Incoming == nil
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
	// A status for a weapon that has already been and gone — the builder's board
	// announcing it once more before it retires the record, or one it has since
	// lost to jets — must not raise a second siege. "Already due" is a sound
	// test again now that it is measured on the real interval: a live weapon
	// flies for AnnihilatorFlightDays and its status leaves on the next run, so
	// it reaches us with the arrival still ahead. It was NOT sound against two
	// unrelated day counters, which is what dropped live weapons.
	if first && st.Launched && days < 0 {
		return
	}
	if first {
		w.Incoming = &Annihilator{Creator: st.FromBoard, Intact: 100}
	}
	in := w.Incoming
	// The instant identifies the weapon, so once one is on the books a status
	// carrying a DIFFERENT instant is about some other weapon and must not
	// rewrite this one's — otherwise the stamp filed when this weapon ends
	// records the wrong arrival and stops recognizing this weapon's own late
	// copies. The builder is free to start a second weapon the day after the
	// first lands, so a second generation routinely reports during the first's
	// five-day siege.
	//
	// This does not fix, and is not meant to fix, the fact that w.Incoming is a
	// single slot: a second BOARD's weapon is already lost here today, because
	// wasFlying is true by then and the arrival warning never fires. The pin
	// stops the record being corrupted; it does not make the planet able to
	// watch two weapons at once.
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
	if w.Incoming == nil || !w.Incoming.Launched || w.Incoming.DaysLeft > 0 {
		return
	}
	if w.GameDay < w.Incoming.ArrivesDay {
		return
	}
	w.Incoming.DaysLeft = AnnihilatorSiegeDays
}

// daysUntil is how many whole days from now to at, rounded UP so a weapon due in
// any part of a day still reads as that day rather than as already landed. It
// goes negative for an instant in the past, which is how a late status is told
// from a stale one.
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
