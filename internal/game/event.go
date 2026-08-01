package game

import (
	"encoding/json"
	"time"
)

// Event is one line of a realm's "since your last play" recap, with the moment
// it happened. BRE stamps every entry with a real date and time — asynchronous
// play is the point of the log, so when a thing happened is part of the news.
type Event struct {
	When time.Time
	Text string
}

// UnmarshalJSON accepts a bare string as well as an object, so a world saved
// before events carried a timestamp still loads. Such an event keeps a zero
// When, which the recap renders without a stamp.
func (ev *Event) UnmarshalJSON(b []byte) error {
	var text string
	if err := json.Unmarshal(b, &text); err == nil {
		ev.When, ev.Text = time.Time{}, text
		return nil
	}
	type plain Event // avoid recursing into this method
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*ev = Event(p)
	return nil
}

// addEvent files a line on this realm's recap, stamped now.
func (e *Empire) addEvent(text string) {
	e.Events = append(e.Events, Event{When: time.Now(), Text: text})
}
