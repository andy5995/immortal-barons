package game

import (
	"fmt"
	"strings"
)

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
//
// FromEmpire is empty for one kind of message this board sends itself: a
// bounce telling a sender their own message could not be delivered (see
// bounceIPMessage). That is a notice from the PLANET, not from a baron, and the
// emptiness is also the loop guard — deliverIPMessage never bounces a message
// whose FromEmpire is already empty, so two boards that can no longer reach
// each other's barons cannot bounce a failure back and forth forever.
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
	to := strings.Join(boards, ", ")
	if toCoordinator {
		to = ipAddress([]string{"CO"}, boards...)
	}
	w.keepIPCopy(from, boards, to, body)
}

// ipAddress renders an interplanetary address for a sender's copy: the names,
// then "@" and the planets. Names and planets only, so nothing in it needs
// translating.
func ipAddress(names []string, boards ...string) string {
	return strings.Join(names, ", ") + " @ " + strings.Join(boards, ", ")
}

// keepIPCopy files the sender's copy of an interplanetary message, once however
// many boards or realms it went to, and only when sendIP actually sent it.
func (w *World) keepIPCopy(from *Empire, boards []string, to, body string) {
	if from == nil || body == "" || len(boards) == 0 {
		return
	}
	KeepSentCopy(from, w.Config.BoardID, to, StoredStamp(timeNow()), body)
}

// SendIPMessageToBarons queues body for named realms on one board, one message
// each, narrowed with ToEmpire. That is how BRE's Single Planet mode addresses
// a message once the sender has picked letters at its `(A-Y,Z=All,?=List) Send
// to:` prompt: the address is a set of realms on the chosen planet, not the
// planet itself. N messages rather than a recipient list on one keeps the
// packet's shape unchanged — ToEmpire already exists for the author-only reply,
// and deliverIPMessage already honors it.
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
	if len(addressed) > 0 {
		w.keepIPCopy(from, []string{board}, ipAddress(addressed, board), body)
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
// and the text; the sender and the stamp are filled in here. from is nil only
// for a bounce (see bounceIPMessage), which speaks for the planet rather than
// for a baron and so leaves FromEmpire as m already has it — empty.
func (w *World) sendIP(from *Empire, boards []string, m IPMessage) {
	if m.Body == "" || len(boards) == 0 {
		return
	}
	m.FromBoard = w.Config.BoardID
	if from != nil {
		m.FromEmpire = from.Name
	}
	m.When = StoredStamp(timeNow())
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

// deliverIPMessage puts an arriving message in the mailboxes it is addressed
// to, and posts nothing to this planet's news either way. IB used to post a
// planet news line for each arrival — naming the sender for a planet-wide
// message, and reporting one sent to the Coordinator or to a realm that had
// died — which put private mail in front of the whole planet and filled the
// feed with traffic notices (#146). The original posts nothing either:
// process_interbbs_message_packet writes DATA\MSG.BRF and touches no news
// file.
//
// **Deliberate divergence, decided 2026-09-13.** BRE tells the sender nothing
// when delivery fails, either — the disassembled handler (BRE.OVR 0x048f2f) is
// file I/O only, with no check that the addressee exists and no call that
// queues anything back. A message to a dead realm or an unelected Coordinator
// is written and never read, silently, forever. IB chooses to do better: a
// message that cannot be delivered is bounced back to its sender as an
// ordinary IP message (see bounceIPMessage) rather than reintroducing #146's
// planet-news mistake — the sender sees it as mail, which is the model players
// already have, not as a recap event the way an unreachable strike reports
// home (applyAttackResult, OutcomeNotFound). That is a deliberate difference,
// not an oversight: a strike has nothing else to show, where a message can
// just be handed back.
func (w *World) deliverIPMessage(m IPMessage) {
	when := m.When
	if when == "" {
		when = StoredStamp(timeNow())
	}
	msg := Message{From: m.FromEmpire, FromBoard: m.FromBoard, When: when, Body: m.Body}
	if m.ToEmpire != "" {
		to := w.remoteTarget(m.ToEmpire)
		if to == nil {
			w.bounceIPMessage(m, fmt.Sprintf("Your message to %s of %s could not be delivered. There is no such realm there.", m.ToEmpire, w.Config.BoardID))
			return
		}
		msg.To = w.addressLetters(m.ToEmpires, to)
		to.deliver(msg)
		return
	}
	if m.ToCoordinator {
		co := w.BBSCoordinator()
		if co == nil {
			w.bounceIPMessage(m, fmt.Sprintf("Your message to the Coordinator of %s could not be delivered. No Coordinator has been elected there.", w.Config.BoardID))
			return
		}
		msg.To = w.EmpireLetter(co)
		co.deliver(msg)
		return
	}
	// Every living realm, computer barons included — the same reach the local
	// "send to all" has.
	// A planet-wide message is addressed to everyone, so the letters are this
	// board's own roster and no list has to ride the packet. Nothing here can
	// fail this way: there is no address to come up empty.
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
		e.deliver(one)
	}
}

// bounceIPMessage tells m's sender their message did not arrive: reason, then a
// copy of the message itself — its original date and its address — so the
// sender can judge what to do next (address the planet instead, or give up on
// a realm that is simply gone). It rides the same IPMessages channel as any
// other message, addressed back with ToEmpire and no FromEmpire, which is what
// marks it as the planet speaking rather than a baron.
//
// The wording is English and stays English. It is composed on the RECEIVING
// board, which knows nothing about the reader's language — the reader is a
// baron on another planet — and it rides the wire as message text, so there is
// no later render to translate it at. Wrapping it in tr() would pick the wrong
// board's language, which is worse than leaving it alone.
//
// Never called for a message that is ITSELF such a notice (m.FromEmpire == "")
// or that names no sender to answer (m.FromBoard == "") — the loop guard: a
// bounce that could not itself be delivered (the original sender has since
// died too) must die quietly rather than bounce again.
func (w *World) bounceIPMessage(m IPMessage, reason string) {
	if m.FromEmpire == "" || m.FromBoard == "" {
		return
	}
	var b strings.Builder
	b.WriteString(reason)
	fmt.Fprintf(&b, "\nSent %s.\n", m.When)
	if to := strings.Join(m.ToEmpires, ", "); to != "" {
		fmt.Fprintf(&b, "To: %s.\n", to)
	}
	b.WriteString("\n")
	b.WriteString(m.Body)
	w.sendIP(nil, []string{m.FromBoard}, IPMessage{ToEmpire: m.FromEmpire, Body: b.String()})
}

// ReplyIPMessage answers an interplanetary message. A public reply goes to the
// whole of the sender's planet; otherwise it reaches only the baron who wrote,
// which is the choice BRE's "Public Reply?" offers.
func (w *World) ReplyIPMessage(from *Empire, board, author, body string, public bool) {
	m := IPMessage{Body: body}
	to := board
	if !public {
		m.ToEmpire = author
		to = ipAddress([]string{author}, board)
	}
	w.sendIP(from, []string{board}, m)
	w.keepIPCopy(from, []string{board}, to, body)
}
