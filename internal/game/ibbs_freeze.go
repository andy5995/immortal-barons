package game

import (
	"errors"
	"time"
)

// ibbs_freeze.go — the league freeze: the Coordinator stopping the whole league
// so every packet in flight can land before a change that the boards must make
// together, such as a new packet protocol.
//
// Upgrading across a protocol bump used to be done by hand: every sysop closed
// the door, the league waited for its mail to drain, and everyone switched on
// the same evening. A packet still travelling when its reader moved on is held
// and never read (see Protocol). The freeze makes that a single order from node
// 1, and makes each board tell the Coordinator when it has gone quiet, so the
// Coordinator knows when it is safe to announce the upgrade.
//
// While frozen a board:
//   - lets nobody play; the door shows the Coordinator's message and ends;
//   - stops its game clock. Maintenance runs no day while frozen, and the thaw
//     moves the maintenance date on by the frozen days, so they are skipped
//     rather than caught up; every deadline kept as an instant is moved on by
//     the length of the freeze;
//   - applies what arrives and relays what it is passing on, but writes nothing
//     of its own. Anything its game produces waits in the Outbox and goes out
//     after the thaw, stamped by whichever build is running then. The two
//     exceptions are the Coordinator's orders and this board's quiet report.
//
// A relayed packet has to keep moving: it carries its writer's protocol stamp,
// so one held here across an upgrade would arrive as a format its reader has
// moved past.

// LeagueFreeze is the Coordinator's order to freeze or thaw the league. Serial
// increases with every order, so a board can tell a new one from a replay.
type LeagueFreeze struct {
	Serial  int
	Frozen  bool
	Message string `json:",omitempty"` // shown to callers while frozen
}

// QuietReport is a frozen board telling the Coordinator when it last applied a
// packet that was not part of the freeze itself. Serial is the freeze it
// answers, so a report left over from an earlier freeze is ignored. QuietSince
// is a StoredStamp, the form every instant crossing between boards takes.
type QuietReport struct {
	Serial     int
	QuietSince string
}

var (
	ErrAlreadyFrozen = errors.New("the league is already frozen")
	ErrNotFrozen     = errors.New("the league is not frozen")
)

// DeclareLeagueFreeze is the Coordinator freezing (frozen) or thawing the
// league. It changes this board at once and queues the signed order for every
// other.
func (w *World) DeclareLeagueFreeze(frozen bool, message string) error {
	if !w.IsLeagueCoordinator() {
		return ErrNotCoordinator
	}
	if len(w.CoordKey) == 0 {
		return ErrNoCoordKey
	}
	if frozen && w.Frozen {
		return ErrAlreadyFrozen
	}
	if !frozen && !w.Frozen {
		return ErrNotFrozen
	}
	f := &LeagueFreeze{Serial: w.FreezeSerial + 1, Frozen: frozen, Message: message}
	p := Packet{FromBoard: w.Config.BoardID, Date: w.LastMaintDate, Seq: w.NextSeq(), Freeze: f}
	if err := w.SignAsCoordinator(&p); err != nil {
		return err
	}
	w.Outbox = append(w.Outbox, p)
	w.applyLeagueFreeze(f)
	return nil
}

// applyLeagueFreeze carries out a freeze or thaw order. An order no newer than
// the last one applied is a replay and does nothing.
func (w *World) applyLeagueFreeze(f *LeagueFreeze) {
	if f.Serial <= w.FreezeSerial {
		return
	}
	w.FreezeSerial = f.Serial
	now := timeNow()
	switch {
	case f.Frozen && !w.Frozen:
		w.Frozen = true
		w.FrozenAt = now
		w.FreezeMessage = f.Message
		w.QuietSince, w.QuietSent = now, time.Time{}
		w.QuietBoards = nil
	case f.Frozen:
		// Still frozen from an earlier freeze whose thaw never arrived here —
		// lost, or delivered after this order. It must stay frozen, not thaw;
		// only the message the Coordinator gave this time applies. Its quiet
		// report answered the old serial, which the Coordinator no longer
		// files, so it reports again under this one.
		w.FreezeMessage = f.Message
		w.QuietSent = time.Time{}
	case w.Frozen:
		w.shiftDeadlines(now.Sub(w.FrozenAt))
		w.skipFrozenDays(w.FrozenAt, now)
		w.ThawedAt = now
		w.Frozen = false
		w.FrozenAt, w.QuietSince, w.QuietSent = time.Time{}, time.Time{}, time.Time{}
		w.FreezeMessage = ""
		w.QuietBoards = nil
		w.postNews("The League Coordinator has ended the league's pause. Play resumes.")
	}
}

