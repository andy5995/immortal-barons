package game

import "time"

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

// EmpireLetter is the display letter (A, B, …) for e, from its slot in the
// world's empire list — the way BRE addresses mail recipients. Returns "?" for
// an empire not in the world.
func (w *World) EmpireLetter(e *Empire) string {
	for i, x := range w.Empires {
		if x == e {
			return string(rune('A' + i))
		}
	}
	return "?"
}

// PostBulletin adds a line to the planetary bulletin, keeping the most
// recent 20 entries.
func (w *World) PostBulletin(from *Empire, text string) {
	w.NewsToday = append(w.NewsToday, from.Name+": "+text)
	if len(w.NewsToday) > 20 {
		w.NewsToday = w.NewsToday[len(w.NewsToday)-20:]
	}
}
