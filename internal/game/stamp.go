package game

import (
	"strings"
	"time"

	// The zone database, compiled in. A door runs wherever its BBS runs, and a
	// Windows host has no /usr/share/zoneinfo at all, so a player's zone would
	// silently fall back to UTC on exactly the platform the door most often runs
	// on. This is the standard library's own copy; it costs about 450 KB.
	_ "time/tzdata"
)

// StampFormat is how the original prints a date and time — two spaces between
// them. Used for a message's When and for a recap entry's stamp.
//
// A rendered stamp always carries its zone (stampZone), because a league's
// boards are not on one clock and a bulletin written on one is read on the
// others: an unmarked time is right for whoever wrote it and wrong for most of
// the people reading it (#267).
const StampFormat = "01/02/2006  15:04:05"

// RecordedTimeFormat is how the sysop reports stamp a time, copied from the
// original's own BBSINFO.LST: MM/DD/YYYY HH:MM:SS.
const RecordedTimeFormat = "01/02/2006 15:04:05"

// stampZone is the zone marker appended to every rendered stamp. Go's "MST"
// element prints the zone's own abbreviation — "UTC", "EST", or a numeric
// "+0530" where the zone has no lettered name.
const stampZone = " MST"

// Stamp renders t for a reader in loc — their own zone, from Preferences.
func Stamp(t time.Time, loc *time.Location) string {
	return t.In(zoneOr(loc)).Format(StampFormat + stampZone)
}

// StoredStamp is the form a stamp is SAVED in: UTC, so the one string a packet
// carries between boards means the same thing on each of them. It is re-rendered
// into the reader's zone by StampIn when it reaches a screen.
func StoredStamp(t time.Time) string { return Stamp(t, time.UTC) }

// Recorded stamps the sysop reports and logs. Those are files rather than
// screens — no reader to have a preference — so they are UTC and say so.
func Recorded(t time.Time) string {
	return t.UTC().Format(RecordedTimeFormat + stampZone)
}

// ParseStamp reads a stamp written by Stamp or Recorded back into an instant.
// It reports false for a stamp written before stamps carried a zone: those are
// whatever clock the board that wrote them was on, which is not recoverable, so
// callers show them as they stand rather than converting them into a lie.
func ParseStamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{StampFormat, RecordedTimeFormat} {
		if t, err := time.Parse(layout+stampZone, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// StampIn re-renders a stored stamp into loc, for a reader whose Preferences
// name a zone. An unrecognised or zone-less stamp is returned unchanged.
func StampIn(when string, loc *time.Location) string {
	t, ok := ParseStamp(when)
	if !ok {
		return when
	}
	return Stamp(t, loc)
}

// LocalZone is the stored value meaning "the clock this board runs on". It is a
// sentinel rather than the host's IANA name because the name is not knowable
// from Go and because a board that moves takes its players' choice with it.
//
// No drop file carries the CALLER's zone — DOOR32.SYS, DOOR.SYS and PCBOARD.SYS
// have no such field, and BBSDEV.DRP's one time is the logoff deadline, which is
// the board's. So the two answers a player can be offered without asking them to
// name a zone are UTC and the board's own clock.
const LocalZone = "Local"

// TimeOfDay renders just the clock time of a stored stamp, for the news feed,
// whose day is already on the heading above it (#267). It returns "" for a line
// that carries no time, which is what a world saved before news was stamped has.
func TimeOfDay(when string, loc *time.Location) string {
	t, ok := ParseStamp(when)
	if !ok {
		return ""
	}
	return t.In(zoneOr(loc)).Format("15:04:05" + stampZone)
}

// Zone looks up an IANA zone name ("America/New_York"), LocalZone, or "" for
// UTC. An unknown name is UTC — the default, and the fallback, since a stamp in
// a zone nobody can name is worse than one in the zone everybody shares.
func Zone(name string) *time.Location {
	switch name {
	case "":
		return time.UTC
	case LocalZone:
		return time.Local
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// ValidZone reports whether name is a zone this build can load. The Preferences
// picker asks before it stores, so a typo is refused at the prompt rather than
// quietly leaving the player on UTC.
func ValidZone(name string) bool {
	if name == "" || name == LocalZone {
		return true
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// Location is the zone this realm's owner reads times in.
func (e *Empire) Location() *time.Location {
	if e == nil {
		return time.UTC
	}
	return Zone(e.TimeZone)
}

func zoneOr(loc *time.Location) *time.Location {
	if loc == nil {
		return time.UTC
	}
	return loc
}
