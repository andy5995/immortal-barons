package session

import "testing"

func TestStripEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no escapes", "Configuration Editor", "Configuration Editor"},
		{"sgr color", "\x1b[94mTax Rate\x1b[0m", "Tax Rate"},
		{"truecolor sgr", "\x1b[38;2;135;206;235mSky\x1b[m", "Sky"},
		{"cursor position", "\x1b[4;11HRegions", "Regions"},
		{"clear screen", "\x1b[2J\x1b[HGold", "Gold"},
		{"two-byte escape", "\x1b(BPlain", "Plain"},
		{"truncated csi", "Land\x1b[38;", "Land"},
		{"lone escape at end", "Land\x1b", "Land"},
		{"crlf preserved", "\x1b[97;40mA\r\nB", "A\r\nB"},
	}
	for _, c := range cases {
		if got := string(stripEscapes([]byte(c.in))); got != c.want {
			t.Errorf("%s: stripEscapes(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
