// Package i18n is a tiny, dependency-free gettext-PO translator for the game's
// UI strings (menu titles, item labels, prompts, reports). Translators edit the
// per-language catalogs under locale/<lang>.po in the standard gettext format,
// which distro and community tools (Poedit, Weblate) already speak; the game
// embeds them and looks strings up by their English source at render time.
//
// This is deliberately not the po4a/help pipeline (that one translates whole
// Markdown documents). It shares the PO *format* so there is one translator
// workflow, but UI strings are short and looked up individually here.
package i18n

import (
	"embed"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed locale
var locale embed.FS

// catalogs maps a language code to its msgid->msgstr table, and plurals to its
// msgid->msgstr[n] table for entries with a msgid_plural, loaded once at init.
var catalogs, plurals = load()

// T returns the translation of msgid in lang, or msgid itself when lang is
// empty/unknown or the string is untranslated. English callers pass lang "".
func T(lang, msgid string) string {
	if lang == "" || msgid == "" {
		return msgid
	}
	if c, ok := catalogs[lang]; ok {
		if s := c[msgid]; s != "" {
			return s
		}
	}
	return msgid
}

// TN returns the form of a counted message for n in lang: one and many are the
// English singular and plural, and one is the catalog key, as gettext's
// ngettext has it. A language picks among its own number of forms (Russian has
// three) by pluralForm; an untranslated message falls back to English.
func TN(lang, one, many string, n int64) string {
	if forms := plurals[lang][one]; forms != nil {
		if i := pluralForm(lang, n); i < len(forms) && forms[i] != "" {
			return forms[i]
		}
	}
	if n == 1 {
		return one
	}
	return many
}

// pluralRules is each catalog's gettext Plural-Forms rule, in Go: how many
// forms the language has, and which one a count takes. The catalogs state the
// same rule in their headers for msgmerge and the translators' tools;
// TestPluralRulesMatchTheCatalogs holds the two together, and fails for a new
// catalog until its rule is added here.
var pluralRules = map[string]struct {
	forms int
	pick  func(n int64) int
}{
	"de": {2, notOne},
	"nl": {2, notOne},
	"pt": {2, notOne},
	"sv": {2, notOne},
	"ru": {3, func(n int64) int {
		switch {
		case n%10 == 1 && n%100 != 11:
			return 0
		case n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20):
			return 1
		}
		return 2
	}},
}

func notOne(n int64) int {
	if n != 1 {
		return 1
	}
	return 0
}

// pluralForm is the msgstr index n takes in lang; English rules for a language
// without one.
func pluralForm(lang string, n int64) int {
	if n < 0 {
		n = -n
	}
	if r, ok := pluralRules[lang]; ok {
		return r.pick(n)
	}
	return notOne(n)
}

// Has reports whether lang has a catalog on file (used to offer only languages
// that actually ship translations).
func Has(lang string) bool {
	_, ok := catalogs[lang]
	return ok
}

// Strings returns every translated string (the msgstr values) in lang's
// catalog, for callers that inspect a catalog as a whole — e.g. testing whether
// a language is representable in a legacy code page. Order is unspecified; nil
// for an unknown language.
func Strings(lang string) []string {
	c, ok := catalogs[lang]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(c))
	for _, v := range c {
		out = append(out, v)
	}
	return out
}

func load() (map[string]map[string]string, map[string]map[string][]string) {
	out := map[string]map[string]string{}
	outPl := map[string]map[string][]string{}
	entries, err := fs.ReadDir(locale, "locale")
	if err != nil {
		return out, outPl
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".po") {
			continue
		}
		raw, err := locale.ReadFile("locale/" + name)
		if err != nil {
			continue
		}
		lang := strings.TrimSuffix(name, ".po")
		out[lang], outPl[lang] = parsePO(string(raw))
	}
	return out, outPl
}

