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
		if len(w.Incoming) == 0 {
			t.Fatalf("GameDay %d: the weapon vanished — no record, and the planet was told nothing", day)
		}
		if got := w.Incoming[0].ArrivesDay - w.GameDay; got != 2 {
			t.Errorf("GameDay %d: arrival is %d days off on our count, want 2", day, got)
		}
		// It has not landed yet...
		w.ArriveAnnihilator()
		if w.Incoming[0].DaysLeft != 0 {
			t.Errorf("GameDay %d: landed two days early", day)
		}
		// ...and it lands when our own clock reaches it.
		w.GameDay += 2
		w.ArriveAnnihilator()
		if w.Incoming[0].DaysLeft == 0 {
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

// AGE ALONE no longer refuses a weapon, and this test asserted the opposite
// until 2026-09-13. What refuses one is knowing it has already been dealt with
// (AnnihilatorDone, above), because a board that was offline across the arrival
// is indistinguishable by age from a board being told about a weapon twice — and
// refusing both to catch the second is what made a weapon vanish. An old status
// from a board whose weapon this planet never finished with is a siege it
// missed, and it lands.
func TestAnOldStatusIsNotRefusedOnAgeAlone(t *testing.T) {
	w := incomingWorld(t, 40)
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(timeNow().Add(-30 * 24 * time.Hour)), Intact: 100,
	})
	if len(w.Incoming) != 1 {
		t.Fatalf("a weapon this planet never dealt with was refused on age: %+v", w.Incoming)
	}
	w.ArriveAnnihilator()
	if w.Incoming[0].DaysLeft == 0 {
		t.Error("it was recorded but never landed")
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
	if len(w.Incoming) == 0 {
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
	// The arrival is in the PAST, which is what a late copy looks like in real
	// play: the siege takes AnnihilatorSiegeDays, so by the time the record is
	// closed the instant is days old and age alone cannot be what refuses the
	// copy. Only the stamp can.
	st := &AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(-time.Hour)), ArrivesDay: 12, Intact: 100,
	}
	w.applyAnnihilatorStatus(st)
	if len(w.Incoming) == 0 {
		t.Fatal("the weapon never arrived on the books")
	}
	w.ArriveAnnihilator()
	if w.Incoming[0].DaysLeft == 0 {
		t.Fatal("the weapon never landed")
	}
	for i := 0; i < AnnihilatorSiegeDays; i++ {
		w.TickAnnihilator()
	}
	if len(w.Incoming) > 0 {
		t.Fatalf("the siege never ended after %d days", AnnihilatorSiegeDays)
	}

	// The builder's board announcing it once more, before it retired the record.
	w.applyAnnihilatorStatus(st)
	if len(w.Incoming) > 0 {
		t.Error("a late copy of a burned-out weapon raised a second siege")
	}

	// A DIFFERENT weapon from the same board is still heard.
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(20 * 24 * time.Hour)), ArrivesDay: 30, Intact: 100,
	})
	if len(w.Incoming) == 0 {
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
	if len(w.Incoming) == 0 || w.Incoming[0].DaysLeft == 0 {
		t.Fatal("the weapon never landed")
	}
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(now.Add(9 * 24 * time.Hour)), ArrivesDay: 40, Intact: 100,
	})
	if got := Recorded(w.Incoming[0].ArrivesAt); got != first {
		t.Errorf("the besieging weapon's arrival was rewritten to %s, want %s", got, first)
	}
}

