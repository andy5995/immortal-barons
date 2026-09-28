package game

import (
	"encoding/binary"
	"fmt"
	"testing"
)

// seqKey is packetKey's form for a numbered packet, written out by hand so a
// change to packetKey shows up here as a failure rather than following along.
func seqKey(board string, seq uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], seq)
	return fmt.Sprintf("%s#%x", board, b)
}

// A numbered packet is refused on replay by HighSeq alone, with no SeenPackets
// entry recorded for it at all.
func TestANumberedReplayIsRefusedWithoutASeenEntry(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	p := Packet{FromBoard: "Far BBS", Seq: 7, Date: "2026-09-28"}

	if w.SeenPacket(p) {
		t.Fatal("a first packet was reported seen")
	}
	if len(w.SeenPackets) != 0 {
		t.Errorf("a numbered packet was stored in SeenPackets: %v", w.SeenPackets)
	}
	if w.HighSeq["Far BBS"] != 7 {
		t.Errorf("HighSeq = %v, want Far BBS at 7", w.HighSeq)
	}
	if !w.IsPacketSeen(p) || !w.SeenPacket(p) {
		t.Error("the same packet dropped in again was not refused")
	}
	older := Packet{FromBoard: "Far BBS", Seq: 5, Date: "2026-09-27"}
	if !w.IsPacketSeen(older) || !w.SeenPacket(older) {
		t.Error("an older number from the same board was not refused")
	}
	if w.SeenPacket(Packet{FromBoard: "Far BBS", Seq: 8}) {
		t.Error("the next number was refused")
	}
	if len(w.SeenPackets) != 0 {
		t.Errorf("SeenPackets grew: %v", w.SeenPackets)
	}
}

// Packets HighSeq cannot track, with no number or no board name, are still
// fingerprinted and refused on replay.
func TestUntrackablePacketsAreStillFingerprinted(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	for _, p := range []Packet{
		{FromBoard: "Old BBS", Date: "2026-09-28"}, // an older board: no number
		{Seq: 3, Date: "2026-09-28"},               // no board name to key HighSeq by
	} {
		if w.SeenPacket(p) {
			t.Fatalf("%+v: a first packet was reported seen", p)
		}
		if !w.SeenPackets[packetKey(p)] {
			t.Errorf("%+v was not recorded in SeenPackets", p)
		}
		if !w.IsPacketSeen(p) || !w.SeenPacket(p) {
			t.Errorf("%+v was not refused on replay", p)
		}
	}
	if len(w.HighSeq) != 0 {
		t.Errorf("HighSeq tracked a packet it cannot: %v", w.HighSeq)
	}
}

// The load-time prune removes only the entries HighSeq covers.
func TestPruneSeenPacketsKeepsWhatHighSeqCannotCover(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.HighSeq = map[string]uint64{"Far BBS": 10, "Odd#Name": 4}
	hash := packetKey(Packet{FromBoard: "Far BBS", Date: "2026-09-28"})
	keep := []string{
		seqKey("Far BBS", 11),  // above HighSeq: not covered
		seqKey("", 2),          // no board: HighSeq has nothing to cover it with
		seqKey("New BBS", 1),   // a board with no HighSeq entry
		seqKey("Odd#Name", 5),  // a "#" in the name: split at the last one
		hash,                   // a content fingerprint
		"Far BBS#not-hex-here", // not a key packetKey writes
		"no separator",
	}
	drop := []string{
		seqKey("Far BBS", 1),
		seqKey("Far BBS", 10),
		seqKey("Odd#Name", 4),
	}
	w.SeenPackets = map[string]bool{}
	for _, k := range append(append([]string{}, keep...), drop...) {
		w.SeenPackets[k] = true
	}

	if n := w.PruneSeenPackets(); n != len(drop) {
		t.Errorf("pruned %d entries, want %d", n, len(drop))
	}
	for _, k := range keep {
		if !w.SeenPackets[k] {
			t.Errorf("kept entry %q was pruned", k)
		}
	}
	for _, k := range drop {
		if w.SeenPackets[k] {
			t.Errorf("covered entry %q was kept", k)
		}
	}

	var empty World
	if n := empty.PruneSeenPackets(); n != 0 {
		t.Errorf("a world with no maps pruned %d", n)
	}
}
