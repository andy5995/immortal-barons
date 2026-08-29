package i18n

import "testing"

func TestMatchTag(t *testing.T) {
	for _, tc := range []struct{ tag, want string }{
		{"de", "de"},
		{"de-DE", "de"},
		{"de-AT", "de"}, // the catalogs are not regional
		{"DE", "de"},    // BCP 47 comparison is case-insensitive
		{"ru-RU", "ru"},
		{"nl", "nl"},
		{"en-US", ""}, // English is the default, not a catalog
		{"fr", ""},    // no catalog shipped
		{"", ""},
		{"  de-DE  ", "de"},
	} {
		if got := MatchTag(tc.tag); got != tc.want {
			t.Errorf("MatchTag(%q) = %q, want %q", tc.tag, got, tc.want)
		}
	}
}
