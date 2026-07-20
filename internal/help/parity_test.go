package help

import (
	"io/fs"
	"strings"
	"testing"
)

// TestHelpTranslationParity guards against structural translation drift: every
// English help topic must have a counterpart in each language tree, and vice
// versa (no orphans). This is what a topic add/move/remove without regenerating
// leaves behind — e.g. introduction/overview.md once existed in English but was
// never wired into po4a.cfg, so no translated file was produced.
//
// It runs in the normal test suite (so also in CI), needs no po4a, and can't
// produce version-dependent false positives the way a po4a regen-and-diff check
// would. It checks structure, not translation freshness: a present-but-stale
// translation falls back to English and is a separate (deliberately deferred)
// concern.
func TestHelpTranslationParity(t *testing.T) {
	en := topicPathSet(t, "content")
	if len(en) == 0 {
		t.Fatal("no English topics found under content/")
	}
	for _, root := range []string{"content.de", "content.ru"} {
		tr := topicPathSet(t, root)
		for p := range en {
			if !tr[p] {
				t.Errorf("%s: no translated file for %q — run scripts/gen-help-translations.sh and commit", root, p)
			}
		}
		for p := range tr {
			if !en[p] {
				t.Errorf("%s: %q is an orphan (no English source) — run scripts/gen-help-translations.sh and commit", root, p)
			}
		}
	}
}

// topicPathSet collects the language-relative paths of every .md topic under an
// embedded content root (e.g. "regions/regions.md").
func topicPathSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	paths := map[string]bool{}
	err := fs.WalkDir(content, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			paths[strings.TrimPrefix(p, root+"/")] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return paths
}
