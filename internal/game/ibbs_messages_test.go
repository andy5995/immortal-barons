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
