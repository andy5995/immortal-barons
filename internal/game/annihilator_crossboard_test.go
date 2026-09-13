package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func incomingWorld(t *testing.T, gameDay int) *World {
	t.Helper()
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "unix bit"
	w := NewWorldSeed(cfg, 1)
	w.GameDay = gameDay
	return w
}

// A weapon aimed at us must land whatever OUR day counter happens to read.
// GameDay counts from each board's own first maintenance, so two boards' day
// numbers are unrelated; the status packet used to carry the sender's, and a
// target whose counter had run ahead dropped a live weapon with no record and no
// news — which is exactly how one vanished between two test boards.
func TestAnIncomingGooieLandsWhateverTheLocalDayCounterReads(t *testing.T) {
	for _, day := range []int{0, 12, 40, 400} {
		w := incomingWorld(t, day)
		now := timeNow()
		w.applyAnnihilatorStatus(&AnnihilatorStatus{
			FromBoard: "xbit", Funded: true, Launched: true,
			ArrivesAt: Recorded(now.Add(2 * 24 * time.Hour)), ArrivesDay: 12, Intact: 100,
		})
		if w.Incoming == nil {
			t.Fatalf("GameDay %d: the weapon vanished — no record, and the planet was told nothing", day)
		}
		if got := w.Incoming.ArrivesDay - w.GameDay; got != 2 {
			t.Errorf("GameDay %d: arrival is %d days off on our count, want 2", day, got)
		}
		// It has not landed yet...
		w.ArriveAnnihilator()
		if w.Incoming.DaysLeft != 0 {
			t.Errorf("GameDay %d: landed two days early", day)
		}
		// ...and it lands when our own clock reaches it.
		w.GameDay += 2
		w.ArriveAnnihilator()
		if w.Incoming.DaysLeft == 0 {
			t.Errorf("GameDay %d: the weapon never landed", day)
		}
	}
}

// The countdown the planet is told is measured on our clock too. It read
// "1,392 hours" when the sender's day number was taken for ours.
func TestTheIncomingGooieCountdownIsMeasuredHere(t *testing.T) {
	w := incomingWorld(t, 2)
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(timeNow().Add(3 * 24 * time.Hour)), ArrivesDay: 60, Intact: 100,
	})
	var line string
	for _, n := range w.NewsToday {
		if strings.Contains(n.Text, "Gooie Kablooie arrives") {
			line = n.Text
		}
	}
	if line == "" {
		t.Fatal("the planet was never warned")
	}
	if !strings.Contains(line, "72 hours") {
		t.Errorf("countdown reads %q, want 72 hours", line)
	}
}

// A weapon that has been and gone is still ignored — the builder's board
// re-announcing it must not raise a second siege. Judged on the real interval
// now, not on two unrelated counters.
func TestAStaleGooieStatusRaisesNoSiege(t *testing.T) {
	w := incomingWorld(t, 40)
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(timeNow().Add(-30 * 24 * time.Hour)), Intact: 100,
	})
	if w.Incoming != nil {
		t.Errorf("a month-old weapon started a fresh siege: %+v", w.Incoming)
	}
}

// Dismantling is one office holder spending everyone's money, so every baron on
// the planet is told in their own event log rather than left to notice a news
// line. Without this a scrapped weapon reads exactly like one that went missing
// in transit — which is how one actually did.
func TestDismantlingTheGooieTellsEveryBaron(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID, cfg.AICount = true, "xbit", 0
	w := NewWorldSeed(cfg, 1)
	co := w.AddHuman("co", "Coordinator's Realm")
	other := w.AddHuman("other", "Someone Else")
	w.ImportBoard(RemoteBoard{BoardID: "unix bit"})
	co.CoordinatorVote, other.CoordinatorVote = co.Owner, co.Owner
	if w.BBSCoordinator() != co {
		t.Fatalf("test setup: %q is not the BBS Coordinator", co.Name)
	}
	if err := w.StartAnnihilator(co, "unix bit"); err != nil {
		t.Fatalf("StartAnnihilator: %v", err)
	}
	before := len(other.Events)

	if err := w.DismantleAnnihilatorByCoordinator(co); err != nil {
		t.Fatalf("DismantleAnnihilatorByCoordinator: %v", err)
	}

	if len(other.Events) == before {
		t.Fatalf("a baron who did not order it was never told")
	}
	last := other.Events[len(other.Events)-1].Text
	for _, want := range []string{"dismantled", "Gooie Kablooie", "unix bit"} {
		if !strings.Contains(last, want) {
			t.Errorf("notice %q does not mention %q", last, want)
		}
	}
	if !strings.Contains(last, co.Name) {
		t.Errorf("notice %q does not say who ordered it", last)
	}
}

