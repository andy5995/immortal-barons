package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestReadTurnMailReadsWithoutAsking: BRE asks nothing — the messages follow the
// recap and are read one at a time. Deleting both empties the inbox (#3).
func TestReadTurnMailReadsWithoutAsking(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = []game.Message{
		{From: "Alice", To: "A", When: "07/24/2026  09:00:00", Body: "hi"},
		{From: "Bob", To: "A", When: "07/24/2026  09:01:00", Body: "gg"},
	}

	f := &fakeSession{keys: []rune("dd")} // Delete each; no y/n gate to answer
	readTurnMail(f, w, true)

	out := f.out.String()
	if strings.Contains(out, "new message") || strings.Contains(out, "Read them now") {
		t.Errorf("the count line and the read gate are gone in BRE's flow, got: %q", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("expected both senders shown, got: %q", out)
	}
	if len(p.Mail) != 0 {
		t.Errorf("deleting both should empty the inbox, got %v", p.Mail)
	}
}

// An empty inbox says so on the session's first turn — where BRE prints it — and
// stays quiet on later turns so the line isn't repeated all day.
func TestReadTurnMailAnnouncesEmptyOnlyOnce(t *testing.T) {
	w := newWorld()
	w.Player().Mail = nil

	f := &fakeSession{}
	readTurnMail(f, w, true)
	if out := f.out.String(); !strings.Contains(out, "You have no messages.") {
		t.Errorf("first turn should report an empty inbox, got: %q", out)
	}

	f2 := &fakeSession{}
	readTurnMail(f2, w, false)
	if out := f2.out.String(); out != "" {
		t.Errorf("later turns should stay quiet on an empty inbox, got: %q", out)
	}
}

// Enter must not answer the [R]/[D]/[I]/[Q] prompt: a message is read once, and
// a player holding Enter through the pre-turn stops would skip past it.
func TestMailChoiceIgnoresEnter(t *testing.T) {
	f := &fakeSession{keys: []rune("\r\n\rq")}
	if got := mailChoice(f); got != 'Q' {
		t.Errorf("mailChoice = %q, want Q — Enter should have been swallowed", got)
	}
}

// The box measurements come from a live BRE capture: a 76-column top rule with
// the stamp inside it, and a short 41-column rule under the headers.
func TestRenderMessageBoxGeometry(t *testing.T) {
	f := &fakeSession{}
	renderMessage(f, game.Message{From: "Alice", To: "A", When: "07/30/2026  00:24:05", Body: "hi"})

	var top, sep string
	for _, line := range strings.Split(sgr.ReplaceAllString(f.out.String(), ""), "\n") {
		switch {
		case strings.HasPrefix(line, "┌"):
			top = line
		case strings.HasPrefix(line, "├"):
			sep = line
		}
	}
	// Golden literal, not a measure against mailBoxWidth: the renderer builds
	// from that same constant, so measuring against it passes even if the
	// constant drifts off BRE's captured 76.
	if want := "┌──────────────────────────────────────────────────07/30/2026  00:24:05─────"; top != want {
		t.Errorf("top rule:\n got %q\nwant %q", top, want)
	}
	if n := len([]rune(sep)); n != mailSepWidth {
		t.Errorf("separator is %d columns, want %d: %q", n, mailSepWidth, sep)
	}
}
