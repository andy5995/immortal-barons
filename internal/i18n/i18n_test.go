package i18n

import (
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
