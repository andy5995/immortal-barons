package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The Event Log shows the turn-start recap and consumes it, so Play does not
// repeat it; with nothing to show the item is dimmed.
func TestEventLogConsumesTheRecap(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Player().Events = []game.Event{{When: time.Now(), Text: "Pirates raided your coast."}}
	})
	var item *Item
	menus := BuildMenus()
	for i := range menus.Messages.Items {
		if menus.Messages.Items[i].Key == 'E' {
			item = &menus.Messages.Items[i]
		}
	}
	if item == nil {
		t.Fatal("no Event Log item on the Messages menu")
	}
	var dimmed bool
	w.Read(func() { dimmed = item.Dimmed(w) })
	if dimmed {
		t.Error("dimmed with an event waiting")
	}

	f := &fakeSession{keys: []rune(" ")}
	item.Do(f, w)
	if out := stripANSI(f.out.String()); !strings.Contains(out, "Since your last play") || !strings.Contains(out, "Pirates raided") {
		t.Fatalf("the recap was not shown:\n%s", out)
	}
	w.Read(func() { dimmed = item.Dimmed(w) })
	if !dimmed {
		t.Error("not dimmed once the events were read")
	}

	f = &fakeSession{keys: []rune(" ")}
	showTurnEvents(f, w)
	if out := stripANSI(f.out.String()); strings.Contains(out, "Pirates raided") {
		t.Errorf("Play repeated an event already read:\n%s", out)
	}
}
