package docsite

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ghRepo   = "https://github.com/andy5995/immortal-barons"
	ghBranch = "trunk"
)

// linker rewrites intra-repo Markdown links so they work on the site: links to
// published pages become site-relative (MkDocs resolves the final URL), and
// links to files that stay in the repo become GitHub blob/tree URLs. External
// links and pure anchors are left untouched.
type linker struct {
	repoRoot string
	siteMap  map[string]string // normalized repo path -> language-relative site path (e.g. "guide/economy/regions.md")
}

// buildSiteMap maps each published source's repo path (help content normalized
// to its English path) to its language-relative site path.
func newLinker(repoRoot string, enTopics []topic) *linker {
	m := map[string]string{
		"README.md":          "index.md",
		"docs/playing.md":    "guide/index.md",
		"docs/door-setup.md": "door-setup/index.md",
		"docs/webserver.md":  "web-server/index.md",
		"docs/faq.md":        "faq/index.md",
	}
	for _, t := range enTopics {
		m["internal/help/content/"+t.relPath] = "guide/" + t.relPath
	}
	// Dev docs, English only.
	devRoot := filepath.Join(repoRoot, "docs", "dev")
	_ = filepath.WalkDir(devRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, p)
		sub, _ := filepath.Rel(devRoot, p)
		m[filepath.ToSlash(rel)] = "developers/" + filepath.ToSlash(sub)
		return nil
	})
	return &linker{repoRoot: repoRoot, siteMap: m}
}

// mdLink matches a Markdown inline link's target: [text](target).
var mdLink = regexp.MustCompile(`\]\(([^)]+)\)`)

// normContent collapses a translated help path (content.de/…, content.ru/…) to
// its English content path so links resolve to one site page.
func normContent(repoRel string) string {
	for _, lang := range []string{"de", "ru"} {
		repoRel = strings.Replace(repoRel, "internal/help/content."+lang+"/", "internal/help/content/", 1)
	}
	return repoRel
}

// rewrite rewrites the links in md. srcRepoRel is the file's repo path;
// siteRel is its language-relative site path (used to compute relative links).
func (lk *linker) rewrite(md, srcRepoRel, siteRel string) string {
	srcDir := path.Dir(filepath.ToSlash(srcRepoRel))
	siteDir := path.Dir(siteRel)
	return mdLink.ReplaceAllStringFunc(md, func(m string) string {
		target := m[2 : len(m)-1] // strip "](" and ")"
		if target == "" || strings.HasPrefix(target, "#") ||
			strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			return m
		}
		link, anchor := target, ""
		if i := strings.IndexByte(target, '#'); i >= 0 {
			link, anchor = target[:i], target[i:]
		}
		// Resolve the link relative to the source file's directory.
		repoRel := normContent(path.Clean(path.Join(srcDir, link)))
		if dst, ok := lk.siteMap[repoRel]; ok {
			rel := relSitePath(siteDir, dst)
			return "](" + rel + anchor + ")"
		}
		// Not published: point at the file (or dir) on GitHub.
		kind := "blob"
		if info, err := os.Stat(filepath.Join(lk.repoRoot, filepath.FromSlash(repoRel))); err == nil && info.IsDir() {
			kind = "tree"
		}
		return "](" + ghRepo + "/" + kind + "/" + ghBranch + "/" + repoRel + anchor + ")"
	})
}

// relSitePath is the path to dst as seen from a page in siteDir (both
// language-relative). MkDocs turns the .md target into the final URL.
func relSitePath(siteDir, dst string) string {
	if siteDir == "." || siteDir == "" {
		return dst
	}
	rel, err := filepath.Rel(siteDir, dst)
	if err != nil {
		return dst
	}
	return filepath.ToSlash(rel)
}
