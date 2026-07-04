package i18n

import "testing"

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
