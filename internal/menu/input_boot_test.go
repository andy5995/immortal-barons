package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// TestPromptsPropagateIdleBoot: an idle/disconnect boot (ErrSessionEnded) inside
// a single-key prompt must end the session (session.End), not return a
// cancel/default — otherwise the boot only cancels the sub-prompt and the
// session limps on until the next guarded read (a false "Disconnected" + a
// second idle cycle). See the region-picker report.
func TestPromptsPropagateIdleBoot(t *testing.T) {
	prompts := map[string]func(session.Session) int{
		"promptRegionType":    func(s session.Session) int { return promptRegionType(s, "Your choice?") },
		"promptBuyRegionType": promptBuyRegionType,
	}
	for name, fn := range prompts {
		t.Run(name, func(t *testing.T) {
			f := &fakeSession{boot: true}
			ended := false
			func() {
				defer func() {
					if _, ok := session.AsEnd(recover()); ok {
						ended = true
					}
				}()
				fn(f)
			}()
			if !ended {
				t.Fatalf("%s swallowed the idle boot instead of ending the session", name)
			}
		})
	}
}
