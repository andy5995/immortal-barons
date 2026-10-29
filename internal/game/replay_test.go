package game

import (
	"slices"
	"strings"
	"testing"
)

// A numbered packet is refused on replay: the same number again. An older number
// that was never applied is not a replay; it arrived behind a later packet, and
// refusing it lost it (#311).
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
	if w.IsPacketSeen(older) || w.SeenPacket(older) {
		t.Error("an older number never applied here was refused")
	}
	if !w.IsPacketSeen(older) || !w.SeenPacket(older) {
		t.Error("the older packet dropped in again was not refused")
	}
	if w.HighSeq["Far BBS"] != 7 {
		t.Errorf("an older packet moved HighSeq to %d, want 7", w.HighSeq["Far BBS"])
	}
	if w.SeenPacket(Packet{FromBoard: "Far BBS", Seq: 8}) {
		t.Error("the next number was refused")
	}
}

// The set of numbers applied is bounded: once the highest passes the window,
// everything that far behind it counts as seen and is dropped from the set.
func TestTheReplayWindowIsBounded(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	for _, n := range []uint64{1, 3, 5000, 5002} {
		w.SeenPacket(Packet{FromBoard: "Far BBS", Seq: n})
	}
	// 5002 - 4096 = 906: 1 and 3 are below the floor, 2 with them.
	if got := w.ReplayFloor["Far BBS"]; got != 906 {
		t.Errorf("ReplayFloor = %d, want 906", got)
	}
	if got := w.SeenSeqs["Far BBS"]; !slices.Equal(got, []uint64{5000, 5002}) {
		t.Errorf("SeenSeqs = %v, want [5000 5002]", got)
	}
	for _, c := range []struct {
		seq  uint64
		seen bool
	}{{2, true}, {906, true}, {907, false}, {5000, true}, {5001, false}, {5003, false}} {
		if got := w.IsPacketSeen(Packet{FromBoard: "Far BBS", Seq: c.seq}); got != c.seen {
			t.Errorf("IsPacketSeen(%d) = %v, want %v", c.seq, got, c.seen)
		}
	}
}

// A ledger written before the window existed holds only the highest number, so
// nothing at or below it can be told apart from a replay, and all of it stays
// refused. Numbers above it are tracked exactly.
func TestALedgerWithOnlyTheHighestKeepsRefusingBelowIt(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.HighSeq = map[string]uint64{"Far BBS": 40}
	if !w.IsPacketSeen(Packet{FromBoard: "Far BBS", Seq: 39}) {
		t.Error("a number below an old ledger's highest was accepted")
	}
	w.SeenPacket(Packet{FromBoard: "Far BBS", Seq: 45})
	if w.IsPacketSeen(Packet{FromBoard: "Far BBS", Seq: 43}) {
		t.Error("a number between the old highest and the new one was refused")
	}
	if !w.IsPacketSeen(Packet{FromBoard: "Far BBS", Seq: 40}) {
		t.Error("the old highest itself was accepted after the next packet")
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

// A packet applied after a later one from its board still delivers what it
// carries — here a trade deal — but its scores are older than the ones already
// imported, so they do not replace them (#311).
func TestALatePacketDeliversButKeepsTheNewerScores(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "here"
	w := NewWorldSeed(cfg, 1)
	to := w.AddHuman("recv", "Receiver")
	to.Protection = 0
	troopers := to.Troopers

	newer := Packet{FromBoard: "there", Seq: 9, Date: "2026-10-13",
		Scores: []RemoteScore{{Empire: "Sender", Land: 900}}}
	older := Packet{FromBoard: "there", Seq: 8, Date: "2026-10-12",
		Scores:     []RemoteScore{{Empire: "Sender", Land: 100}},
		TradeDeals: []IPTradeDeal{{ID: 1, FromBoard: "there", FromEmpire: "Sender", ToEmpire: "Receiver", Goods: TradeBasket{Troopers: 500}}}}
	w.ApplyPacket(newer)
	reply := w.ApplyPacket(older)

	if to.Troopers != troopers+500 {
		t.Errorf("the late trade deal left %d troopers, want %d", to.Troopers, troopers+500)
	}
	if len(reply.TradeReceipts) != 1 || !reply.TradeReceipts[0].Delivered {
		t.Errorf("no delivered receipt went back: %+v", reply.TradeReceipts)
	}
	found := false
	for _, b := range w.RemoteBoards {
		if b.BoardID == "there" {
			found = true
			if len(b.Scores) != 1 || b.Scores[0].Land != 900 {
				t.Errorf("the late packet's scores replaced the newer ones: %+v", b.Scores)
			}
		}
	}
	if !found {
		t.Error("no scores were imported from there at all")
	}
}

// A bounce records the rules its sender states, but a late one must not roll
// that back to the rules of an older run (#319).
func TestALateBounceKeepsTheNewerRuleset(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "here"
	w := NewWorldSeed(cfg, 1)
	stale := cfg
	stale.TurnsPerDay += 2
	notice := "your packets are being held here"

	w.ApplyPacket(Packet{FromBoard: "there", Seq: 9, Date: "2026-10-13",
		Notice: notice, Ruleset: cfg.RulesetFingerprint()})
	if got := w.BoardRuleset["there"]; got != cfg.RulesetFingerprint() {
		t.Fatalf("a bounce did not record its sender's rules: %q", got)
	}
	w.ApplyPacket(Packet{FromBoard: "there", Seq: 8, Date: "2026-10-12",
		Notice: notice, Ruleset: stale.RulesetFingerprint()})
	if got := w.BoardRuleset["there"]; got != cfg.RulesetFingerprint() {
		t.Errorf("a late bounce rolled the recorded rules back to %q", got)
	}
}

// A bounce shows its sender ran and its transport delivers, so it is heard from
// like any other accepted packet: two boards trading only bounces must not look
// quiet to each other.
func TestABounceRecordsItsSenderAsHeardFrom(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "here"
	w := NewWorldSeed(cfg, 1)

	w.ApplyPacket(Packet{FromBoard: "there", Seq: 1, Date: "2026-10-13",
		Notice: "your packets are being held here", Version: "9.9.9"})
	if w.LastPacketFrom["there"] == "" {
		t.Error("a bounce did not record its sender as heard from")
	}
	if got := w.BoardVersion["there"]; got != "9.9.9" {
		t.Errorf("a bounce recorded version %q, want 9.9.9", got)
	}
}

// A threat record is a one-off report, so a late packet still files it: the
// later packet that overtook it never repeated it.
func TestALatePacketStillFilesItsThreat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "here"
	w := NewWorldSeed(cfg, 1)
	w.ApplyPacket(Packet{FromBoard: "there", Seq: 9, News: []string{"later"}})
	w.ApplyPacket(Packet{FromBoard: "there", Seq: 8, Threats: []Threat{{FromBoard: "there", Kind: ThreatGooie, ID: 2}}})
	if len(w.Threats) != 1 || w.Threats[0].ID != 2 {
		t.Errorf("a late packet's threat was not filed: %+v", w.Threats)
	}
}
