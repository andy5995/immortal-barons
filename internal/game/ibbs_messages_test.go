package game

import (
	"strings"
	"testing"
	"time"
)

// ipWorld is a board with two human barons on it.
func ipWorld(board string) *World {
	w := NewWorldSeed(Config{BoardID: board}, 1)
	w.LastMaintDate = "2026-08-06"
	for _, name := range []string{"Iron Dominion", "Sky Realm"} {
		e := &Empire{Name: name, Owner: strings.ToLower(name), Alive: true}
		w.Empires = append(w.Empires, e)
	}
	return w
}

// TestIPMessageReachesEveryBaron is the whole point of the channel: a message
// aimed at a planet is read by everyone on it, not by one baron.
func TestIPMessageReachesEveryBaron(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")

	here.SendIPMessage(here.Empires[0], []string{"The Eclipse"}, false, "Stand down or we come for you.")
	if len(here.Outbox) != 1 || len(here.Outbox[0].IPMessages) != 1 {
		t.Fatalf("message not queued: %+v", here.Outbox)
	}
	there.ApplyPacket(here.Outbox[0])
	for _, e := range there.Empires {
		if len(e.Mail) != 1 {
			t.Fatalf("%s has %d messages, want 1", e.Name, len(e.Mail))
		}
		m := e.Mail[0]
		if m.Body != "Stand down or we come for you." {
			t.Errorf("body is %q", m.Body)
		}
		// The realm and its planet are separate fields, as BRE's interplanetary
		// reader keeps them ("Message From: <realm> on <planet>"): the reply needs
		// the realm to address the author, and the planet to route the packet.
		if m.From != "Iron Dominion" {
			t.Errorf("sender is %q, want the realm on its own", m.From)
		}
		if m.FromBoard != "Nova Hub" {
			t.Errorf("FromBoard is %q, so a reply could not find its way home", m.FromBoard)
		}
	}
}

// TestIPMessageToCoordinatorIsPrivate checks the one narrower address: only the
// elected Coordinator reads it.
func TestIPMessageToCoordinatorIsPrivate(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")
	co := there.Empires[1]
	there.VoteCoordinator(there.Empires[0], co.Owner)
	there.VoteCoordinator(co, co.Owner)

	here.SendIPMessage(here.Empires[0], []string{"The Eclipse"}, true, "A word between coordinators.")
	there.ApplyPacket(here.Outbox[0])
	if len(co.Mail) != 1 {
		t.Fatalf("the Coordinator has %d messages, want 1", len(co.Mail))
	}
	if len(there.Empires[0].Mail) != 0 {
		t.Errorf("a baron who is not the Coordinator read it: %+v", there.Empires[0].Mail)
	}
}

// TestIPMessageToSeveralPlanets checks one composition is queued once per named
// planet, each in that planet's own packet.
func TestIPMessageToSeveralPlanets(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	w := ipWorld("Nova Hub")
	w.SendIPMessage(w.Empires[0], []string{"The Eclipse", "Starship Junkyard"}, false, "Terms.")
	if len(w.Outbox) != 2 {
		t.Fatalf("queued %d packets, want one per planet", len(w.Outbox))
	}
	for _, p := range w.Outbox {
		if len(p.IPMessages) != 1 || p.ToBoard == "" {
			t.Errorf("packet %+v is not one message addressed to one planet", p)
		}
	}
}

// TestIPMessageReplyGoesBack checks a reply leaves for the sender's planet
// rather than being lost looking for a realm that is not on this one, and that
// BRE's "Public Reply?" decides who reads it there.
func TestIPMessageReplyGoesBack(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	w := ipWorld("The Eclipse")
	w.ReplyIPMessage(w.Empires[0], "Nova Hub", "Iron Dominion", "We accept.", false)
	if len(w.Outbox) != 1 || w.Outbox[0].ToBoard != "Nova Hub" {
		t.Fatalf("reply queued as %+v", w.Outbox)
	}
	m := w.Outbox[0].IPMessages[0]
	if m.Body != "We accept." {
		t.Errorf("reply body %q", m.Body)
	}
	if m.ToEmpire != "Iron Dominion" {
		t.Errorf("a private reply went to %q, want only its author", m.ToEmpire)
	}

	w.Outbox = nil
	w.ReplyIPMessage(w.Empires[0], "Nova Hub", "Iron Dominion", "So does the planet.", true)
	if got := w.Outbox[0].IPMessages[0].ToEmpire; got != "" {
		t.Errorf("a public reply was narrowed to %q", got)
	}
}

