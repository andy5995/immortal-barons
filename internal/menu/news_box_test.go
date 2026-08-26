package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// The Daily Bulletin's title joins the board name to "Daily Bulletin" with an em
// dash, and the box's top border is drawn to the title's measured width. "—" is
// one column only while the caller reads UTF-8: the CP437 and plain-ASCII
// writers rewrite it as two hyphens below every layer that counts columns, so
// the top border used to run a column past the verticals under it (#192) — on
// the default charset, at that. The ASCII case goes through the real writer, so
// the substitution has happened on the wire before anything here measures it.
func TestDailyBulletinBoxBordersLineUp(t *testing.T) {
	for _, charset := range []struct {
		name           string
		utf8           bool
		top, side, bot rune
	}{
		{"UTF-8", true, '┌', '│', '└'},
		{"ASCII", false, '+', '|', '+'},
	} {
		t.Run(charset.name, func(t *testing.T) {
			w := newWorld()
			w.Term = Term{UTF8: charset.utf8, ASCII: !charset.utf8}
			w.With(func() {
				w.Config.IBBS = true
				w.Config.BoardID = "Nite Eyes BBS"
			})
			f := &fakeSession{keys: []rune{' '}}
			var s session.Session = f
			if !charset.utf8 {
				s = session.NewASCIIWriter(f)
			}
			showBulletinToday(s, w)
			out := stripANSI(f.out.String())
			// The screen has to have been reached, and the board name has to have
			// been joined to the heading — that join is what draws the em dash.
			if !strings.Contains(out, "Daily Bulletin") || !strings.Contains(out, "Nite Eyes BBS") {
				t.Fatalf("never reached the titled Daily Bulletin box:\n%s", out)
			}

			var top, bottom string
			var sides []string
			for _, ln := range strings.Split(out, "\n") {
				ln = strings.TrimRight(ln, " \r")
				r := []rune(ln)
				if len(r) < len(newsBoxIndent)+1 || !strings.HasPrefix(ln, newsBoxIndent) {
					continue
				}
				switch r[len(newsBoxIndent)] {
				// The two corners are one glyph in ASCII, so which is which is
				// settled by order, not by the character.
				case charset.top, charset.bot:
					if top == "" {
						top = ln
					} else {
						bottom = ln
					}
				case charset.side:
					sides = append(sides, ln)
				}
			}
			if top == "" || bottom == "" || len(sides) != 3 {
				t.Fatalf("expected a top, a bottom and 3 content rows, got top=%q bottom=%q %d rows:\n%s",
					top, bottom, len(sides), out)
			}
			want := len([]rune(top))
			if n := len([]rune(bottom)); n != want {
				t.Errorf("bottom rule is %d columns, top is %d:\n%q\n%q", n, want, top, bottom)
			}
			for _, ln := range sides {
				r := []rune(ln)
				if len(r) != want {
					t.Errorf("content row is %d columns, the top border is %d:\n%q\n%q", len(r), want, top, ln)
				}
				if r[len(r)-1] != charset.side {
					t.Errorf("content row does not close with %q:\n%q", charset.side, ln)
				}
			}
		})
	}
}
