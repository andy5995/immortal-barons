package menu

import "testing"

// readKey must swallow terminal escape sequences (arrow keys, PgUp/PgDn, Home/
// End, function keys) so their bytes never leak into a menu selection or prompt.
func TestReadKeySwallowsEscapeSequences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want rune
	}{
		{"up-arrow", "\x1b[A5", '5'},
		{"down-arrow", "\x1b[B1", '1'},
		{"pgup", "\x1b[5~2", '2'},   // ESC [ 5 ~
		{"pgdn", "\x1b[6~3", '3'},   // ESC [ 6 ~
		{"home", "\x1b[H4", '4'},    // final byte 'H'
		{"ss3-arrow", "\x1bOA6", '6'}, // SS3 application-mode arrow
		{"plain-key", "q", 'q'},
	}
	for _, c := range cases {
		f := &fakeSession{keys: []rune(c.in)}
		got, err := readKey(f)
		if err != nil {
			t.Errorf("%s: readKey(%q) error: %v", c.name, c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: readKey(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
