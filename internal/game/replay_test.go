package game

import (
	"strings"
	"testing"
)

// A numbered packet is refused on replay by HighSeq: the same number again, or
// an older one from the same board.
func TestANumberedReplayIsRefused(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	p := Packet{FromBoard: "Far BBS", Seq: 7, Date: "2026-09-28"}

	if w.SeenPacket(p) {
		t.Fatal("a first packet was reported seen")
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
}

// A packet with no number or no board name cannot be checked for a replay, so
// it is refused outright, and the sysop is told.
func TestAnUntrackablePacketIsRefused(t *testing.T) {
	for _, p := range []Packet{
		{FromBoard: "Old BBS", Date: "2026-09-28", Notice: "x"}, // no number
		{Seq: 3, Date: "2026-09-28", Notice: "x"},               // no board name
	} {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.ApplyPacket(p)
		if len(w.SysopNotices) != 1 || !strings.Contains(w.SysopNotices[0], "no sequence number") {
			t.Errorf("%+v: sysop notices %q, want the refusal alone", p, w.SysopNotices)
		}
		if len(w.HighSeq) != 0 {
			t.Errorf("%+v: HighSeq recorded a refused packet: %v", p, w.HighSeq)
		}
	}
}

// receive applies p as the next packet its board sent here. A board numbers its
// packets as they are written out (StampOutbox), so one handed straight from a
// test world's Outbox, or a result returned by ApplyPacket, has no number yet
// and ApplyPacket would refuse it.
func (w *World) receive(p Packet) Packet {
	if p.Seq == 0 {
		p.Seq = w.HighSeq[p.FromBoard] + 1
	}
	return w.ApplyPacket(p)
}
