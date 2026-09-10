package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A stamp is stored in UTC and read on whatever clock the reader is on, and it
// says which clock that is — the whole point of #267.
func TestStampReadsOnTheReadersClock(t *testing.T) {
	at := time.Date(2026, 9, 7, 17, 0, 11, 0, time.UTC)
	stored := StoredStamp(at)
	if stored != "09/07/2026  17:00:11 UTC" {
		t.Fatalf("StoredStamp = %q", stored)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("the zone database is not compiled in: %v", err)
	}
	if got := StampIn(stored, berlin); got != "09/07/2026  19:00:11 CEST" {
		t.Errorf("StampIn(Berlin) = %q, want 09/07/2026  19:00:11 CEST", got)
	}
	if got := StampIn(stored, time.UTC); got != stored {
		t.Errorf("StampIn(UTC) = %q, want %q", got, stored)
	}
}

// A stamp written before stamps carried a zone is on a clock nobody can name,
// so it is shown as it stands rather than converted into a different time.
func TestZonelessStampIsLeftAlone(t *testing.T) {
	const legacy = "09/07/2026  17:00:11"
	if _, ok := ParseStamp(legacy); ok {
		t.Errorf("a zone-less stamp must not parse")
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if got := StampIn(legacy, tokyo); got != legacy {
		t.Errorf("StampIn on a legacy stamp = %q, want it unchanged", got)
	}
}

// An unknown zone name falls back to UTC rather than to the host's clock: a
// board's own zone is the one answer that is wrong for most of a league.
func TestUnknownZoneIsUTC(t *testing.T) {
	if Zone("Mars/Olympus").String() != "UTC" {
		t.Errorf("an unknown zone should be UTC, got %s", Zone("Mars/Olympus"))
	}
	if Zone("").String() != "UTC" {
		t.Errorf("the default zone should be UTC")
	}
	if Zone(LocalZone) != time.Local {
		t.Errorf("LocalZone should be the host's own clock")
	}
	if ValidZone("Mars/Olympus") {
		t.Errorf("the picker must refuse a zone the game cannot load")
	}
}

// Every zone the picker offers has to be loadable, or a player choosing it is
// silently left on UTC.
func TestEveryOfferedZoneLoads(t *testing.T) {
	for _, r := range ZoneRegions {
		for _, z := range r.Zones {
			if !ValidZone(z) {
				t.Errorf("%s (%s) does not load", z, r.Name)
			}
		}
	}
}

// News is stamped as it is posted, and the time reads back on the reader's
// clock without the date, which the screen's own heading carries.
func TestNewsLinesAreStamped(t *testing.T) {
	w := &World{}
	restore := timeNow
	timeNow = func() time.Time { return time.Date(2026, 9, 7, 17, 0, 11, 0, time.UTC) }
	defer func() { timeNow = restore }()

	w.postNews("The people take to the streets.")
	if len(w.NewsToday) != 1 {
		t.Fatalf("news = %v", w.NewsToday)
	}
	if got := TimeOfDay(w.NewsToday[0].At, time.UTC); got != "17:00:11 UTC" {
		t.Errorf("TimeOfDay = %q, want 17:00:11 UTC", got)
	}
	if got := TimeOfDay("", time.UTC); got != "" {
		t.Errorf("an unstamped line must show no time, got %q", got)
	}
}

// A world saved before news lines carried a time still loads, keeping its text.
func TestLegacyNewsLoadsAsText(t *testing.T) {
	var f NewsFeed
	if err := json.Unmarshal([]byte(`["a riot","a raid"]`), &f); err != nil {
		t.Fatalf("legacy news does not load: %v", err)
	}
	if f.Join("|") != "a riot|a raid" {
		t.Errorf("legacy news = %+v", f)
	}
	if f[0].At != "" {
		t.Errorf("a legacy line must carry no time, got %q", f[0].At)
	}
	// And the round trip of the current form keeps both halves.
	b, err := json.Marshal(NewsFeed{{At: StoredStamp(time.Now()), Text: "a raid"}})
	if err != nil {
		t.Fatal(err)
	}
	var back NewsFeed
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].Text != "a raid" || !strings.HasSuffix(back[0].At, " UTC") {
		t.Errorf("round trip = %+v", back)
	}
}
