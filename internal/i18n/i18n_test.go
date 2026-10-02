package i18n

import (
	"io/fs"
	"regexp"
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
	m := parsePO("msgid \"a\"\nmsgstr \"\"\n\"b\"\n\nmsgid \"c\"\nmsgstr \"d\"\n")
	if m["a"] != "b" {
		t.Errorf("continuation msgstr = %q, want b", m["a"])
	}
	if m["c"] != "d" {
		t.Errorf("simple msgstr = %q, want d", m["c"])
	}
}

func TestParsePOSkipsHeaderAndEmpty(t *testing.T) {
	m := parsePO("msgid \"\"\nmsgstr \"Language: de\"\n\nmsgid \"x\"\nmsgstr \"\"\n")
	if _, ok := m[""]; ok {
		t.Error("header (empty msgid) should be skipped")
	}
	if _, ok := m["x"]; ok {
		t.Error("untranslated (empty msgstr) should be skipped")
	}
}

func TestParsePOSkipsFuzzy(t *testing.T) {
	m := parsePO("#, fuzzy\nmsgid \"x\"\nmsgstr \"y\"\n\nmsgid \"z\"\nmsgstr \"w\"\n")
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