// A board that predates ArrivesAt must still VERIFY a packet carrying it.
// Signing covers the marshalled packet, so such a board drops the field on
// parse, re-marshals without it, and the signature fails — refusing the whole
// status rather than merely reading it without an instant. `omitempty` does not
// help: it protects every packet that does not carry the field, never the one
// that does. That is the Bulletins breakage of 1da5698, which stopped a
// six-board league's orders while looking like a wrong key.
func TestABoardThatPredatesTheArrivalInstantStillVerifiesThePacket(t *testing.T) {
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p := Packet{FromBoard: "xbit", Annihilator: &AnnihilatorStatus{
		FromBoard: "xbit", Launched: true, ArrivesAt: Recorded(timeNow()), ArrivesDay: 6, Intact: 100,
	}}
	msg, err := boardSigningBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	p.BoardSig = ed25519.Sign(sec, msg)
	if p.Annihilator.ArrivesAt == "" {
		t.Fatal("signing emptied the caller's own packet")
	}

	// What an older board is left holding: the field it does not know is gone.
	raw, _ := json.Marshal(p)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	ann, ok := m["Annihilator"].(map[string]any)
	if !ok {
		t.Fatal("no Annihilator in the marshalled packet")
	}
	if _, carried := ann["ArrivesAt"]; !carried {
		t.Fatal("the instant never reached the wire, so this proves nothing")
	}
	delete(ann, "ArrivesAt")
	trimmed, _ := json.Marshal(m)
	var older Packet
	if err := json.Unmarshal(trimmed, &older); err != nil {
		t.Fatal(err)
	}
	olderMsg, err := boardSigningBytes(older)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, olderMsg, p.BoardSig) {
		t.Error("an older board refuses the packet — the whole Gooie status is lost, not just its instant")
	}
}

// A weapon already in flight when its board upgrades has no arrival instant.
// Sending one anyway renders a zero time as "01/01/0001", which parses fine and
// reads as 106,751 days in the past — so the receiver drops it as stale, which
// is the very disappearance the instant was added to prevent.
func TestAWeaponWithNoInstantFallsBackInsteadOfVanishing(t *testing.T) {
	w := incomingWorld(t, 40)
	// What a board that launched under the older build exports.
	st := &AnnihilatorStatus{FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: arrivalStamp(time.Time{}), ArrivesDay: 42, Intact: 100}
	if st.ArrivesAt != "" {
		t.Fatalf("a zero instant was put on the wire as %q", st.ArrivesAt)
	}
	w.applyAnnihilatorStatus(st)
	if w.Incoming == nil {
		t.Fatal("the weapon vanished: a missing instant must fall back to the day number, not drop it")
	}
}

// Dismantle notices carry no day, so `omitempty` on ArrivesDay would drop the
// field — and a board on the older struct re-marshals it back in, breaking the
// signature and with it the WHOLE packet. omitempty is safe on a new field and
// unsafe on an existing one.
func TestTheDayFieldStaysOnTheWireSoOlderBoardsStillVerify(t *testing.T) {
	raw, err := json.Marshal(&AnnihilatorStatus{FromBoard: "xbit", Dismantled: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "ArrivesDay") {
		t.Errorf("ArrivesDay left the wire: %s", raw)
	}
}

// A weapon that has been and gone must not be raised again by a late copy of
// its own status. IB re-announces a weapon in flight on every planetary run so
// one lost packet cannot leave the target unwarned, which means a copy can
// outlive the weapon; the arrival instant is what tells that copy from the first
// word of a new weapon.
func TestAFinishedGooieIsNotLandedTwiceByALateStatus(t *testing.T) {
	w := incomingWorld(t, 10)
	now := timeNow()
	st := &AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(2 * 24 * time.Hour)), ArrivesDay: 12, Intact: 100,
	}
	w.applyAnnihilatorStatus(st)
	if w.Incoming == nil {
		t.Fatal("the weapon never arrived on the books")
	}
	w.GameDay += 2
	w.ArriveAnnihilator()
	if w.Incoming.DaysLeft == 0 {
		t.Fatal("the weapon never landed")
	}
	for i := 0; i < AnnihilatorSiegeDays; i++ {
		w.TickAnnihilator()
	}
	if w.Incoming != nil {
		t.Fatalf("the siege never ended after %d days", AnnihilatorSiegeDays)
	}

	// The builder's board announcing it once more, before it retired the record.
	w.applyAnnihilatorStatus(st)
	if w.Incoming != nil {
		t.Error("a late copy of a burned-out weapon raised a second siege")
	}

	// A DIFFERENT weapon from the same board is still heard.
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(20 * 24 * time.Hour)), ArrivesDay: 30, Intact: 100,
	})
	if w.Incoming == nil {
		t.Error("the builder's next weapon was refused as though it were the old one")
	}
}

// A second generation's status routinely reports during the first weapon's
// siege — the builder is free to start another the day after the first lands.
// It must not rewrite the live weapon's arrival instant, or the stamp filed when
// that weapon ends records the wrong arrival and stops recognizing its own late
// copies.
func TestALiveGooieKeepsItsOwnArrivalInstant(t *testing.T) {
	w := incomingWorld(t, 10)
	now := timeNow()
	first := Recorded(now.Add(2 * 24 * time.Hour))
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: first, ArrivesDay: 12, Intact: 100,
	})
	w.GameDay += 2
	w.ArriveAnnihilator()
	if w.Incoming == nil || w.Incoming.DaysLeft == 0 {
		t.Fatal("the weapon never landed")
	}
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(9 * 24 * time.Hour)), ArrivesDay: 40, Intact: 100,
	})
	if got := Recorded(w.Incoming.ArrivesAt); got != first {
		t.Errorf("the besieging weapon's arrival was rewritten to %s, want %s", got, first)
	}
}
