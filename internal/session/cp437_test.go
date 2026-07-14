package session

import (
	"bytes"
	"io"
	"testing"
)

// fakeInner is a minimal Session backing the CP437 writer tests.
type fakeInner struct {
	out  bytes.Buffer
	keys []rune
	pos  int
}

func (f *fakeInner) Write(p []byte) (int, error) { return f.out.Write(p) }
func (f *fakeInner) ReadKey() (rune, error) {
	if f.pos >= len(f.keys) {
		return 0, io.EOF
	}
	r := f.keys[f.pos]
	f.pos++
	return r, nil
}

func TestCP437WriterTranscodes(t *testing.T) {
	inner := &fakeInner{}
	w := NewCP437Writer(inner)

	// ASCII passes through; the box/block glyphs map to their canonical CP437
	// bytes; a rune with no CP437 equivalent (Cyrillic) degrades to the
	// substitution byte 0x1a.
	cases := []struct {
		in   string
		want []byte
	}{
		{"AB", []byte{0x41, 0x42}},
		{"\x1b[31m", []byte{0x1b, '[', '3', '1', 'm'}}, // ANSI escape is ASCII
		{"█", []byte{0xDB}},                            // U+2588 full block
		{"─", []byte{0xC4}},                            // U+2500 light horizontal
		{"│", []byte{0xB3}},                            // U+2502 light vertical
		{"Я", []byte{0x1a}},                            // Cyrillic: unmappable -> SUB
	}
	for _, c := range cases {
		inner.out.Reset()
		if _, err := io.WriteString(w, c.in); err != nil {
			t.Fatalf("write %q: %v", c.in, err)
		}
		if got := inner.out.Bytes(); !bytes.Equal(got, c.want) {
			t.Errorf("encode %q = % x, want % x", c.in, got, c.want)
		}
	}
}

// Typographic characters with no CP437 form degrade to readable ASCII
// look-alikes (via cp437Fallback), not the substitution glyph.
func TestCP437WriterTypographicFallback(t *testing.T) {
	inner := &fakeInner{}
	w := NewCP437Writer(inner)

	if _, err := io.WriteString(w, "a—b–c…“d”‘e’"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got, want := inner.out.String(), `a--b-c..."d"'e'`; got != want {
		t.Errorf("fallback = %q, want %q", got, want)
	}
}

func TestCP437Encodable(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"plain ASCII", true},
		{"äöüß ñ é", true}, // German/Spanish letters exist in CP437
		{"█─│░▒▓", true},   // block/box glyphs
		{"Ж", false},       // Cyrillic
		{"—", false},       // em dash (the classic translator slip)
		{"…", false},       // ellipsis
		{"„deutsche Anführungszeichen“", false}, // low-9 quotes
	}
	for _, c := range cases {
		if got := CP437Encodable(c.in); got != c.want {
			t.Errorf("CP437Encodable(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAllCP437Encodable(t *testing.T) {
	if !AllCP437Encodable(nil) {
		t.Error("nil slice should be encodable (vacuously)")
	}
	if !AllCP437Encodable([]string{"abc", "äöü"}) {
		t.Error("all-safe slice should be encodable")
	}
	if AllCP437Encodable([]string{"abc", "Ж"}) {
		t.Error("one unmappable string should fail the whole slice")
	}
}

func TestCP437WriterReadKeyPassesThrough(t *testing.T) {
	inner := &fakeInner{keys: []rune("xy")}
	w := NewCP437Writer(inner)
	for _, want := range []rune{'x', 'y'} {
		if got, err := w.ReadKey(); got != want || err != nil {
			t.Errorf("ReadKey = %q, %v; want %q, nil", got, err, want)
		}
	}
}

func TestIsUTF8(t *testing.T) {
	plain := &fakeInner{}
	if !IsUTF8(plain) {
		t.Error("a session with no charset marker should be treated as UTF-8")
	}
	if IsUTF8(NewCP437Writer(plain)) {
		t.Error("a CP437 writer should report not-UTF-8")
	}
}

func TestLocaleIsUTF8(t *testing.T) {
	cases := []struct {
		lcAll, lcCtype, lang string
		want                 bool
	}{
		{"", "", "", false},                        // nothing set
		{"", "", "en_US.UTF-8", true},              // LANG UTF-8
		{"", "", "C", false},                       // LANG=C
		{"", "", "en_US.utf8", true},               // lowercase spelling
		{"", "en_US.UTF-8", "C", true},             // LC_CTYPE beats LANG
		{"de_DE.UTF-8", "C", "C", true},            // LC_ALL beats the rest
		{"C", "en_US.UTF-8", "en_US.UTF-8", false}, // LC_ALL=C wins
	}
	for _, c := range cases {
		t.Setenv("LC_ALL", c.lcAll)
		t.Setenv("LC_CTYPE", c.lcCtype)
		t.Setenv("LANG", c.lang)
		if got := LocaleIsUTF8(); got != c.want {
			t.Errorf("LC_ALL=%q LC_CTYPE=%q LANG=%q -> %v, want %v", c.lcAll, c.lcCtype, c.lang, got, c.want)
		}
	}
}