// TestIPReplyToAuthorReachesOnlyThem is the receiving half: a private reply
// lands with the baron who wrote, and nobody else on their planet sees it.
func TestIPReplyToAuthorReachesOnlyThem(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("The Eclipse")
	there := ipWorld("Nova Hub")
	author := there.Empires[0]
	here.ReplyIPMessage(here.Empires[0], "Nova Hub", author.Name, "Between us.", false)
	there.ApplyPacket(here.Outbox[0])
	if len(author.Mail) != 1 {
		t.Fatalf("the author has %d messages, want 1", len(author.Mail))
	}
	if n := len(there.Empires[1].Mail); n != 0 {
		t.Errorf("a baron who was not written to read it (%d messages)", n)
	}
}

// TestIPMessageToOwnPlanetIsDeliveredLocally guards the one address that has no
// packet to ride: BRE's planet list includes the board you are calling from, and
// a message queued for it would leave on a transport with nowhere to take it.
func TestIPMessageToOwnPlanetIsDeliveredLocally(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	w := ipWorld("Nova Hub")
	w.SendIPMessage(w.Empires[0], []string{"Nova Hub"}, false, "A word to my own.")
	if len(w.Outbox) != 0 {
		t.Errorf("a message home was queued for the transport: %+v", w.Outbox)
	}
	for _, e := range w.Empires {
		if len(e.Mail) != 1 {
			t.Errorf("%s has %d messages, want 1", e.Name, len(e.Mail))
		}
	}
}

// A message arriving from another planet is mail, and mail only: it reaches the
// mailboxes and posts nothing to the planet news (#146). BRE's own inbound
// handler writes DATA\\MSG.BRF and touches no news file.
func TestArrivingIPMessagesStayOutOfTheNews(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")

	here.SendIPMessage(here.Empires[0], []string{"The Eclipse"}, false, "Stand down.")
	before := len(there.NewsToday)
	there.ApplyPacket(here.Outbox[0])

	if len(there.Empires[0].Mail) != 1 {
		t.Fatalf("the message should still be delivered, got %d", len(there.Empires[0].Mail))
	}
	if got := there.NewsToday[before:]; len(got) != 0 {
		t.Errorf("an arriving message reached the news: %q", got)
	}
}

// TestIPMessageToNamedBaronsReachesOnlyThem covers Send Message -> Single
// Planet once the sender has picked letters at the "(A-Y,Z=All,?=List) Send
// to:" prompt: one packet message per named realm, each narrowed with ToEmpire,
// and nobody else on that planet reads it.
func TestIPMessageToNamedBaronsReachesOnlyThem(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")
	// A third realm on the far planet, so "only them" has something to exclude.
	there.Empires = append(there.Empires, &Empire{Name: "Gap Origix", Owner: "gap", Alive: true})

	here.SendIPMessageToBarons(here.Empires[0], "The Eclipse",
		[]string{"Iron Dominion", "Gap Origix"}, "A word for you two.")
	if len(here.Outbox) != 1 {
		t.Fatalf("queued %d packets, want one for The Eclipse: %+v", len(here.Outbox), here.Outbox)
	}
	msgs := here.Outbox[0].IPMessages
	if len(msgs) != 2 {
		t.Fatalf("queued %d messages, want one per named baron", len(msgs))
	}
	var to []string
	for _, m := range msgs {
		to = append(to, m.ToEmpire)
	}
	if to[0] != "Iron Dominion" || to[1] != "Gap Origix" {
		t.Fatalf("ToEmpire = %v, want the two named realms — an unaddressed message goes planet-wide", to)
	}

	there.ApplyPacket(here.Outbox[0])
	for _, e := range there.Empires {
		want := 0
		if e.Name == "Iron Dominion" || e.Name == "Gap Origix" {
			want = 1
		}
		if len(e.Mail) != want {
			t.Errorf("%s has %d messages, want %d", e.Name, len(e.Mail), want)
		}
	}
}

