package game

// ibbs_spy.go — spy reports on other planets, and the global recon that
// gathers them.

// SpyReport is intel on a remote empire, stored in the planet-wide Spy
// Database and readable by every baron here. Populated by interplanetary spy
// ops (built in a later increment).
type SpyReport struct {
	Board   string
	Empire  string
	Date    string
	Land    int
	Offense int
	Defense int
	Gold    int64
}

// spyReport is what this board tells another about one of its realms: the
// figures as they stand right now, which is what separates a spy's word from
// the shared score table.
func (w *World) spyReport(e *Empire) SpyReport {
	return SpyReport{
		Board:   w.Config.BoardID,
		Empire:  e.Name,
		Date:    w.LastMaintDate,
		Land:    e.Land,
		Offense: e.Offense(),
		Defense: e.Defense(),
		Gold:    e.Gold,
	}
}

// GlobalReconRequest is the Coordinator Ops menu's own scouting sweep (#48):
// one request to every other board in the league, each answered with a report on
// every realm that board holds. The original posts `Recon Requests Created to
// All BBSs` and says no more, so what it charges is not established — IB spends
// ONE agent for the sweep rather than one per board, because the alternative
// prices the Coordinator out of the item as the league grows.
//
// It returns how many boards were asked, so the screen can say so.
func (w *World) GlobalReconRequest(e *Empire) (int, error) {
	if e.Agents < 1 {
		return 0, ErrNoAgents
	}
	var boards []string
	seen := map[string]bool{w.Config.BoardID: true}
	for _, n := range w.LeagueNodes {
		if !seen[n.Name] {
			seen[n.Name] = true
			boards = append(boards, n.Name)
		}
	}
	for _, b := range w.RemoteBoards {
		if !seen[b.BoardID] {
			seen[b.BoardID] = true
			boards = append(boards, b.BoardID)
		}
	}
	if len(boards) == 0 {
		return 0, nil
	}
	e.Agents--
	for _, b := range boards {
		w.NextAttackID++
		req := ReconRequest{ID: w.NextAttackID, FromBoard: w.Config.BoardID, FromOwner: e.Owner}
		pkt := w.outboxFor(b)
		pkt.Recon = append(pkt.Recon, req)
	}
	return len(boards), nil
}