// parsePO reads a gettext .po into a msgid->msgstr map, plus a msgid->msgstr[n]
// map for the entries that carry a msgid_plural. It handles the common subset
// the UI needs: msgid/msgid_plural/msgstr/msgstr[n] with adjacent quoted
// continuation lines and the standard C string escapes. The header entry (empty
// msgid) is skipped, as are comments and untranslated (empty msgstr) entries.
// Entries flagged "#, fuzzy" are also skipped: gettext marks a translation
// fuzzy precisely because a human has not validated it yet, and msgfmt excludes
// fuzzy entries from its output unless --use-fuzzy (which its own manual calls
// "usually wrong"). Skipping them here falls back to English until a translator
// clears the flag, and keeps unreviewed msgmerge guesses (whose format verbs may
// not even match) out of the game.
func parsePO(src string) (map[string]string, map[string][]string) {
	out := map[string]string{}
	outPl := map[string][]string{}
	var id, idPl, str strings.Builder
	var forms []strings.Builder
	// which field the current quoted lines append to: 0=none, 1=msgid,
	// 2=msgstr, 3=msgid_plural, 4+i=msgstr[i]
	field := 0
	fuzzy := false
	flush := func() {
		if id.Len() > 0 && !fuzzy {
			if idPl.Len() > 0 {
				var got []string
				has := false
				for _, f := range forms {
					got = append(got, f.String())
					has = has || f.Len() > 0
				}
				if has {
					outPl[id.String()] = got
				}
			} else if str.Len() > 0 {
				out[id.String()] = str.String()
			}
		}
		if id.Len() > 0 {
			fuzzy = false // the flag belongs to this entry only
		}
		id.Reset()
		idPl.Reset()
		str.Reset()
		forms = nil
		field = 0
	}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "#,"):
			// A flag comment (e.g. "#, fuzzy" or "#, fuzzy, c-format") for the
			// entry that follows. Note fuzzy without clearing it on this no-op
			// flush, so it survives to the entry's own flush.
			flush()
			if strings.Contains(t, "fuzzy") {
				fuzzy = true
			}
		case t == "" || strings.HasPrefix(t, "#"):
			flush()
		case strings.HasPrefix(t, "msgid_plural "):
			field = 3
			idPl.WriteString(unquote(strings.TrimPrefix(t, "msgid_plural ")))
		case strings.HasPrefix(t, "msgid "):
			flush()
			field = 1
			id.WriteString(unquote(strings.TrimPrefix(t, "msgid ")))
		case strings.HasPrefix(t, "msgstr["):
			i, rest, ok := strings.Cut(strings.TrimPrefix(t, "msgstr["), "]")
			n, err := strconv.Atoi(i)
			if !ok || err != nil || n < 0 || n > 9 {
				field = 0
				continue
			}
			for len(forms) <= n {
				forms = append(forms, strings.Builder{})
			}
			field = 4 + n
			forms[n].WriteString(unquote(rest))
		case strings.HasPrefix(t, "msgstr "):
			field = 2
			str.WriteString(unquote(strings.TrimPrefix(t, "msgstr ")))
		case strings.HasPrefix(t, "\""):
			switch {
			case field == 1:
				id.WriteString(unquote(t))
			case field == 2:
				str.WriteString(unquote(t))
			case field == 3:
				idPl.WriteString(unquote(t))
			case field >= 4:
				forms[field-4].WriteString(unquote(t))
			}
		}
	}
	flush()
	return out, outPl
}

// unquote turns a `"..."` PO token into its string value, applying the C escapes
// gettext uses. Anything not wrapped in quotes is returned as-is.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	if v, err := strconv.Unquote(s); err == nil {
		return v
	}
	// Fall back to a manual pass if strconv is unhappy with an odd escape.
	inner := s[1 : len(s)-1]
	inner = strings.ReplaceAll(inner, `\"`, `"`)
	inner = strings.ReplaceAll(inner, `\n`, "\n")
	inner = strings.ReplaceAll(inner, `\t`, "\t")
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	return inner
}
