package menu

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

var errNotCoordinator = errors.New("You no longer hold the office of BBS Coordinator.")

// directivesMenu is the Messages menu's "Directives from CO": the planet's
// players read the BBS Coordinator's standing notice, and the Coordinator is
// asked first whether to read, write, append to or erase it. IB's own.
func directivesMenu(s session.Session, w *ctx) Result {
	var isCoordinator bool
	w.Read(func() {
		isCoordinator = w.Player() != nil && w.BBSCoordinator() == w.Player()
	})
	choice := 'R'
	if isCoordinator {
		choice = coordinatorChoice(s)
	}
	switch choice {
	case 'W':
		writeDirectives(s, w, false)
	case 'A':
		writeDirectives(s, w, true)
	case 'E':
		eraseDirectives(s, w)
	default:
		if !showDirectives(s, w) {
			ok(s, "There are no directives from the BBS Coordinator.")
		}
	}
	return Stay
}

// coordinatorOpts are what the Coordinator may do with the directives.
var coordinatorOpts = []keyOpt{
	{Key: 'R', Label: "Read,", Echo: "Read"},
	{Key: 'W', Label: "Write,", Echo: "Write"},
	{Key: 'A', Label: "Append, or", Echo: "Append"},
	{Key: 'E', Label: "Erase", Echo: "Erase"},
}

// coordinatorChoice asks the Coordinator [R]ead, [W]rite, [A]ppend or [E]rase;
// Enter reads, and so does input that has ended.
func coordinatorChoice(s session.Session) rune {
	fmt.Fprint(s, "\n")
	drawKeys(s, coordinatorOpts)
	return readKeys(s, coordinatorOpts, 'R', 'R')
}

// eraseDirectives removes the directives once the Coordinator confirms.
func eraseDirectives(s session.Session, w *ctx) {
	var have bool
	w.Read(func() { have = w.Directives != nil })
	if !have {
		ok(s, "There are no directives from the BBS Coordinator.")
		return
	}
	if !AskYesNo(s, "Erase the current directives?", false) {
		return
	}
	if _, err := setDirectives(w, ""); err != nil {
		fail(s, err)
		return
	}
	ok(s, "The directives have been erased.")
}

// writeDirectives replaces the planet's directives with what the Coordinator
// types. To append, the editor opens with the current lines already in it, as
// a reply opens with its quote, so the whole stays within the editor's line
// limit. Saving an empty editor clears them; aborting leaves them as they were.
func writeDirectives(s session.Session, w *ctx, appending bool) {
	var current []string
	if appending {
		w.Read(func() {
			if w.Directives != nil {
				current = strings.Split(w.Directives.Body, "\n")
			}
		})
		if len(current) >= msgMaxLines {
			// The editor would open with no line free and hand the same text back
			// as saved, reposting it under a new stamp.
			ok(s, "The directives already fill all %d lines. Write or erase them to make room.", msgMaxLines)
			return
		}
	}
	text, send := composeMessageFrom(s, current)
	if !send {
		return
	}
	erased, err := setDirectives(w, text)
	switch {
	case err != nil:
		fail(s, err)
	case erased:
		ok(s, "The directives have been erased.")
	default:
		ok(s, "The directives have been posted.")
	}
}

// setDirectives stores text as the directives, or removes them when it is
// blank, and reports which. The office is checked again under the lock, since
// the vote can move while the Coordinator is at a prompt.
func setDirectives(w *ctx, text string) (erased bool, err error) {
	text = strings.TrimRight(text, " \n")
	erased = strings.TrimSpace(text) == ""
	w.With(func() {
		p := w.Player()
		if p == nil || w.BBSCoordinator() != p {
			err = errNotCoordinator
			return
		}
		if erased {
			w.Directives = nil
			return
		}
		w.Directives = &game.Message{From: p.Name, When: game.StoredStamp(game.Now()), Body: text}
	})
	return erased, err
}

// showDirectives prints the Coordinator's directives and pauses, reporting
// false when there are none (or the game is not inter-BBS) so a caller can say
// so or stay silent.
func showDirectives(s session.Session, w *ctx) bool {
	if !w.Config.InterBBSEnabled() {
		return false
	}
	var d *game.Message
	w.Read(func() {
		if w.Directives != nil {
			c := *w.Directives
			d = &c
		}
	})
	if d == nil {
		return false
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Directives from CO"), ansi.Reset)
	d.To = tr(s, "All")
	messageBox(s, *d)
	pause(s)
	return true
}