// An interplanetary message addressed to several barons shows every recipient
// the WHOLE address, as BRE does: `Message To  : ABCDE` (cap/eots-ibbs-02.cap).
// Each copy travels separately, so without the list riding the packet a reader
// sees only its own letter and cannot tell whether the message went to it alone.
func TestIPMessageToBaronsShowsTheWholeAddress(t *testing.T) {
	send := NewWorldSeed(DefaultConfig(), 1)
	send.Config.BoardID = "Alpha BBS"
	from := send.AddHuman("s", "Sender")

	recv := NewWorldSeed(DefaultConfig(), 1)
	recv.Config.BoardID = "Bravo BBS"
	a := recv.AddHuman("a", "Anvil")
	b := recv.AddHuman("b", "Bastion")
	c := recv.AddHuman("c", "Crag")

	send.SendIPMessageToBarons(from, "Bravo BBS", []string{"Anvil", "Bastion", "Crag"}, "all of you")

	var delivered int
	for _, p := range send.Outbox {
		for _, m := range p.IPMessages {
			recv.deliverIPMessage(m)
			delivered++
		}
	}
	if delivered != 3 {
		t.Fatalf("expected one copy per baron, got %d", delivered)
	}

	want := recv.EmpireLetter(a) + recv.EmpireLetter(b) + recv.EmpireLetter(c)
	for _, e := range []*Empire{a, b, c} {
		if len(e.Mail) != 1 {
			t.Fatalf("%s got %d messages, want 1", e.Name, len(e.Mail))
		}
		if got := e.Mail[0].To; got != want {
			t.Errorf("%s sees Message To %q, want the whole address %q", e.Name, got, want)
		}
	}
}

// A realm the sender addressed but this board no longer has is dropped from the
// rendered address rather than leaving a letter for a realm that is not there.
func TestIPMessageAddressSkipsRealmsThisBoardDoesNotHave(t *testing.T) {
	send := NewWorldSeed(DefaultConfig(), 1)
	send.Config.BoardID = "Alpha BBS"
	from := send.AddHuman("s", "Sender")

	recv := NewWorldSeed(DefaultConfig(), 1)
	recv.Config.BoardID = "Bravo BBS"
	a := recv.AddHuman("a", "Anvil")

	send.SendIPMessageToBarons(from, "Bravo BBS", []string{"Anvil", "Ghost"}, "hello")
	for _, p := range send.Outbox {
		for _, m := range p.IPMessages {
			recv.deliverIPMessage(m)
		}
	}
	if len(a.Mail) != 1 {
		t.Fatalf("Anvil got %d messages, want 1", len(a.Mail))
	}
	if got, want := a.Mail[0].To, recv.EmpireLetter(a); got != want {
		t.Errorf("Message To %q, want %q — a realm this board lacks must not appear", got, want)
	}
}

// A planet-wide message is addressed to everyone, and the receiving board
// renders that from its own roster rather than from anything on the wire.
func TestIPMessageToAllShowsEveryLetter(t *testing.T) {
	send := NewWorldSeed(DefaultConfig(), 1)
	send.Config.BoardID = "Alpha BBS"
	from := send.AddHuman("s", "Sender")

	recv := NewWorldSeed(DefaultConfig(), 1)
	recv.Config.BoardID = "Bravo BBS"
	a := recv.AddHuman("a", "Anvil")
	b := recv.AddHuman("b", "Bastion")

	send.SendIPMessage(from, []string{"Bravo BBS"}, false, "everyone")
	for _, p := range send.Outbox {
		for _, m := range p.IPMessages {
			recv.deliverIPMessage(m)
		}
	}
	want := recv.EmpireLetter(a) + recv.EmpireLetter(b)
	for _, e := range []*Empire{a, b} {
		if len(e.Mail) != 1 || e.Mail[0].To != want {
			t.Errorf("%s sees Message To %q, want %q", e.Name, e.Mail[0].To, want)
		}
	}
}

