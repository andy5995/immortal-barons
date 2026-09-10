package game

import "time"

// threats.go — what other planets have aimed at this one, kept as records
// rather than only as sentences (#268).
//
// A watcher's warning has always arrived as planet news, which is what the
// original's NEWS_DATA record makes it, and news is FROZEN: "leaves in 12 hours"
// means twelve hours from whenever that line was written, and says nothing an
// hour later. The line stays exactly as it is — a log entry that changed
// meaning as it was read would be a worse log entry — and a record rides beside
// it so the Incoming view can count the same threat down live.
//
// Nothing here is a new source of knowledge. A record is written only where a
// report was already being sent, so a planet with no watcher out still learns
// nothing, and the watched planet is still never told it is being watched.

// The kinds of threat a planet can be told about. Strings rather than an
// integer: they cross the wire, and a packet is read by hand often enough that
// "gooie" beats 2.
const (
	ThreatAttack = "attack" // a group attack assembling against us
	ThreatGooie  = "gooie"  // a Gooie Kablooie being built for us
)

// Threat is one thing another planet has aimed at this one.
type Threat struct {
	FromBoard string
	Kind      string
	// ID is the sending board's own id for the thing — a GroupAttack.ID, or 0
	// for the Gooie, of which a planet builds one at a time. It is what makes a
	// later report REPLACE an earlier one instead of listing the same strike
	// twice: a watcher who arrives after the party was formed is answered with
	// everything already standing, which is the same party again.
	ID int
	// Target names the baron aimed at, or is empty for the whole planet.
	Target string
	// At is when it leaves (a group attack) or launches (a Gooie): RFC 3339 in
	// UTC, a string for the reason TimeCheck.Sent is one — a time.Time on the
	// wire drags the runtime's own Location and zone structs into the frozen
	// packet shape, where a Go release could move them. Empty where the far
	// board could not say: a Gooie still being funded has no launch date yet.
	At string `json:",omitempty"`
	// Gone marks a threat called off: the party disbanded, the weapon
	// dismantled. It travels as a record of its own so the reader's list can
	// drop the row rather than counting down to something that is not coming.
	Gone bool `json:",omitempty"`
}

// When is the instant At names, or the zero time when it names none.
func (t Threat) When() time.Time {
	if t.At == "" {
		return time.Time{}
	}
	at, err := time.Parse(time.RFC3339, t.At)
	if err != nil {
		return time.Time{}
	}
	return at
}

// ThreatAt renders an instant for the wire.
func ThreatAt(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// Same reports whether two records are about the same thing.
func (t Threat) Same(o Threat) bool {
	return t.FromBoard == o.FromBoard && t.Kind == o.Kind && t.ID == o.ID
}

// noteThreat files a record arriving from another planet, replacing any earlier
// one about the same thing and dropping it when it is called off.
func (w *World) noteThreat(t Threat) {
	if t.FromBoard == "" || t.Kind == "" {
		return
	}
	for i, have := range w.Threats {
		if !have.Same(t) {
			continue
		}
		if t.Gone {
			w.Threats = append(w.Threats[:i], w.Threats[i+1:]...)
			return
		}
		w.Threats[i] = t
		return
	}
	if !t.Gone {
		w.Threats = append(w.Threats, t)
	}
}

// reportThreat sends a record to a planet watching this one, on the same terms
// reportToSpy sends the sentence: no watcher, no report.
func (w *World) reportThreat(board string, t Threat) {
	if !w.watching(board) {
		return
	}
	t.FromBoard = w.Config.BoardID
	p := w.outboxFor(board)
	p.Threats = append(p.Threats, t)
}

// ThreatMemoryHours is how long a threat stays on the list after its hour has
// come and gone. The force has left by then and its arrival is the planetary
// run's business, but a row that vanished the moment it departed would take the
// warning off the screen at the one moment it matters most.
const ThreatMemoryHours = 24

// expireThreats drops what is long past. Run once a game day.
func (w *World) expireThreats() {
	cutoff := timeNow().Add(-ThreatMemoryHours * time.Hour)
	kept := w.Threats[:0]
	for _, t := range w.Threats {
		if at := t.When(); !at.IsZero() && at.Before(cutoff) {
			continue
		}
		kept = append(kept, t)
	}
	w.Threats = kept
}

// IncomingThreats is everything aimed at this planet, soonest first, for the
// Incoming view. A threat whose hour has not been named sorts last: it is real,
// but nothing can be said about when.
func (w *World) IncomingThreats() []Threat {
	out := append([]Threat(nil), w.Threats...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && threatSoonerThan(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func threatSoonerThan(a, b Threat) bool {
	x, y := a.When(), b.When()
	switch {
	case x.IsZero():
		return false
	case y.IsZero():
		return true
	}
	return x.Before(y)
}
