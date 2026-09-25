package game

import (
	"encoding/json"
	"time"
)

// Message is one piece of empire mail: who sent it, the recipient letter(s) it
// was addressed to, when it was sent, and the body. Body lines beginning with
// "> " are quoted text — a reply quotes the message it answers.
type Message struct {
	From string
	To   string
	When string
	Body string
	// FromBoard names the sending planet on an interplanetary message, and is
	// empty on local mail. It is what lets a reply find its way back off this
	// planet, since the sender is not a realm anyone here can look up.
	FromBoard string `json:",omitempty"`
	// Sent marks the sender's own copy of a message they sent, kept in their
	// inbox beside the mail they received (IB's own; BRE keeps nothing of what
	// a player sends). To then names who it went to.
	Sent bool `json:",omitempty"`
}

// UnmarshalJSON accepts a bare string as well as an object, so a world saved
// before mail carried a sender, recipient and stamp still loads (be48fb2, in
// v0.0.3, changed the field's element type without this). Such a message keeps
// its text as the body and shows no sender or date.
func (m *Message) UnmarshalJSON(b []byte) error {
	var body string
	if err := json.Unmarshal(b, &body); err == nil {
		*m = Message{Body: body}
		return nil
	}
	type plain Message // avoid recursing into this method
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*m = Message(p)
	return nil
}

// SendMail delivers m from `from` to `to`'s inbox. From is stamped from the
// sender and When from the clock unless the caller set it, so callers only fill
// To/Body.
func (w *World) SendMail(from, to *Empire, m Message) {
	m.From = from.Name
	if m.When == "" {
		m.When = StoredStamp(timeNow())
	}
	to.deliver(m)
}

// KeepSentCopy files the sender's own copy of a message they have just sent.
// It shares the inbox, and MailboxMax, with received mail, in the order
// written. to is how the address reads in "Message To  :"; fromBoard is this
// board on an interplanetary message and empty on local mail, as on mail
// received.
func KeepSentCopy(from *Empire, fromBoard, to, when, body string) {
	from.deliver(Message{From: from.Name, FromBoard: fromBoard, To: to, When: when, Body: body, Sent: true})
}

// MailLoss is the tally a full inbox keeps of what it has deleted: how many,
// the stamp of the oldest, and when the last went. The recap reports it as one
// line and clears it.
type MailLoss struct {
	Count  int
	Oldest string
	At     time.Time
}

// deliver puts m in the inbox and holds it at MailboxMax. A full inbox gives
// up the oldest of the owner's own sent copies first, and only when it holds
// none the oldest message received, so what they send never costs them mail
// they have not read. Each loss is tallied for the owner's recap (MailLost).
func (e *Empire) deliver(m Message) {
	e.Mail = append(e.Mail, m)
	for len(e.Mail) > MailboxMax {
		drop := 0
		for i, old := range e.Mail {
			if old.Sent {
				drop = i
				break
			}
		}
		gone := e.Mail[drop]
		e.Mail = append(e.Mail[:drop], e.Mail[drop+1:]...)
		if drop == len(e.Mail) {
			// The copy just being filed was the only one: an inbox full of mail
			// received simply does not keep it, and there is nothing to report.
			continue
		}
		if e.MailLost == nil {
			e.MailLost = &MailLoss{Oldest: gone.When}
		}
		e.MailLost.Count++
		e.MailLost.At = timeNow()
	}
}

// A message's "To" records the recipient's slot letter — see EmpireLetter in
// slots.go. It is the label the recipient carried when the mail was sent, not a
// live reference: a realm that falls and has its slot re-occupied leaves old
// mail naming a letter someone else now answers to.
