package i18n

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestEnglishAndUnknownFallBack(t *testing.T) {
	if got := T("", "Troopers"); got != "Troopers" {
		t.Errorf("empty lang should pass through, got %q", got)
	}
	if got := T("de", "No Such String"); got != "No Such String" {
		t.Errorf("untranslated should fall back to msgid, got %q", got)
	}
	if got := T("xx", "Troopers"); got != "Troopers" {
		t.Errorf("unknown lang should fall back, got %q", got)
	}
}

func TestTranslations(t *testing.T) {
	if got := T("de", "Troopers"); got != "Soldaten" {
		t.Errorf("de Troopers = %q, want Soldaten", got)
	}
	if got := T("ru", "Regions"); got != "Регионы" {
		t.Errorf("ru Regions = %q, want Регионы", got)
	}
}

// msgidsOf returns every active msgid in a .po in file order (with quoted
// continuations joined; obsolete "#~" and fuzzy flags are irrelevant to
// identity). Duplicates are what we are hunting, so it does NOT dedupe.
func msgidsOf(src string) []string {
	var ids []string
	var cur strings.Builder
	inID := false
	end := func() {
		if inID {
			ids = append(ids, cur.String())
			cur.Reset()
			inID = false
		}
	}
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "msgid "):
			end()
			inID = true
			cur.WriteString(unquote(strings.TrimPrefix(t, "msgid ")))
		case inID && strings.HasPrefix(t, "\""):
			cur.WriteString(unquote(t))
		default:
			end()
		}
	}
	end()
	return ids
}

// A duplicate msgid is "invalid input for other programs like msgfmt, msgmerge
// or msgcat" (gettext manual) — msgmerge outright refuses to run. Our in-house
// reader silently last-wins, so guard the committed catalogs here.
func TestCatalogsNoDuplicateMsgids(t *testing.T) {
	entries, err := fs.ReadDir(locale, "locale")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".po") {
			continue
		}
		raw, err := locale.ReadFile("locale/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, id := range msgidsOf(string(raw)) {
			if seen[id] {
				t.Errorf("%s: duplicate msgid %q (invalid gettext input)", e.Name(), id)
			}
			seen[id] = true
		}
	}
}

func TestParsePOContinuation(t *testing.T) {
	m, _ := parsePO("msgid \"a\"\nmsgstr \"\"\n\"b\"\n\nmsgid \"c\"\nmsgstr \"d\"\n")
	if m["a"] != "b" {
		t.Errorf("continuation msgstr = %q, want b", m["a"])
	}
	if m["c"] != "d" {
		t.Errorf("simple msgstr = %q, want d", m["c"])
	}
}

func TestParsePOSkipsHeaderAndEmpty(t *testing.T) {
	m, _ := parsePO("msgid \"\"\nmsgstr \"Language: de\"\n\nmsgid \"x\"\nmsgstr \"\"\n")
	if _, ok := m[""]; ok {
		t.Error("header (empty msgid) should be skipped")
	}
	if _, ok := m["x"]; ok {
		t.Error("untranslated (empty msgstr) should be skipped")
	}
}

func TestParsePOSkipsFuzzy(t *testing.T) {
	m, _ := parsePO("#, fuzzy\nmsgid \"x\"\nmsgstr \"y\"\n\nmsgid \"z\"\nmsgstr \"w\"\n")
	if _, ok := m["x"]; ok {
		t.Error("fuzzy entry should be skipped (unvalidated by a human)")
	}
	if m["z"] != "w" {
		t.Errorf("fuzzy flag must not leak to the next entry: z = %q, want w", m["z"])
	}
}

// verbs extracts fmt format verbs (%d, %s, ...) from a string, ignoring the
// literal %% escape and their flags/width.
func verbs(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && strings.ContainsRune("+-# 0123456789.[]*", rune(s[j])) {
			j++
		}
		if j < len(s) {
			out = append(out, string(s[j]))
		}
	}
	return out
}

// A mismatched verb set between msgid and msgstr would make fmt.Fprintf emit
// %!verb garbage at runtime, so guard the committed catalogs.
func TestCatalogFormatVerbsMatch(t *testing.T) {
	for lang, cat := range catalogs {
		for id, str := range cat {
			iv, sv := verbs(id), verbs(str)
			if len(iv) != len(sv) {
				t.Errorf("[%s] verb count differs\n  id:  %q %v\n  str: %q %v", lang, id, iv, str, sv)
				continue
			}
			// Order matters for %-verbs without positional args (our case).
			for k := range iv {
				if iv[k] != sv[k] {
					t.Errorf("[%s] verb %d differs (%s vs %s)\n  id:  %q\n  str: %q", lang, k, iv[k], sv[k], id, str)
					break
				}
			}
		}
	}
}

