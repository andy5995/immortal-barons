package game

import (
	"encoding/json"
	"testing"
)

// A world saved before events carried a timestamp stores them as bare strings.
// Those saves must still load, with the text intact and no time.
func TestEventDecodesLegacyStringForm(t *testing.T) {
	var got []Event
	if err := json.Unmarshal([]byte(`["Enemy raided you!",{"When":"2026-07-31T09:07:07Z","Text":"Beta accepted."}]`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Text != "Enemy raided you!" || !got[0].When.IsZero() {
		t.Errorf("legacy event = %+v, want the text with no stamp", got[0])
	}
	if got[1].Text != "Beta accepted." || got[1].When.IsZero() {
		t.Errorf("current event = %+v, want text and stamp", got[1])
	}
}

func TestAddEventStampsNow(t *testing.T) {
	e := &Empire{}
	e.addEvent("A dragon attacked your regions.")
	if len(e.Events) != 1 || e.Events[0].Text != "A dragon attacked your regions." {
		t.Fatalf("events = %+v", e.Events)
	}
	if e.Events[0].When.IsZero() {
		t.Error("addEvent must stamp the event")
	}
}
