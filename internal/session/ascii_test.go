package session

import "testing"

func TestToASCII(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain text passes through", "Turns per day", "Turns per day"},
		{"box rules keep their shape", "┌───┬───┐", "+---+---+"},
		{"double rules", "╔═══╗", "+===+"},
		{"table divider", "Gold │ 1,200", "Gold | 1,200"},
		{"blocks and shades", "█▓▒░", "##:."},
		{"arrows", "A → B", "A -> B"},
		{"em dash and ellipsis", "Baron — wait…", "Baron -- wait..."},
		{"accents lose the accent, not the letter", "Übergröße", "Ubergrosse"},
		{"escapes are untouched", "\x1b[96mHelp\x1b[0m", "\x1b[96mHelp\x1b[0m"},
		{"no look-alike becomes a question mark", "Империя", "???????"},
	}
	for _, c := range cases {
		if got := ToASCII(c.in); got != c.want {
			t.Errorf("%s: ToASCII(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestASCIIWriterMarkers(t *testing.T) {
	c := &capture{}
	s := NewASCIIWriter(c)
	if _, err := s.Write([]byte("─ Gold ─")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := c.buf.String(); got != "- Gold -" {
		t.Errorf("wrote %q, want %q", got, "- Gold -")
	}
	if IsUTF8(s) {
		t.Error("an ASCII session is not UTF-8")
	}
	if !IsASCII(s) {
		t.Error("an ASCII session should advertise itself")
	}
	if IsASCII(c) {
		t.Error("a session that advertises nothing is not ASCII-only")
	}
	if HasANSI(NewPlain(NewASCIIWriter(c))) {
		t.Error("ASCII wrapping should not hide the no-ANSI marker")
	}
}

func TestASCIIEncodable(t *testing.T) {
	if !ASCIIEncodable("Straße") {
		t.Error("ß transliterates to ss, so German stays showable")
	}
	if !ASCIIEncodable("Deutsch — Übersicht") {
		t.Error("German text should survive the substitution")
	}
	if ASCIIEncodable("Русский") {
		t.Error("Cyrillic cannot be shown in ASCII")
	}
	if !ASCIIEncodable("Really?") {
		t.Error("a question mark of the string's own is not a substitution")
	}
}
