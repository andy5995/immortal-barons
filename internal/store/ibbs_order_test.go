package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
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
		shuffleGroupOrder(keys, rand.Reader)
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

type alwaysFailReader struct{}

func (alwaysFailReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated randomness failure")
}

// TestShuffleGroupOrderLeavesKeysUntouchedOnSourceFailure is a fix from
// review: the first version of shuffleGroupOrder swapped elements of keys IN
// PLACE during the Fisher-Yates pass, so a source that failed partway through
// left keys partially shuffled -- neither this run's random order nor its
// original scan order, contradicting what the function documented. It now
// shuffles a scratch copy and only writes back once the whole pass succeeds,
// so keys must come back byte-for-byte identical to what went in.
func TestShuffleGroupOrderLeavesKeysUntouchedOnSourceFailure(t *testing.T) {
	original := []string{"n1", "n2", "n3", "n4", "n5"}
	keys := append([]string(nil), original...)
	shuffleGroupOrder(keys, alwaysFailReader{})
	for i := range keys {
		if keys[i] != original[i] {
			t.Fatalf("keys changed despite the randomness source failing: got %v, want %v (unchanged)", keys, original)
		}
	}
}

// TestReadInboundGroupsSameBoardTogetherRegardlessOfFromNode is a fix from
// review: originKey originally preferred FromNode when a packet carried one,
// but replay detection (IsPacketSeen/SeenPacket's HighSeq) keys on FromBoard
// alone. A board's own packets do not all carry the same FromNode within one
// batch -- an older or backlogged packet can predate the board's roster
// entry (FromNode 0) while a newer one from the same board carries it -- and
// splitting those into two groups let the group with the higher Seq apply
// first, marking the other group's lower Seq a false replay: a real packet
// silently dropped, not just misordered. Run enough trials that the old,
// shuffle-order-dependent failure (roughly half of them) would show up if
// the fix regressed.
func TestReadInboundGroupsSameBoardTogetherRegardlessOfFromNode(t *testing.T) {
	for trial := 0; trial < 40; trial++ {
		inbound := t.TempDir()
		writePacket(t, inbound, "1-old", game.Packet{FromBoard: "Origin BBS", FromNode: 0, Seq: 1})
		writePacket(t, inbound, "2-new", game.Packet{FromBoard: "Origin BBS", FromNode: 7, Seq: 2})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{{Number: 7, Name: "Origin BBS"}, {Number: 9, Name: "Receiver BBS"}}

		result, err := ReadInbound(w, inbound, false)
		if err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		if result.Applied != 2 {
			t.Fatalf("trial %d: want both packets applied from the same board, got applied=%d alreadySeen=%d: %+v",
				trial, result.Applied, result.AlreadySeen, result)
		}
	}
}

// TestQuarantinePacketAvoidsNameCollision is a fix from review:
// quarantinePacket originally moved straight to dataDir/bad/<basename>,
// which os.Rename silently overwrites on Unix (destroying the first
// quarantined copy) and fails outright on Windows (propagating an error that
// would abort the whole run -- exactly the failure quarantining exists to
// avoid). It now steps around a collision instead of hitting it.
func TestQuarantinePacketAvoidsNameCollision(t *testing.T) {
	dataDir := t.TempDir()
	first := filepath.Join(t.TempDir(), "dupe.brp")
	second := filepath.Join(t.TempDir(), "dupe.brp")
	if err := os.WriteFile(first, []byte("first corrupt file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second corrupt file"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := quarantinePacket(dataDir, first); err != nil {
		t.Fatalf("first quarantinePacket: %v", err)
	}
	if err := quarantinePacket(dataDir, second); err != nil {
		t.Fatalf("second quarantinePacket (same basename) should not fail: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, BadDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 distinct files in %s after a name collision, got %d: %+v", BadDir, len(entries), entries)
	}
	firstContent, err := os.ReadFile(filepath.Join(dataDir, BadDir, "dupe.brp"))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstContent) != "first corrupt file" {
		t.Errorf("the first quarantined file's content was overwritten: got %q", firstContent)
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