// skipFrozenDays moves the maintenance clock on by the calendar days the league
// spent frozen, so the next maintenance neither simulates them nor loses a day
// the board still owed from before the freeze. It never runs past today.
func (w *World) skipFrozenDays(from, to time.Time) {
	if w.LastMaintDate == "" {
		return
	}
	last, err := time.Parse("2006-01-02", w.LastMaintDate)
	if err != nil {
		return
	}
	d0, _ := time.Parse("2006-01-02", from.Format("2006-01-02"))
	d1, _ := time.Parse("2006-01-02", to.Format("2006-01-02"))
	days := int(d1.Sub(d0).Hours() / 24)
	if days <= 0 {
		return
	}
	next := last.AddDate(0, 0, days).Format("2006-01-02")
	if today := to.Format("2006-01-02"); next > today {
		next = today
	}
	w.LastMaintDate = next
}

// shiftDeadlines moves every deadline kept as an instant on by d, so the time
// the league spent frozen counts against none of them. Deadlines kept in game
// days need nothing: the day did not advance.
func (w *World) shiftDeadlines(d time.Duration) {
	if d <= 0 {
		return
	}
	later := func(t *time.Time) {
		if !t.IsZero() {
			*t = t.Add(d)
		}
	}
	for i := range w.GroupAttacks {
		later(&w.GroupAttacks[i].DepartAt)
	}
	weapons := append([]*Annihilator{w.Annihilator}, w.Incoming...)
	for _, a := range weapons {
		if a != nil {
			later(&a.LaunchAt)
			later(&a.ArrivesAt)
		}
	}
	for _, e := range w.Empires {
		for i := range e.TradeDeals {
			later(&e.TradeDeals[i].Expires)
		}
	}
	// The threats list is this board's copy of what others announced; their
	// boards shift the originals but do not announce them again.
	for i := range w.Threats {
		if at := w.Threats[i].When(); !at.IsZero() {
			w.Threats[i].At = ThreatAt(at.Add(d))
		}
	}
}

// noteFrozenTraffic records that a frozen board applied a packet that was not
// only part of the freeze itself, which restarts its quiet time.
func (w *World) noteFrozenTraffic(p Packet) {
	if !w.Frozen {
		return
	}
	q := p
	q.Freeze, q.Quiet = nil, nil
	if q.HasPayload() {
		w.QuietSince = timeNow()
	}
}

// ReportQuiet queues this board's quiet report for the Coordinator when its
// quiet time has changed since it last reported. The Coordinator's own board
// files it directly.
func (w *World) ReportQuiet() {
	if !w.Frozen || w.QuietSince.Equal(w.QuietSent) {
		return
	}
	r := QuietReport{Serial: w.FreezeSerial, QuietSince: StoredStamp(w.QuietSince)}
	if w.IsLeagueCoordinator() {
		w.QuietSent = w.QuietSince
		w.fileQuietReport(w.Config.BoardID, r)
		return
	}
	// Marked sent only once it is queued, so a run with no roster yet tries
	// again on the next one.
	coord := w.CoordinatorBoardID()
	if coord == "" {
		return
	}
	w.QuietSent = w.QuietSince
	w.Outbox = append(w.Outbox, Packet{FromBoard: w.Config.BoardID, ToBoard: coord,
		Date: w.LastMaintDate, Quiet: &r})
}

// fileQuietReport keeps a board's quiet report on the Coordinator's board, if
// it answers the freeze now in force.
func (w *World) fileQuietReport(board string, r QuietReport) {
	if !w.Frozen || r.Serial != w.FreezeSerial {
		return
	}
	if w.QuietBoards == nil {
		w.QuietBoards = map[string]string{}
	}
	w.QuietBoards[board] = r.QuietSince
}

// FrozenSendable reports whether a frozen board may still write p: the
// Coordinator's orders — the freeze itself, and a ruleset, roster or bulletins
// it sends while frozen — and a quiet report. Everything else waits.
func FrozenSendable(p Packet) bool {
	return p.Freeze != nil || p.Quiet != nil || CarriesCoordinatorOrders(p)
}

// ErrLeagueFrozen refuses a new season while the league is frozen: the order
// would land on boards mid-upgrade, and a board wiped then plays nothing until
// the thaw anyway.
var ErrLeagueFrozen = errors.New("the league is frozen; thaw it before starting a new season")