// keyMarks finds the keys a prompt names inside its own text: "[Y]es" or
// "(O)ne". The code reads those exact keys, so a translation must show the same
// letters, in the same order, whatever words it wraps around them.
var keyMarks = regexp.MustCompile(`\[([A-Za-z0-9])\]|\(([A-Z0-9])\)`)

func marks(s string) []string {
	var out []string
	for _, m := range keyMarks.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1]+m[2])
	}
	return out
}

// A translated prompt that names a key the code does not read leaves the player
// pressing a key that does nothing. German, Dutch and Portuguese all did this
// until 2026-10-02 ("[J]a" on a prompt that answers only Y, N and I). Menu
// hotkeys are not at risk — the engine draws those apart from the label — so
// this covers only keys written into translatable text.
func TestCatalogKeyLettersMatch(t *testing.T) {
	for lang, cat := range catalogs {
		for id, str := range cat {
			want := marks(id)
			if len(want) == 0 {
				continue
			}
			if got := marks(str); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("[%s] keys %v shown as %v\n  id:  %q\n  str: %q", lang, want, got, id, str)
			}
		}
	}
}

func TestParsePOPlural(t *testing.T) {
	_, pl := parsePO("msgid \"1 day\"\nmsgid_plural \"%d days\"\nmsgstr[0] \"1 Tag\"\nmsgstr[1] \"%d \"\n\"Tage\"\n\n" +
		"#, fuzzy\nmsgid \"1 hour\"\nmsgid_plural \"%d hours\"\nmsgstr[0] \"x\"\nmsgstr[1] \"y\"\n")
	if got := pl["1 day"]; len(got) != 2 || got[0] != "1 Tag" || got[1] != "%d Tage" {
		t.Errorf("plural forms = %q, want [1 Tag, %%d Tage]", got)
	}
	if _, ok := pl["1 hour"]; ok {
		t.Error("a fuzzy plural entry should be skipped")
	}
}

// The Russian rule is the one with three forms, so it is the one worth pinning:
// 1 and 21 take the first, 2-4 and 22 the second, 5-20 and 11-14 the third.
func TestPluralFormRussian(t *testing.T) {
	for n, want := range map[int64]int{1: 0, 21: 0, 101: 0, 2: 1, 4: 1, 22: 1, 5: 2, 11: 2, 12: 2, 14: 2, 20: 2, 0: 2, 111: 2} {
		if got := pluralForm("ru", n); got != want {
			t.Errorf("ru form for %d = %d, want %d", n, got, want)
		}
	}
	if pluralForm("de", 1) != 0 || pluralForm("de", 0) != 1 || pluralForm("de", 2) != 1 {
		t.Error("de should take form 0 for exactly one and form 1 otherwise")
	}
}

func TestTNFallsBackToEnglish(t *testing.T) {
	if got := TN("", "1 day", "%d days", 1); got != "1 day" {
		t.Errorf("English singular = %q", got)
	}
	if got := TN("de", "no such one", "no such many", 3); got != "no such many" {
		t.Errorf("untranslated plural = %q, want the English plural", got)
	}
}

// Every catalog's Plural-Forms header must state the nplurals its Go rule
// picks among, or msgmerge would hand translators the wrong number of forms.
func TestPluralRulesMatchTheCatalogs(t *testing.T) {
	entries, err := fs.ReadDir(locale, "locale")
	if err != nil {
		t.Fatal(err)
	}
	nplurals := regexp.MustCompile(`Plural-Forms: nplurals=(\d+);`)
	for _, e := range entries {
		lang := strings.TrimSuffix(e.Name(), ".po")
		raw, _ := locale.ReadFile("locale/" + e.Name())
		r, ok := pluralRules[lang]
		if !ok {
			t.Errorf("%s has no plural rule in pluralRules", lang)
			continue
		}
		m := nplurals.FindSubmatch(raw)
		if m == nil {
			t.Errorf("%s.po has no Plural-Forms header", lang)
			continue
		}
		if string(m[1]) != strconv.Itoa(r.forms) {
			t.Errorf("%s.po says nplurals=%s, pluralRules says %d", lang, m[1], r.forms)
		}
	}
}
