package docsite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteLinks(t *testing.T) {
	root := t.TempDir()
	// An off-site file and an off-site directory for GitHub-URL rewriting.
	os.WriteFile(filepath.Join(root, "LICENSE"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(root, "docs", "superpowers", "specs"), 0o755)

	lk := &linker{
		repoRoot: root,
		siteMap: map[string]string{
			"README.md":           "index.md",
			"docs/sysop-guide.md": "running-a-board/index.md",
			"internal/help/content/economy/regions.md": "guide/economy/regions.md",
			"internal/help/content/warfare/attacks.md": "guide/warfare/attacks.md",
		},
	}

	tests := []struct {
		name, md, srcRepoRel, siteRel, want string
	}{
		{"published from Home", "[s](docs/sysop-guide.md)", "README.md", "index.md",
			"[s](running-a-board/index.md)"},
		{"off-site file", "[l](LICENSE)", "README.md", "index.md",
			"[l](https://github.com/andy5995/immortal-barons/blob/trunk/LICENSE)"},
		{"off-site dir", "[d](docs/superpowers/specs/)", "README.md", "index.md",
			"[d](https://github.com/andy5995/immortal-barons/tree/trunk/docs/superpowers/specs)"},
		{"external untouched", "[e](https://example.com)", "README.md", "index.md",
			"[e](https://example.com)"},
		{"anchor untouched", "[a](#heritage)", "README.md", "index.md",
			"[a](#heritage)"},
		{"published keeps anchor", "[a](docs/sysop-guide.md#reset)", "README.md", "index.md",
			"[a](running-a-board/index.md#reset)"},
		{"relative between topics", "[a](../warfare/attacks.md)", "internal/help/content/economy/regions.md", "guide/economy/regions.md",
			"[a](../warfare/attacks.md)"},
		{"translated topic resolves to English page", "[a](../warfare/attacks.md)", "internal/help/content.de/economy/regions.md", "guide/economy/regions.md",
			"[a](../warfare/attacks.md)"},
	}
	for _, tc := range tests {
		if got := lk.rewrite(tc.md, tc.srcRepoRel, tc.siteRel); got != tc.want {
			t.Errorf("%s:\n  got  %s\n  want %s", tc.name, got, tc.want)
		}
	}
}