// Two planets can besiege this one at once. Until 2026-09-13 the second board's
// weapon was folded into the first's record: its warning was never posted,
// because the launch news fires only on the transition to flying, and whether it
// landed at all came down to whether its next status happened to arrive before
// its arrival instant.
func TestTwoPlanetsCanAimAtThisOneAtOnce(t *testing.T) {
	w := incomingWorld(t, 10)
	now := timeNow()
	for _, from := range []string{"xbit", "The Eclipse"} {
		w.applyAnnihilatorStatus(&AnnihilatorStatus{
			FromBoard: from, Funded: true, Launched: true,
			ArrivesAt: Recorded(now.Add(2 * 24 * time.Hour)), ArrivesDay: 12, Intact: 100,
		})
	}
	if len(w.Incoming) != 2 {
		t.Fatalf("tracking %d weapons, want 2: %+v", len(w.Incoming), w.Incoming)
	}
	// Both planets were named to the barons, not just the first.
	news := w.NewsToday.Join("\n")
	for _, from := range []string{"xbit", "The Eclipse"} {
		if !strings.Contains(news, from) {
			t.Errorf("no warning names %s — that planet's weapon arrives unannounced:\n%s", from, news)
		}
	}
	// And both land.
	w.GameDay += 2
	w.ArriveAnnihilator()
	for _, d := range w.Incoming {
		if d.DaysLeft == 0 {
			t.Errorf("the weapon from %s never landed", d.Creator)
		}
	}
	// Shooting one down leaves the other besieging.
	e := &Empire{Name: "Defender", Alive: true, Jets: 400_000}
	e.Regions = RegionMix{Agricultural: 5000}
	e.syncLand()
	w.Empires = append(w.Empires, e)
	for i := 0; i < 20 && w.IncomingFrom("xbit") != nil; i++ {
		e.Jets = 400_000
		if _, _, err := w.InterceptAnnihilator(e, "xbit", 400_000); err != nil {
			t.Fatalf("InterceptAnnihilator: %v", err)
		}
	}
	if w.IncomingFrom("xbit") != nil {
		t.Error("the weapon from xbit survived twenty full sorties")
	}
	if w.IncomingFrom("The Eclipse") == nil {
		t.Error("shooting down one planet's weapon removed the other planet's too")
	}
}

// A save written while the planet tracked a single incoming weapon keeps it.
func TestAPreListSaveKeepsItsIncomingWeapon(t *testing.T) {
	w := incomingWorld(t, 10)
	w.IncomingOne = &Annihilator{Creator: "xbit", Launched: true, Intact: 100, DaysLeft: 3}
	w.EnsureIncoming()
	if len(w.Incoming) != 1 || w.Incoming[0].Creator != "xbit" {
		t.Fatalf("the weapon was lost in the migration: %+v", w.Incoming)
	}
	if w.IncomingOne != nil {
		t.Error("the legacy field was left set, so the next load would add it twice")
	}
	w.EnsureIncoming()
	if len(w.Incoming) != 1 {
		t.Errorf("a second migration pass duplicated the weapon: %+v", w.Incoming)
	}
}

// A board that was down across the arrival must find its planet under siege when
// it comes back. The original has no such problem — it sends one attack packet
// AT arrival, and the target applies it whenever it next reads inbound — so a
// status whose arrival has already passed lands retroactively rather than being
// refused as stale. Refusing it is how a weapon disappeared between two boards.
func TestAnOfflineTargetStillGetsTheSiegeWhenItComesBack(t *testing.T) {
	w := incomingWorld(t, 10)
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true,
		ArrivesAt: Recorded(timeNow().Add(-2 * 24 * time.Hour)), ArrivesDay: 12, Intact: 100,
	})
	if len(w.Incoming) == 0 {
		t.Fatal("the weapon was refused as stale — the planet it hit was told nothing")
	}
	if w.Incoming[0].ArrivesDay > w.GameDay {
		t.Errorf("arrival is day %d with the clock at %d: an instant in the past must not read as future",
			w.Incoming[0].ArrivesDay, w.GameDay)
	}
	w.ArriveAnnihilator()
	if w.Incoming[0].DaysLeft == 0 {
		t.Error("the weapon is on the books but never landed")
	}
}

// The age test still governs a status with no instant, where nothing else can
// tell a live weapon from one long gone.
func TestALegacyStatusWithNoInstantIsStillJudgedByAge(t *testing.T) {
	w := incomingWorld(t, 40)
	w.applyAnnihilatorStatus(&AnnihilatorStatus{
		FromBoard: "xbit", Funded: true, Launched: true, ArrivesDay: 12, Intact: 100,
	})
	if len(w.Incoming) > 0 {
		t.Errorf("a month-old legacy status started a fresh siege: %+v", w.Incoming)
	}
}
