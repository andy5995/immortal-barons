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

// catalogs maps a language code to its msgid->msgstr table, loaded once at init.
var catalogs = load()

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

// Has reports whether lang has a catalog on file (used to offer only languages
// that actually ship translations).
func Has(lang string) bool {
	_, ok := catalogs[lang]
	return ok
}

func load() map[string]map[string]string {
	out := map[string]map[string]string{}
	entries, err := fs.ReadDir(locale, "locale")
	if err != nil {
		return out
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
		out[lang] = parsePO(string(raw))
	}
	return out
}

// parsePO reads a gettext .po into a msgid->msgstr map. It handles the common
// subset the UI needs: msgid/msgstr with adjacent quoted continuation lines and
// the standard C string escapes. The header entry (empty msgid) is skipped, as
// are comments and untranslated (empty msgstr) entries.
func parsePO(src string) map[string]string {
	out := map[string]string{}
	var id, str strings.Builder
	// which field the current quoted lines append to: 0=none, 1=msgid, 2=msgstr
	field := 0
	flush := func() {
		if id.Len() > 0 && str.Len() > 0 {
			out[id.String()] = str.String()
		}
		id.Reset()
		str.Reset()
		field = 0
	}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
			flush()
		case strings.HasPrefix(t, "msgid "):
			flush()
			field = 1
			id.WriteString(unquote(strings.TrimPrefix(t, "msgid ")))
		case strings.HasPrefix(t, "msgstr "):
			field = 2
			str.WriteString(unquote(strings.TrimPrefix(t, "msgstr ")))
		case strings.HasPrefix(t, "\""):
			if field == 1 {
				id.WriteString(unquote(t))
			} else if field == 2 {
				str.WriteString(unquote(t))
			}
		}
	}
	flush()
	return out
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
