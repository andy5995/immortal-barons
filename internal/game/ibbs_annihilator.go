package game

import "fmt"

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
			w.Incoming = nil
			w.postNews(fmt.Sprintf("The Gooie Kablooie being built at %s has been dismantled.", st.FromBoard))
		}
		return
	}
	first := w.Incoming == nil
	// A status for a weapon that has already been and gone — the builder's board
	// announcing it once more before it retires the record — must not raise a
	// second siege.
	if first && st.Launched && st.ArrivesDay < w.GameDay {
		return
	}
	if first {
		w.Incoming = &Annihilator{Creator: st.FromBoard, Intact: 100}
	}
	in := w.Incoming
	wasFlying := in.Launched
	in.TargetBoard = w.Config.BoardID
	in.ArrivesDay = st.ArrivesDay
	if st.Launched {
		in.Launched = true
	}
	if st.Intact > 0 && st.Intact < in.Intact {
		in.Intact = st.Intact
	}
	switch {
	case st.Launched && !wasFlying:
		hours := (st.ArrivesDay - w.GameDay) * 24
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
