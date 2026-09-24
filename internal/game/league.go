package game

// Reset starts a fresh game: this is BRE's sysop "reset". It wipes every empire
// (humans re-onboard on their next login) and re-seeds the AI barons, and it
// does NOT crown a winner — crowning happens only when a timed league runs out
// its length (endGame), which is a separate event. LastMaster and the bulletin
// persist across the reset.
func (w *World) Reset() { w.initFreshGame() }

// endGame ends a timed league: crown the Planetary Master (planetMaster, the
// living realm with the most regions), then reset for a fresh game. Called when GameLength is
// reached (see turn.go). The sysop -reset command uses Reset (no crowning).
func (w *World) endGame() {
	best := ""
	if m := w.planetMaster(); m != nil {
		best = m.Name
	}
	w.initFreshGame()
	if found {
		w.LastMaster = best // crown after the reset so the trophy survives it
	}
}
