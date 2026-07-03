package game

// endGame crowns the highest-net-worth living empire as Planetary Master
// and resets the world for a fresh game.
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
	if found {
		w.LastMaster = best
	}
	w.resetForNewGame()
}

// resetForNewGame wipes all empires (humans re-onboard on next login) and
// re-seeds AI, resetting per-game state. LastMaster and Bulletin persist.
func (w *World) resetForNewGame() {
	w.Empires = nil
	w.Alliances = nil
	w.GameDay = 0
	// The next session re-resolves Active after maintenance.
	w.Active = nil
	w.seedAIEmpires()
}
