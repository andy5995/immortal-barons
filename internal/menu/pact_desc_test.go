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
	action := negotiateTreaty(game.FreeTradeAgreement)
	f := &fakeSession{keys: []rune("0")} // cancel at the picker
	action(f, w)
	out := f.out.String()
	if !strings.Contains(out, "income") {
		t.Errorf("Free Trade Agreement flow should describe the pact (its income effect), got:\n%s", out)
	}
}

// Every pact carries a description, so none opens without a blurb, and every one
// the engine knows is reachable from the menu — a row added to the table with no
// item behind it is a treaty nobody can form.
func TestEveryPactHasADescriptionAndAnItem(t *testing.T) {
	items := BuildMenus().Diplomacy.Items
	for _, p := range game.Pacts {
		if strings.TrimSpace(p.Desc) == "" {
			t.Errorf("pact %q has no description", p.Name)
		}
		found := false
		for _, it := range items {
			if it.Label == p.Name {
				found = true
			}
		}
		if !found {
			t.Errorf("pact %q is on no Diplomacy menu item", p.Name)
		}
	}
}
