package game

import (
	"encoding/json"
	"time"
)

// StampFormat is how the original prints a date and time — two spaces between
// them. Used for a message's When and for a recap entry's stamp.
const StampFormat = "01/02/2006  15:04:05"

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
		m.When = time.Now().Format(StampFormat)
	}
	to.Mail = append(to.Mail, m)
}

// A message's "To" records the recipient's slot letter — see EmpireLetter in
// slots.go. It is the label the recipient carried when the mail was sent, not a
// live reference: a realm that falls and has its slot re-occupied leaves old
// mail naming a letter someone else now answers to.
