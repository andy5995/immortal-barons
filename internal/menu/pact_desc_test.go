package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Opening a treaty type's negotiation shows a short description of what the pact
// does (BRE shows a pact blurb before the send-to list).
func TestDiplomacyShowsPactDescription(t *testing.T) {
	w := newWorld()
	action := negotiateTreaty("Free Trade Agreement")
	f := &fakeSession{keys: []rune("0")} // cancel at the picker
	action(f, w)
	out := f.out.String()
	if !strings.Contains(out, "income") {
		t.Errorf("Free Trade Agreement flow should describe the pact (its income effect), got:\n%s", out)
	}
}

// Every treaty type has a description, so none opens without a blurb.
func TestEveryTreatyTypeHasADescription(t *testing.T) {
	for _, ttype := range game.TreatyTypes {
		if strings.TrimSpace(treatyDescriptions[ttype]) == "" {
			t.Errorf("treaty type %q has no description", ttype)
		}
	}
}
