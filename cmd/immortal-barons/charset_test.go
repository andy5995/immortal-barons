package main

import "testing"

func TestCharsetNamed(t *testing.T) {
	for _, tc := range []struct {
		name string
		want charset
		ok   bool
	}{
		{"UTF-8", charsetUTF8, true},
		{"utf8", charsetUTF8, true},
		{"IBM437", charsetCP437, true},
		{"cp437", charsetCP437, true},
		{"437", charsetCP437, true},
		{"US-ASCII", charsetASCII, true},
		{"ANSI_X3.4-1968", charsetASCII, true},
		// An encoding the door cannot send degrades to ASCII, not to the CP437
		// default: CP437 on a Latin-1 terminal garbles every box rule, while
		// ASCII look-alikes read correctly on any of them.
		{"ISO-8859-1", charsetASCII, true},
		{"IBM866", charsetASCII, true},
		// Empty is every format except BBSDEV.DRP, and means "the door decides".
		{"", 0, false},
	} {
		got, ok := charsetNamed(tc.name)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("charsetNamed(%q) = %v,%v; want %v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
