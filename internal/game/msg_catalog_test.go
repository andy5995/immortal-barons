package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

// engineTemplates is every template this package writes, read from its source
// with the markers scripts/gen-ui-pot.py extracts by, from the one file both
// read: singles, and counted pairs (one, many).
func engineTemplates(t *testing.T) (singles []string, pairs [][2]string) {
	t.Helper()
	raw, err := os.ReadFile("../../scripts/engine-msg-markers.json")
	if err != nil {
		t.Fatal(err)
	}
	var markers struct{ Single, Pair []string }
	if err := json.Unmarshal(raw, &markers); err != nil {
		t.Fatal(err)
	}
	str := `"((?:[^"\\]|\\.)*)"`
	single := regexp.MustCompile(`\b(?:` + strings.Join(markers.Single, "|") + `)\(\s*` + str)
	pair := regexp.MustCompile(`(?s)\b(?:` + strings.Join(markers.Pair, "|") + `)\(\s*` + str + `,\s*` + str)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range single.FindAllStringSubmatch(string(raw), -1) {
			singles = append(singles, m[1])
		}
		for _, m := range pair.FindAllStringSubmatch(string(raw), -1) {
			pairs = append(pairs, [2]string{m[1], m[2]})
		}
	}
	if len(singles) < 100 || len(pairs) < 20 {
		t.Fatalf("found %d templates and %d pairs; the scan is not reading the source", len(singles), len(pairs))
	}
	return singles, pairs
}

var placeholder = regexp.MustCompile(`\{[a-z]+\}`)

func placeholders(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range placeholder.FindAllString(s, -1) {
		out[p] = true
	}
	return out
}

// A translation of an engine message may move its placeholders but must keep
// every one, and add none: the values go in after translation (#297), so a
// dropped {who} silently loses a realm's name and an invented one prints braces
// to the player. A plural form may leave the count out ("One of them") but may
// use nothing the English pair does not.
func TestCatalogsKeepEnginePlaceholders(t *testing.T) {
	singles, pairs := engineTemplates(t)
	for _, l := range i18n.Languages {
		lang := l.Code
		if lang == "en" {
			continue
		}
		for _, id := range singles {
			want, got := placeholders(id), placeholders(i18n.T(lang, id))
			for p := range want {
				if !got[p] {
					t.Errorf("[%s] %q drops %s: %q", lang, id, p, i18n.T(lang, id))
				}
			}
			for p := range got {
				if !want[p] {
					t.Errorf("[%s] %q adds %s: %q", lang, id, p, i18n.T(lang, id))
				}
			}
		}
		for _, pr := range pairs {
			allowed := placeholders(pr[0] + pr[1])
			for _, n := range []int64{0, 1, 2, 5, 21} {
				form := i18n.TN(lang, pr[0], pr[1], n)
				for p := range placeholders(form) {
					if !allowed[p] {
						t.Errorf("[%s] %q (n=%d) adds %s: %q", lang, pr[0], n, p, form)
					}
				}
			}
		}
	}
}
