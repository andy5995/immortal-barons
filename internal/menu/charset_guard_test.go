package menu

import (
	"strings"
	"testing"
)

// A UTF-8 session renders any language. A CP437 session renders a catalog that
// maps entirely to CP437 (German) and falls back to English for one that does
// not (Russian/Cyrillic), so a web-chosen language can't mojibake through a
// CP437 door.
func TestPlayerLangCP437Fallback(t *testing.T) {
	w := newWorld() // newWorld sets UTF8 = true
	w.Player().Language = "de"
	if got := playerLang(w); got != "de" {
		t.Errorf("UTF-8 session: playerLang = %q, want %q", got, "de")
	}

	w.UTF8 = false
	if got := playerLang(w); got != "de" {
		t.Errorf("CP437 session, German (CP437-safe): playerLang = %q, want %q", got, "de")
	}

	w.Player().Language = "ru" // Cyrillic: not representable in CP437
	if got := playerLang(w); got != "" {
		t.Errorf("CP437 session, Russian: playerLang = %q, want English (\"\")", got)
	}
}

// In a CP437 session the language picker offers only CP437-representable
// languages (English, German) — not Cyrillic — and selecting one still works.
func TestPickLanguageCP437Subset(t *testing.T) {
	w := newWorld()
	w.UTF8 = false
	w.Player().Language = ""               // English
	f := &fakeSession{keys: []rune("2\r")} // 1) English  2) Deutsch  (Russian excluded)

	pickLanguage(f, w)

	if strings.Contains(f.out.String(), "Русский") {
		t.Errorf("CP437 picker should not offer Cyrillic; got:\n%s", f.out.String())
	}
	if w.Player().Language != "de" {
		t.Errorf("expected German selected in CP437 picker, got %q", w.Player().Language)
	}
}