// A message addressed to a realm that is not (or no longer) on the target
// planet used to vanish with no trace (Andy's report, #146's silent-failure
// half). It now bounces back as an ordinary IP message — no new packet field —
// carrying why, the original date, the address, and the body.
func TestIPMessageToUnknownRealmBouncesBack(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")

	here.SendIPMessageToBarons(here.Empires[0], "The Eclipse", []string{"Ghost Realm"}, "Where are you?")
	if len(here.Outbox) != 1 || len(here.Outbox[0].IPMessages) != 1 {
		t.Fatalf("message not queued: %+v", here.Outbox)
	}
	there.ApplyPacket(here.Outbox[0])

	if len(there.Outbox) != 1 || len(there.Outbox[0].IPMessages) != 1 {
		t.Fatalf("no bounce was queued: %+v", there.Outbox)
	}
	bounce := there.Outbox[0].IPMessages[0]
	if bounce.FromEmpire != "" {
		t.Errorf("a bounce speaks for the planet, not a baron: FromEmpire = %q", bounce.FromEmpire)
	}
	if bounce.ToEmpire != "Iron Dominion" {
		t.Errorf("bounce ToEmpire = %q, want the original sender", bounce.ToEmpire)
	}

	here.ApplyPacket(there.Outbox[0])
	sender := here.Empires[0]
	if len(sender.Mail) != 1 {
		t.Fatalf("the sender should be told; Mail len = %d, want 1", len(sender.Mail))
	}
	got := sender.Mail[0]
	if got.From != "" || got.FromBoard != "The Eclipse" {
		t.Errorf("bounce mail From=%q FromBoard=%q, want empty From and the target board", got.From, got.FromBoard)
	}
	if !strings.Contains(got.Body, "Ghost Realm of The Eclipse could not be delivered") ||
		!strings.Contains(got.Body, "no such realm there") {
		t.Errorf("bounce body should say why: %q", got.Body)
	}
	if !strings.Contains(got.Body, "To: Ghost Realm.") {
		t.Errorf("bounce body should show the address: %q", got.Body)
	}
	if !strings.Contains(got.Body, "Where are you?") {
		t.Errorf("bounce body should quote the original message: %q", got.Body)
	}
}

// The Coordinator failure is a different fact from a realm not found — no
// office elected yet, rather than a wrong or dead name — and gets its own
// wording so the sender knows to wait or address the planet instead.
func TestIPMessageToUnelectedCoordinatorBouncesBack(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse") // nobody has voted, so there is no Coordinator

	here.SendIPMessage(here.Empires[0], []string{"The Eclipse"}, true, "Coordinator, respond.")
	there.ApplyPacket(here.Outbox[0])
	if len(there.Outbox) != 1 || len(there.Outbox[0].IPMessages) != 1 {
		t.Fatalf("no bounce was queued: %+v", there.Outbox)
	}

	here.ApplyPacket(there.Outbox[0])
	sender := here.Empires[0]
	if len(sender.Mail) != 1 {
		t.Fatalf("the sender should be told; Mail len = %d, want 1", len(sender.Mail))
	}
	if !strings.Contains(sender.Mail[0].Body, "No Coordinator has been elected") {
		t.Errorf("bounce body should say why: %q", sender.Mail[0].Body)
	}
}

// The loop guard: a bounce is itself an IPMessage with no FromEmpire, so if it
// ALSO cannot be delivered — the original sender has since died too — nothing
// sends a second bounce chasing it. Two boards that can no longer reach each
// other's barons must not bounce a failure back and forth forever.
func TestIPMessageBounceDoesNotLoopWhenSenderIsAlsoGone(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")

	here.SendIPMessageToBarons(here.Empires[0], "The Eclipse", []string{"Ghost Realm"}, "Where are you?")
	there.ApplyPacket(here.Outbox[0])
	if len(there.Outbox) != 1 {
		t.Fatalf("no bounce was queued: %+v", there.Outbox)
	}

	// The original sender is gone too by the time the bounce comes home. here's
	// own Outbox already holds the original outbound message (nothing drains it
	// here — that is the transport's job), so the check is that applying the
	// bounce adds nothing further to it, not that it stays empty.
	here.Empires[0].Alive = false
	beforePackets, beforeMsgs := len(here.Outbox), len(here.Outbox[0].IPMessages)

	here.ApplyPacket(there.Outbox[0])
	if len(here.Outbox) != beforePackets || len(here.Outbox[0].IPMessages) != beforeMsgs {
		t.Fatalf("a bounce that cannot itself be delivered must die quietly, not bounce again: %+v", here.Outbox)
	}
}

// A message to your OWN planet is delivered without a packet at all (see
// sendIP), so a failure on it bounces back at once, in the same call, with no
// round trip.
func TestIPMessageToOwnUnknownRealmBouncesBackLocally(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")

	here.SendIPMessageToBarons(here.Empires[0], "Nova Hub", []string{"Ghost Realm"}, "hello?")

	sender := here.Empires[0]
	if len(sender.Mail) != 1 {
		t.Fatalf("a message to your own unreachable planet should bounce back at once; Mail len = %d, want 1", len(sender.Mail))
	}
	if !strings.Contains(sender.Mail[0].Body, "Ghost Realm") {
		t.Errorf("bounce body should name the realm: %q", sender.Mail[0].Body)
	}
}
