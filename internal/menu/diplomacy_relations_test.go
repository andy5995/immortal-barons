package menu

import (
	"strings"
	"testing"
)

// The Diplomacy send-to picker shows a Relations column: the treaty types held
// with each empire (or "None"), matching BRE's Relations list.
func TestDiplomacyPickerShowsRelationsColumn(t *testing.T) {
	w := newWorld()
	p := w.Player()
	ally := recipients(w)[0]
	stranger := w.World.AddHuman("bob", "Bobland") // no treaty with p

	// Form a Full Defense Alliance between p and ally.
	w.World.ProposeTreaty(ally, p, "Full Defense Alliance")
	w.World.AcceptTreaty(p, ally.Name, "Full Defense Alliance")

	action := negotiateTreaty("Free Trade Agreement")
	f := &fakeSession{keys: []rune("0")} // list, then cancel at the prompt
	action(f, w)
	out := f.out.String()

	if !strings.Contains(out, "Relations") {
		t.Errorf("picker should show a Relations column, got:\n%s", out)
	}
	if !strings.Contains(out, "Full Defense Alliance") {
		t.Errorf("the allied empire's held treaty should show in Relations, got:\n%s", out)
	}
	if !strings.Contains(out, stranger.Name) || !strings.Contains(out, "None") {
		t.Errorf("an empire with no treaty should show Relations: None, got:\n%s", out)
	}
}
