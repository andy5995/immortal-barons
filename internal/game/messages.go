package game

// SendMail appends a message from `from` to `to`'s inbox.
func (w *World) SendMail(from, to *Empire, text string) {
	to.Mail = append(to.Mail, from.Name+": "+text)
}

// PostBulletin adds a line to the planetary bulletin, keeping the most
// recent 20 entries.
func (w *World) PostBulletin(from *Empire, text string) {
	w.NewsToday = append(w.NewsToday, from.Name+": "+text)
	if len(w.NewsToday) > 20 {
		w.NewsToday = w.NewsToday[len(w.NewsToday)-20:]
	}
}
