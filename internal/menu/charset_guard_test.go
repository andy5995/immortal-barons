package menu

import (
	"strings"
	"testing"
)

// In a CP437 session (UTF8 false) all output is forced to English regardless of
// the empire's stored language, so a language chosen via the UTF-8 web
// front-end can't mojibake when the same empire is reached through a CP437 door.
func TestPlayerLangForcedEnglishWhenNotUTF8(t *testing.T) {
	w := newWorld() // newWorld sets UTF8 = true
	w.Player().Language = "de"

	if got := playerLang(w); got != "de" {
		t.Errorf("UTF-8 session: playerLang = %q, want %q", got, "de")
	}

	w.UTF8 = false
	if got := playerLang(w); got != "" {
		t.Errorf("CP437 session: playerLang = %q, want English (\"\")", got)
	}
}

// pickLanguage is refused in a CP437 session and does not change the language.
func TestPickLanguageBlockedWhenNotUTF8(t *testing.T) {
	w := newWorld()
	w.UTF8 = false
	w.Player().Language = "en"
	f := &fakeSession{keys: []rune("2\r")} // would pick a language if it got that far

	pickLanguage(f, w)

	if !strings.Contains(f.out.String(), "-utf8") {
		t.Errorf("expected the -utf8 requirement message; got:\n%s", f.out.String())
	}
	if w.Player().Language != "en" {
		t.Errorf("language changed in CP437 mode: %q", w.Player().Language)
	}
}
