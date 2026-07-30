package game

import "fmt"

// The Queen Royale's proclamations: a planetary news line in which the crown
// borrows a famous line and bends it to the realm's business.
//
// BRE does the same thing — its monarch misquotes twentieth-century politicians
// while handing out gold or buying support. IB's set is ORIGINAL: none of the
// original's lines are reproduced, and the two figures it leans on are
// deliberately absent here so nothing traces back to it.
//
// House rules for adding to the pool:
//
//   - The figure must be historical, not living, and the line must be clearly
//     BENT — a recognisable cadence twisted onto taxes, land, war or the
//     treasury. A verbatim quotation is neither funny nor ours to ship.
//   - Attribute nothing. The Queen says these; the joke is that she believes
//     they are hers. Naming the real author turns a parody into a misquotation.
//   - A quote is ONLY the quoted words — no trailing narration, and no closing
//     full stop. The deed supplies the narration and the sentence's end, so the
//     two lists combine freely and neither has to be edited to extend the other.
//   - Nothing that mocks the person rather than the crown. The target of every
//     one of these is the Queen's self-regard, not the figure she is robbing.
var queenProclamations = []string{
	"Be the change you wish to see in the realm",
	"I have a dream that barons shall be judged by the content of their treasury",
	"We shall fight them on the coastlines",
	"Give peace a chance",
	"One small step for a baron, one giant leap for the crown's share of it",
	"Nothing in this realm is to be feared, only taxed",
	"The only thing we have to fear is the crown tax itself",
	"It always seems impossible until someone else's regions are taken",
	"The supreme art of war is to subdue a rival's economy without a battle",
	"Turn and face the strange new tax rate",
	"Adventure is worthwhile in itself; land is worthwhile in gold",
	"Action is the antidote to despair, and to an idle treasury",
	"Imagination is more important than knowledge, and neither pays maintenance",
	"Let them eat surplus grain",
	"I came, I saw, I invoiced",
	"Speak softly, and keep a very large standing army",
	"A baron who stands for nothing will fall for any treaty",
	"The arc of the planet is long, but it bends toward the crown",
	"They may take our land, but they will never take our paperwork",
	"The only thing necessary for the triumph of taxation is that barons do nothing",
}

// proclamationDeeds are what the crown is quietly doing while it proclaims.
// Each completes the sentence the quote begins, so it starts lowercase and ends
// with the full stop.
var proclamationDeeds = []string{
	"and distributes gold to nobody in particular.",
	"while quietly raising the palace budget.",
	"and orders a statue of herself, at your expense.",
	"as the treasury doors are seen closing.",
	"and declares the matter settled.",
	"while the court applauds on cue.",
	"and returns to her banquet.",
	"as the tax collectors ride out.",
	"and raises the crown tax anyway.",
	"while changing precisely nothing.",
}

// Proclamation renders one royal proclamation from a quote and a deed. Split out
// from the news call so the wording can be exercised by a test without driving a
// whole game day.
func Proclamation(quote, deed string) string {
	return fmt.Sprintf("The Queen Royale proclaims, %q, %s", quote, deed)
}

// postProclamationNews adds one royal proclamation to the day's news, on a
// chance so the feed does not turn into a quote book. Flavour only: it moves
// nothing in the economy. Draws from the world rng, like the rest of the day's
// randomness.
func (w *World) postProclamationNews() {
	if w.rng.Intn(100) >= ProclamationChancePct {
		return
	}
	w.postNews(Proclamation(
		queenProclamations[w.rng.Intn(len(queenProclamations))],
		proclamationDeeds[w.rng.Intn(len(proclamationDeeds))],
	))
}
