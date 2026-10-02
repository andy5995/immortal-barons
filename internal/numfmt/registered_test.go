package numfmt

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

// Every language the game offers must name its thousands separator. A missing
// one falls back to the English comma without failing, which is how Dutch
// printed "10,000 goud" until 2026-09-17.
func TestEveryLanguageHasASeparator(t *testing.T) {
	for _, code := range i18n.Codes() {
		if _, ok := groupSep[code]; !ok {
			t.Errorf("%s is in i18n.Languages but has no entry in groupSep", code)
		}
	}
}
