package game

import "strings"

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
	// ToEmpires is the WHOLE address, on every copy, so a reader can see who else
	// the message went to. BRE prints the address as a run of realm letters —
	// `Message To  : ABCDE` (cap/eots-ibbs-02.cap) — and IB's local mail has done
	// the same since it was built; only the interplanetary path was showing the
	// reader its own letter alone, because it sends one message per recipient and
	// each copy knew only itself. Empty for a Coordinator message and for the
	// planet-wide case, which the receiving board renders from its own roster.
	ToEmpires []string `json:",omitempty"`
	When      string
	Body      string
}

// SendIPMessage queues body for delivery on each of boards. An empty body or an
// empty board list sends nothing.
func (w *World) SendIPMessage(from *Empire, boards []string, toCoordinator bool, body string) {
	w.sendIP(from, boards, IPMessage{ToCoordinator: toCoordinator, Body: body})
}

// SendIPMessageToBarons queues body for named realms on one board, one message
// each, narrowed with ToEmpire. That is how BRE's Single Planet mode addresses
// a message once the sender has picked letters at its `(A-Y,Z=All,?=List) Send
// to:` prompt: the address is a set of realms on the chosen planet, not the
// planet itself. N messages rather than a recipient list on one keeps the
// packet's shape unchanged — ToEmpire already exists for the author-only reply,
// and deliverIPMessage already honours it.
func (w *World) SendIPMessageToBarons(from *Empire, board string, toEmpires []string, body string) {
	addressed := make([]string, 0, len(toEmpires))
	for _, name := range toEmpires {
		if name != "" {
			addressed = append(addressed, name)
		}
	}
	for _, name := range addressed {
		w.sendIP(from, []string{board}, IPMessage{ToEmpire: name, ToEmpires: addressed, Body: body})
	}
}

// addressLetters renders an address as the run of realm letters BRE prints in
// "Message To  :", in this board's own empire order. Names that do not resolve
// here are dropped — a sender's roster can be stale — and an address that
// resolves to nothing falls back to the reader's own letter, so the header is
// never blank.
func (w *World) addressLetters(names []string, fallback *Empire) string {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var b strings.Builder
	for _, e := range w.Empires {
		if want[e.Name] {
			b.WriteString(w.EmpireLetter(e))
		}
	}
	if b.Len() == 0 {
		return w.EmpireLetter(fallback)
	}
	return b.String()
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

// deliverIPMessage puts an arriving message in the mailboxes it is addressed to,
// and says nothing anywhere else. IB used to post a planet news line for each
// arrival — naming the sender for a planet-wide message, and reporting one sent
// to the Coordinator or to a realm that had died — which put private mail in
// front of the whole planet and filled the feed with traffic notices (#146).
// The original posts nothing: process_interbbs_message_packet writes
// DATA\MSG.BRF and touches no news file.
func (w *World) deliverIPMessage(m IPMessage) {
	when := m.When
	if when == "" {
		when = timeNow().Format(StampFormat)
	}
	msg := Message{From: m.FromEmpire, FromBoard: m.FromBoard, When: when, Body: m.Body}
	if m.ToEmpire != "" {
		to := w.remoteTarget(m.ToEmpire)
		if to == nil {
			return
		}
		msg.To = w.addressLetters(m.ToEmpires, to)
		to.Mail = append(to.Mail, msg)
		return
	}
	if m.ToCoordinator {
		co := w.BBSCoordinator()
		if co == nil {
			return
		}
		msg.To = w.EmpireLetter(co)
		co.Mail = append(co.Mail, msg)
		return
	}
	// Every living realm, computer barons included — the same reach the local
	// "send to all" has.
	// A planet-wide message is addressed to everyone, so the letters are this
	// board's own roster and no list has to ride the packet.
	var everyone strings.Builder
	for _, e := range w.Empires {
		if e.Alive {
			everyone.WriteString(w.EmpireLetter(e))
		}
	}
	all := everyone.String()
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		one := msg
		one.To = all
		e.Mail = append(e.Mail, one)
	}
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
