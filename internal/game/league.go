package game

// endGame crowns the highest-net-worth living empire as Planetary Master
// and resets the world for a fresh game.
func (w *World) endGame() {
	best := ""
	bestNW := -1
	for _, e := range w.Empires {
		if e.Alive {
			if nw := w.NetWorth(e); nw > bestNW {
				bestNW = nw
				best = e.Name
			}
		}
	}
	if best != "" {
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
	w.seedAIEmpires()
}
