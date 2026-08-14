package game

// Reset starts a fresh game: this is BRE's sysop "reset" — it wipes the world
// to a clean start and does NOT crown a winner. Crowning happens only when a
// timed league runs out its length (endGame), which is a separate event.
// LastMaster and Bulletin persist across the reset.
func (w *World) Reset() { w.resetForNewGame() }

// endGame ends a timed league: crown the highest-net-worth living empire as
// Planetary Master, then reset for a fresh game. Called when GameLength is
// reached (see turn.go). The sysop -reset command uses Reset (no crowning).
func (w *World) endGame() {
	best := ""
	bestNW := 0
	found := false
	for _, e := range w.Empires {
		if e.Alive {
			if nw := w.NetWorth(e); !found || nw > bestNW {
				bestNW = nw
				best = e.Name
				found = true
			}
		}
	}
	w.resetForNewGame()
	if found {
		w.LastMaster = best // crown after the reset so the trophy survives it
	}
}

// resetForNewGame wipes all empires (humans re-onboard on next login) and
// re-seeds AI, resetting per-game state. LastMaster and Bulletin persist.
// Epoch advances so a packet already sitting in an inbound directory, written
// for the game just wiped, is recognised as stale rather than applied (#104).
func (w *World) resetForNewGame() {
	w.Epoch++
	w.initFreshGame()
}
