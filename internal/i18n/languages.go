package i18n

import "strings"

// Language is one non-English language that Immortal Barons ships translations
// for. English is always the default and the fallback for untranslated text, so
// it is not listed here.
type Language struct {
	Code string // ISO code, e.g. "de"
	Name string // endonym (the language's name in its own tongue), for pickers
}

// Languages is the single source of truth for the translated languages the game
// and the website offer, in menu order. Add a language once, here, and the
// in-game language menu and the website language switcher both pick it up; the
// translation scripts discover it from the catalog files on disk. The only
// other wiring a new language needs is its PO catalogs (see docs/translating.md).
var Languages = []Language{
	{Code: "de", Name: "Deutsch"},
	{Code: "ru", Name: "Русский"},
	{Code: "nl", Name: "Nederlands"},
}

// Codes returns just the language codes from Languages, in order.
func Codes() []string {
	out := make([]string, len(Languages))
	for i, l := range Languages {
		out[i] = l.Code
	}
	return out
}

// MatchTag returns the language code for a BCP 47 tag, or "" for English and
// for any language the game does not ship. It compares the primary subtag only:
// a board writing "de-AT" or "de-DE" means the German catalog either way, since
// the catalogs are not regional. Comparison is case-insensitive, as BCP 47
// requires.
//
// "" is the game's English default, so an unknown tag is not an error here —
// the caller simply reads English, which is also the fallback for any string a
// catalog has not translated.
func MatchTag(tag string) string {
	primary, _, _ := strings.Cut(strings.TrimSpace(tag), "-")
	primary = strings.ToLower(primary)
	for _, l := range Languages {
		if l.Code == primary {
			return l.Code
		}
	}
	return ""
}
