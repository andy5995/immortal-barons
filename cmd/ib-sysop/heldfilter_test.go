package main

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

func heldOf(reasons ...store.HeldReason) []store.HeldPacket {
	var out []store.HeldPacket
	for _, r := range reasons {
		out = append(out, store.HeldPacket{File: string(r), Reason: r})
	}
	return out
}

func TestHeldReasonCountsKeepFirstSeenOrder(t *testing.T) {
	order, counts := heldReasonCounts(heldOf(store.HeldSignature, store.HeldProtocol, store.HeldSignature))
	if len(order) != 2 || order[0] != store.HeldSignature || order[1] != store.HeldProtocol {
		t.Errorf("order = %v", order)
	}
	if counts[store.HeldSignature] != 2 || counts[store.HeldProtocol] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

// Hide drops the selected reason; Show only keeps it alone; Show all clears
// both. Nothing is hidden before a reason is picked.
func TestHeldFilterHideAndShowOnly(t *testing.T) {
	held := heldOf(store.HeldProtocol, store.HeldSignature, store.HeldRules, store.HeldProtocol)
	order, _ := heldReasonCounts(held)
	var f heldFilter

	f.hide()
	f.only(order)
	if len(f.shown(held)) != 4 {
		t.Fatal("with no reason picked, Hide and Show only must change nothing")
	}

	f.selected = store.HeldProtocol
	f.hide()
	for _, h := range f.shown(held) {
		if h.Reason == store.HeldProtocol {
			t.Fatal("Hide left the selected reason in the table")
		}
	}
	if len(f.shown(held)) != 2 {
		t.Errorf("Hide kept %d rows, want 2", len(f.shown(held)))
	}

	f.selected = store.HeldSignature
	f.only(order)
	got := f.shown(held)
	if len(got) != 1 || got[0].Reason != store.HeldSignature {
		t.Errorf("Show only gave %v", got)
	}

	f.hidden = nil // Show all
	if len(f.shown(held)) != 4 {
		t.Error("Show all should bring every row back")
	}
}
