package menu

import (
	"io"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// eofSession has nothing left to give, which is what a run with stdin closed
// looks like: `immortal-barons -reset </dev/null`, a cron job, a BBS that hands
// the door no terminal.
type eofSession struct{ reads int }

func (e *eofSession) Write(p []byte) (int, error) { return len(p), nil }
func (e *eofSession) ReadKey() (rune, error)      { e.reads++; return 0, io.EOF }

// A prompt whose input has ended must END the session, not return an empty
// string. Its callers loop on input they cannot parse, so a swallowed EOF spins
// them against a stream that will never produce another byte: -reset with stdin
// closed redrew the Configuration Editor 300,616 times and wrote 262 MB in five
// seconds before anything stopped it.
func TestAPromptEndsTheSessionWhenInputRunsOut(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(session.Session)
	}{
		{"prompt", func(s session.Session) { prompt(s, "Edit which?") }},
		{"promptInt", func(s session.Session) { promptInt(s, "How many?") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &eofSession{}
			ended := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						if _, ok := session.AsEnd(r); ok {
							ended = true
							return
						}
						panic(r)
					}
				}()
				tc.call(s)
			}()
			if !ended {
				t.Error("returned instead of ending the session, so a caller looping on bad input would spin")
			}
			if s.reads > 1 {
				t.Errorf("read %d times after the stream ended; one is enough to know", s.reads)
			}
		})
	}
}
