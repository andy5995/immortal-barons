package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

// Every Yes/No on the Preferences menu right-aligns to ONE column, in every
// language. The width was a hardcoded 28 that English fits and translations do
// not: Dutch has two labels at 29 columns and German one at 32, each of which
// shoved its own value right and left the column ragged. English is measured
// from BRE and must not move, so the field is a floor that widens to fit.
func TestPreferenceValuesShareOneColumn(t *testing.T) {
	// Every shipped language, English included, so a language added to
	// i18n.Languages is covered without touching this test.
	for _, lang := range append([]string{"en"}, i18n.Codes()...) {
		menus := BuildMenus()
		w := newWorld()
		w.Player().Language = lang
		f := &fakeSession{}
		draw(f, w, menus.Prefs)

		yes, no := i18n.T(lang, "Yes"), i18n.T(lang, "No")
		right := map[int][]string{}
		for _, line := range strings.Split(stripANSI(f.out.String()), "\n") {
			line = strings.TrimRight(line, " ")
			if !strings.HasSuffix(line, yes) && !strings.HasSuffix(line, no) {
				continue
			}
			right[len([]rune(line))] = append(right[len([]rune(line))], line)
		}
		if len(right) == 0 {
			t.Fatalf("%s: no %s/%s rows rendered at all", lang, yes, no)
		}
		// The answers are prose, not a key the player types, so they translate.
		if lang != "en" && (yes == "Yes" || no == "No") {
			t.Errorf("%s: the toggle answers are still English (%q/%q)", lang, yes, no)
		}
		if len(right) > 1 {
			t.Errorf("%s: Yes/No ends at %d different columns, want one:\n%v",
				lang, len(right), right)
		}
	}
}

// English keeps BRE's own column. A change that widened the field for everyone
// would line the values up while silently moving the screen off the capture it
// was matched to.
func TestEnglishPreferenceColumnIsUnchanged(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	w.Player().Language = "en"
	f := &fakeSession{}
	draw(f, w, menus.Prefs)
	if !strings.Contains(stripANSI(f.out.String()), "  (7) Auto-Feed Empire             Yes") {
		t.Errorf("English row moved off BRE's column:\n%s", stripANSI(f.out.String()))
	}
}
