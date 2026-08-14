package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestSendMessageVanishedRecipientConflict proves Send Message re-finds its
// recipient by realm name inside the transaction. Node B picks Victimville,
// and while B composes the message another node eliminates Victimville. Because
// encoding/json reuses *Empire pointers by slice INDEX, B's reload rebinds
// Victimville's old slot to Decoyland — so mailing through a cached pointer
// would deliver to the WRONG inbox. Re-finding by name sees Victimville is gone
// and aborts, leaving Decoyland's inbox empty.
func TestSendMessageVanishedRecipientConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, nil)
	commitOnFile(t, cfg, func(w *game.World) { w.AddHuman("victim", "Victimville") })
	commitOnFile(t, cfg, func(w *game.World) { w.AddHuman("decoy", "Decoyland") })

	fb := &hookSession{
		// (B) Victimville — the picker letters realms by world slot and Alethia
		// holds (A) — RETURN closes the recipient list, type "hi", save, no more
		fakeSession: fakeSession{keys: []rune("B\rhi\r/Sn")},
		marker:      "lines for your message",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("victim")) })
		},
	}
	sendMessage(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); len(d.Mail) != 0 {
		t.Fatalf("Decoyland inbox = %v, want empty — a stale pointer mailed the wrong realm", d.Mail)
	}
	if out := fb.out.String(); !strings.Contains(out, "no longer there") {
		t.Fatalf("node B should have aborted with the target-gone notice, got: %q", out)
	}
}

// TestConcurrentMailBothLand proves a send appends to the recipient's inbox
// against fresh on-disk state, so a message committed by another node while B
// composes is not clobbered — both messages land. This is the FileStore
// reload-before-append guarantee the concurrent mail path relies on.
func TestConcurrentMailBothLand(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, nil)
	commitOnFile(t, cfg, func(w *game.World) { w.AddHuman("victim", "Victimville") })

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("B\rhi\r/Sn")}, // pick (B) Victimville, close the list, type "hi", save, no more
		marker:      "lines for your message",
		hook: func() { // another node drops a message into the same inbox first
			commitOnFile(t, cfg, func(w *game.World) {
				v := w.FindByOwner("victim")
				v.Mail = append(v.Mail, game.Message{From: "Other", Body: "from another node"})
			})
		},
	}
	sendMessage(fb, b)

	v := committedEmpire(t, cfg, "victim")
	var gotOther, gotMine bool
	for _, m := range v.Mail {
		if strings.Contains(m.Body, "from another node") {
			gotOther = true
		}
		if m.From == "Alethia" && strings.Contains(m.Body, "hi") {
			gotMine = true
		}
	}
	if !gotOther || !gotMine {
		t.Fatalf("inbox = %v, want both the other node's message and Alethia's — a message was lost", v.Mail)
	}
}
