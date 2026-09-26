package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// pageLines is how many lines of a long text go by before the reader is asked
// to continue: a 24-row screen less the prompt and a line of context.
const pageLines = 20

// linePager writes a long text a screen at a time, pausing every perPage lines
// for Enter (continue) or Q (stop). Help topics, the Instructions reader and
// the bulletins all page through it, so they ask in the same words and read the
// key the same way.
type linePager struct {
	s       session.Session
	perPage int
	count   int
	quit    bool
}

// line writes one line, pausing first when a screen is full. It reports false
// once the reader has pressed Q.
func (p *linePager) line(text string) bool {
	if p.quit {
		return false
	}
	fmt.Fprintf(p.s, "%s\n", text)
	p.count++
	if p.count < p.perPage {
		return true
	}
	p.count = 0
	fmt.Fprintf(p.s, "\n%s%s%s", ansi.FgBrightCyan, tr(p.s, "─»>Enter to continue, Q to quit<«─"), ansi.Reset)
	k, err := readKey(p.s)
	// The rest of the line goes too: a terminal that sends CR LF for Enter would
	// otherwise leave the LF to answer the next page's prompt at once, and that
	// page would scroll past unread.
	drainInput(p.s)
	fmt.Fprint(p.s, "\n")
	if err != nil || k == 'q' || k == 'Q' {
		p.quit = true
		return false
	}
	return true
}

// finish ends the text with the usual pause, unless the reader already stopped
// or a page break has just been answered.
func (p *linePager) finish() {
	if !p.quit && p.count > 0 {
		pause(p.s)
	}
}
