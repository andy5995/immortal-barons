package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The picker's first two answers are the ones a player can give without knowing
// an IANA name (#267): UTC, which a league agrees on, and the board's own clock.
func TestTimeZonePickerOffersUTCAndTheBoard(t *testing.T) {
	for _, c := range []struct {
		keys string
		want string
	}{
		{"1\r\r", ""},
		{"2\r\r", game.LocalZone},
	} {
		w := newWorld()
		w.Plain = true
		w.Player().TimeZone = "Asia/Tokyo"
		f := &fakeSession{keys: []rune(c.keys)}
		pickTimeZone(f, w)
		if got := w.Player().TimeZone; got != c.want {
			t.Errorf("keys %q left TimeZone %q, want %q", c.keys, got, c.want)
		}
	}
}

// A zone chosen from the list is stored by name, and the confirmation shows what
// a time will now look like — the thing the player is really choosing between.
func TestTimeZonePickerTakesAZoneFromTheList(t *testing.T) {
	w := newWorld()
	w.Plain = true
	f := &fakeSession{keys: []rune("3\r3\r1\r\r")} // choose a zone, Europe, its first
	pickTimeZone(f, w)

	want := game.ZoneRegions[2].Zones[0]
	if got := w.Player().TimeZone; got != want {
		t.Errorf("TimeZone = %q, want %q", got, want)
	}
	if out := f.out.String(); !strings.Contains(out, "Times now read") {
		t.Errorf("the picker did not show what a time now reads:\n%s", out)
	}
}

// A name the zone database cannot load is refused at the prompt rather than
// leaving the player silently on UTC.
func TestTypedZoneIsChecked(t *testing.T) {
	w := newWorld()
	w.Plain = true
	f := &fakeSession{keys: []rune("3\r7\rMars/Olympus\r\r")}
	pickTimeZone(f, w)
	if got := w.Player().TimeZone; got != "" {
		t.Errorf("a bad zone was stored: %q", got)
	}
	// Asserted on the output too: an empty TimeZone is also what a script that
	// never reached the prompt would leave behind.
	if out := f.out.String(); !strings.Contains(out, "is not a zone this game knows") {
		t.Errorf("the typed zone was not refused:\n%s", out)
	}
}
