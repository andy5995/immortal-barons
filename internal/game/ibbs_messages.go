package game

import "fmt"

// Interplanetary messages (BRE's "IP Messages"). A message is normally
// addressed to a PLANET: the original offers one planet, several picked
// planets, every planet, allied planets, or a planet's Coordinator, and the text
// lands in the mailbox of everyone on the receiving side. That is what a baron
// uses to make a demand the whole target planet will read.
//
// A REPLY is the one message that can be narrowed to a single baron: BRE's
// interplanetary reader asks "Public Reply?" and sends the answer either to the
// whole planet or to the author alone (strings at BRE.OVR 0x1F94C, a reader
// separate from the local one).

// IPMessage is one such message in transit. ToCoordinator narrows delivery to
// the receiving planet's elected Coordinator; ToEmpire narrows it to one named
// realm, which is how a non-public reply reaches the baron who wrote to you.
// With neither set it goes to the whole planet.
type IPMessage struct {
	FromBoard     string
	FromEmpire    string
	ToCoordinator bool
	ToEmpire      string
	When          string
	Body          string
}

// SendIPMessage queues body for delivery on each of boards. An empty body or an
// empty board list sends nothing.
func (w *World) SendIPMessage(from *Empire, boards []string, toCoordinator bool, body string) {
	w.sendIP(from, boards, IPMessage{ToCoordinator: toCoordinator, Body: body})
}

// sendIP queues one addressed message for each board. m carries the addressing
// and the text; the sender and the stamp are filled in here.
func (w *World) sendIP(from *Empire, boards []string, m IPMessage) {
	if m.Body == "" || len(boards) == 0 {
		return
	}
	m.FromBoard = w.Config.BoardID
	m.FromEmpire = from.Name
	m.When = timeNow().Format(StampFormat)
	for _, b := range boards {
		// BRE's planet list includes the board you are calling from, so a message
		// can be addressed home. Queueing that as a packet would send it out to a
		// transport that has nowhere to take it, so deliver it here and now.
		if b == w.Config.BoardID {
			w.deliverIPMessage(m)
			continue
		}
		p := w.outboxFor(b)
		p.IPMessages = append(p.IPMessages, m)
	}
}

// deliverIPMessage puts an arriving message in the mailboxes it is addressed to.
func (w *World) deliverIPMessage(m IPMessage) {
	from := fmt.Sprintf("%s of %s", m.FromEmpire, m.FromBoard)
	when := m.When
	if when == "" {
		when = timeNow().Format(StampFormat)
	}
	msg := Message{From: m.FromEmpire, FromBoard: m.FromBoard, When: when, Body: m.Body}
	if m.ToEmpire != "" {
		to := w.remoteTarget(m.ToEmpire)
		if to == nil {
			w.postNews(fmt.Sprintf("%s wrote to %s, who is no longer with us.", from, m.ToEmpire))
			return
		}
		msg.To = w.EmpireLetter(to)
		to.Mail = append(to.Mail, msg)
		return
	}
	if m.ToCoordinator {
		co := w.BBSCoordinator()
		if co == nil {
			w.postNews(fmt.Sprintf("A message from %s arrived for our Planet Coordinator, but we have elected none.", from))
			return
		}
		msg.To = w.EmpireLetter(co)
		co.Mail = append(co.Mail, msg)
		w.postNews(fmt.Sprintf("%s sent word to our Planet Coordinator.", from))
		return
	}
	// Every living realm, computer barons included — the same reach the local
	// "send to all" has. The news line goes up either way, so a planet whose
	// humans have all fallen still shows that someone out there wrote to it.
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		one := msg
		one.To = w.EmpireLetter(e)
		e.Mail = append(e.Mail, one)
	}
	w.postNews(fmt.Sprintf("%s addressed our whole planet.", from))
}

// ReplyIPMessage answers an interplanetary message. A public reply goes to the
// whole of the sender's planet; otherwise it reaches only the baron who wrote,
// which is the choice BRE's "Public Reply?" offers.
func (w *World) ReplyIPMessage(from *Empire, board, author, body string, public bool) {
	m := IPMessage{Body: body}
	if !public {
		m.ToEmpire = author
	}
	w.sendIP(from, []string{board}, m)
}
