package menu

import (
	"testing"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/screen"
)

// Every label cell must be wide enough for the label in every language, so a
// translation never silently truncates in-game. If this fails, widen that
// field's { } span in the named .ans template.
func TestEmpireStatusLabelsFitEveryLanguage(t *testing.T) {
	for _, page := range empireStatusPages {
		tpl, err := empireStatusFS.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, lbl := range screen.Labels(tpl) {
			for _, lang := range i18n.Codes() {
				got := i18n.T(lang, lbl.Msgid)
				if w := utf8.RuneCountInString(got); w > lbl.Width {
					t.Errorf("%s: %q [%s] is %d cols but its cell is %d — widen the {%s} span",
						page, got, lang, w, lbl.Width, lbl.Msgid)
				}
			}
		}
	}
}

// A treasury past a billion reaches the status screen in the abbreviated form.
// The formatter is unit-tested on its own; this pins that the screen actually
// routes gold through it, which is where the digits used to run past the column.
func TestEmpireStatusAbbreviatesBillions(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Bank = 1_847_392_104, 12_500_000_000

	fields, _ := empireStatusFields(&fakeSession{}, w)
	if got := fields["gold"]; got != "1.8473B" {
		t.Errorf("gold rendered %q, want %q", got, "1.8473B")
	}
	if got := fields["bank"]; got != "12.5000B" {
		t.Errorf("bank rendered %q, want %q", got, "12.5000B")
	}
}
