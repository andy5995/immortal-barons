package game

// SendMail appends a message from `from` to `to`'s inbox.
func (w *World) SendMail(from, to *Empire, text string) {
	to.Mail = append(to.Mail, from.Name+": "+text)
}

// PostBulletin adds a line to the planetary bulletin, keeping the most
// recent 20 entries.
func (w *World) PostBulletin(from *Empire, text string) {
	w.Bulletin = append(w.Bulletin, from.Name+": "+text)
	if len(w.Bulletin) > 20 {
		w.Bulletin = w.Bulletin[len(w.Bulletin)-20:]
	}
}
