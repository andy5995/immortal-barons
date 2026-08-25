package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestReadInboundAppliesCoordinatorPacketBeforeOthers is the fix for #178: a
// roster update from the Coordinator has to be adopted before anything else in
// the same batch is checked against the roster, not whenever its filename
// happens to sort. The Coordinator's file here is named to sort AFTER the
// packet that depends on it -- exactly the ordering that broke this before the
// fix, since directory order is filename order and the fix no longer follows
// it.
func TestReadInboundAppliesCoordinatorPacketBeforeOthers(t *testing.T) {
	inbound := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// The Coordinator broadcasts a roster that, for the first time, gives the
	// receiving board ("Newbie BBS") a node number.
	coordCfg := game.DefaultConfig()
	coordCfg.BoardID = "Coordinator BBS"
	coord := game.NewWorldSeed(coordCfg, 1)
	coord.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 7, Name: "Newbie BBS"},
	}
	coord.CoordKey = priv
	coord.ExportNodeList()
	coord.StampOutbox()
	if len(coord.Outbox) != 1 {
		t.Fatalf("Coordinator did not queue a roster broadcast: %+v", coord.Outbox)
	}
	writePacket(t, inbound, "z-roster-update", coord.Outbox[0])

	// A packet from a third board, addressed ONLY by node number -- no ToBoard
	// to fall back on. Until the roster above is adopted, the receiving board
	// has no node number of its own, so this cannot be recognized as addressed
	// here at all. Its filename sorts BEFORE the roster update's.
	writePacket(t, inbound, "a-gift-for-newbie", game.Packet{
		FromBoard: "Third BBS",
		ToNode:    7,
		Seq:       1,
	})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Newbie BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.CoordPub = pub

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if len(w.LeagueNodes) != 2 {
		t.Fatalf("roster was not adopted: %+v", w.LeagueNodes)
	}
	if result.Applied != 2 {
		t.Errorf("want both packets applied (the roster update first makes the second recognizable), got applied=%d meshCopy=%d refused=%d: %+v",
			result.Applied, result.MeshCopy, result.Refused, result)
	}
}

// TestReadInboundKeepsOneOriginsPacketsInSeqOrder locks in the half of #178
// that was already correct and must stay that way: grouping packets by origin
// must never reorder a single origin's own packets against each other, even
// when their filenames (built here to defeat plain alphabetical sort) say
// otherwise.
func TestReadInboundKeepsOneOriginsPacketsInSeqOrder(t *testing.T) {
	inbound := t.TempDir()
	// Filenames deliberately run opposite to Seq order.
	writePacket(t, inbound, "1-third", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 3, Notice: "third"})
	writePacket(t, inbound, "2-first", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 1, Notice: "first"})
	writePacket(t, inbound, "3-second", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 2, Notice: "second"})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{{Number: 5, Name: "Origin BBS"}, {Number: 9, Name: "Receiver BBS"}}

	// Notice carries no payload (see HasPayload), so nothing but SysopNotices
	// records the order ApplyPacket saw these in: a refusal notice per Notice
	// isn't posted for a plain Notice field, so record order a different way --
	// HighSeq only advances forward, and IsPacketSeen would reject the second
	// and third packets as replays if they were ever applied out of Seq order.
	if _, err := ReadInbound(w, inbound, false); err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if w.HighSeq["Origin BBS"] != 3 {
		t.Errorf("HighSeq[Origin BBS] = %d, want 3 (applied out of Seq order would have stalled below 3 or rejected a later one as a replay)",
			w.HighSeq["Origin BBS"])
	}
}

// TestShuffleGroupOrderVaries checks that shuffleGroupOrder is not a no-op
// dressed up as a shuffle: with enough elements and enough trials, seeing the
// exact same permutation every time would mean the randomness isn't reaching
// the result, not that it got unlucky (5! = 120 permutations; 25 identical
// trials in a row has probability on the order of 120^-24).
func TestShuffleGroupOrderVaries(t *testing.T) {
	first := []string{"n1", "n2", "n3", "n4", "n5"}
	same := true
	for i := 0; i < 25; i++ {
		keys := append([]string(nil), first...)
		shuffleGroupOrder(keys)
		for j := range keys {
			if keys[j] != first[j] {
				same = false
			}
		}
		if !same {
			break
		}
	}
	if same {
		t.Error("shuffleGroupOrder produced the same order on every trial; expected real variation")
	}
}

// TestReadInboundQuarantinesUnreadablePacket is concern #3 from #178: one
// corrupt file must not stop the rest of the batch, and must not be silently
// dropped either -- it goes to BadDir for a sysop to find, and every OTHER
// packet in the same run is still applied.
func TestReadInboundQuarantinesUnreadablePacket(t *testing.T) {
	inbound := t.TempDir()
	writePacket(t, inbound, "a-good-one", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 1})
	if err := os.WriteFile(filepath.Join(inbound, "b-corrupt.brp"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound should not abort the batch on a corrupt packet, got: %v", err)
	}
	if result.Applied != 1 {
		t.Errorf("the good packet should still be applied, got applied=%d", result.Applied)
	}
	if result.Quarantined != 1 {
		t.Errorf("the corrupt packet should be counted quarantined, got %d", result.Quarantined)
	}
	if _, err := os.Stat(filepath.Join(inbound, "b-corrupt.brp")); !os.IsNotExist(err) {
		t.Error("the corrupt packet should have been moved out of the inbound directory")
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, BadDir, "b-corrupt.brp")); err != nil {
		t.Errorf("the corrupt packet should be in %s: %v", BadDir, err)
	}
	if len(w.SysopNotices) == 0 {
		t.Error("a quarantined packet should leave a sysop notice explaining why")
	}
}
