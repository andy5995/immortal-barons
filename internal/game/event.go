package game

import (
	"encoding/json"
	"time"
)

// Event is one line of a realm's "since your last play" recap, with the moment
// it happened. BRE stamps every entry with a real date and time — asynchronous
// play is the point of the log, so when a thing happened is part of the news.
//
// Msg is the event itself, put into words in the reader's language when the
// recap is drawn (#297). Text is the same event in English: what world.json
// shows a sysop reading it, and all an event filed before Msg existed has.
type Event struct {
	When time.Time
	Text string
	Msg  *Msg `json:",omitempty"`
}

// In is the event in lang: its Msg rendered there, or the English Text of an
// event filed before events carried one.
func (ev Event) In(lang string) string {
	if ev.Msg != nil {
		return ev.Msg.In(lang)
	}
	return ev.Text
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
func (e *Empire) addEvent(m Msg) {
	e.Events = append(e.Events, Event{When: timeNow(), Text: m.English(), Msg: &m})
}
