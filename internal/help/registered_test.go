package help

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

// Every language the game offers must have its help tree mapped here. One that
// is embedded but missing from translated is silently unused: its callers read
// the English topics and nothing fails. TestHelpTranslationParity checks the
// trees on disk; this checks that each one is wired in.
func TestEveryLanguageHasItsHelp(t *testing.T) {
	for _, code := range i18n.Codes() {
		if len(translated[code]) == 0 {
			t.Errorf("%s is in i18n.Languages but has no help tree in the translated map (help.go)", code)
		}
	}
}
