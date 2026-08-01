package docsite

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/help"
)

// copyMarkdown reads a Markdown file, rewrites its intra-repo links for the
// site (see linker), and writes it to dst (creating parent dirs). srcRepoRel is
// the file's repo path; siteRel is its language-relative site path.
func copyMarkdown(src, dst, srcRepoRel, siteRel string, lk *linker) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out := lk.rewrite(string(data), srcRepoRel, siteRel)
	return os.WriteFile(dst, []byte(out), 0o644)
}

// copyIfExists copies src to dst when src exists; a missing src is not an error
// (that language simply falls back to English for this page).
func copyIfExists(src, dst, srcRepoRel, siteRel string, lk *linker) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return copyMarkdown(src, dst, srcRepoRel, siteRel, lk)
}

// copyTree copies every .md under srcDir to dstDir (site path siteBase/…),
// preserving structure.
func copyTree(srcDir, dstDir, repoRoot, siteBase string, lk *linker) error {
	if _, err := os.Stat(srcDir); err != nil {
		return nil // no such source tree (e.g. no dev docs) — skip
	}
	return filepath.WalkDir(srcDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		srcRepoRel, _ := filepath.Rel(repoRoot, p)
		siteRel := siteBase + "/" + filepath.ToSlash(rel)
		return copyMarkdown(p, filepath.Join(dstDir, rel), filepath.ToSlash(srcRepoRel), siteRel, lk)
	})
}

// navNode is one entry in the MkDocs nav: a leaf (title + path) or a branch
// (title + children).
type navNode struct {
	title    string
	path     string
	children []navNode
}

// buildNav assembles the site nav from the English help topics (which fix the
// section's structure) plus the fixed Home / Door Setup / Developers sections.
// The topic section is titled "Game Instructions" to match what the player sees
// in-game: showInstructions renders these exact topics, and it is BRE's own
// opening-menu wording. Nav paths are relative to a language folder; the i18n plugin
// resolves them per language.
func buildNav(repoRoot string, enTopics []topic) ([]navNode, error) {
	nav := []navNode{{title: "Home", path: "index.md"}}
	// FAQ sits right after Home so it is easy to find in the nav/tabs.
	nav = append(nav, navNode{title: "FAQ", path: "faq/index.md"})
	// Command Reference next: it is the page a returning sysop wants fastest,
	// so it sits above the long Game Instructions section rather than below it.
	nav = append(nav, navNode{title: "Command Reference", path: "command-reference/index.md"})
	nav = append(nav, navNode{title: "Download", path: "download/index.md"})

	// Game Instructions: an overview, then a group per category in the help's
	// fixed order. It is the one deep section, so it collapses in the sidebar.
	guide := []navNode{{title: "Overview", path: "guide/index.md"}}
	byCat := map[string][]topic{}
	for _, t := range enTopics {
		byCat[t.Category] = append(byCat[t.Category], t)
	}
	for _, cat := range orderedCategories(enTopics) {
		ts := byCat[cat]
		sortTopics(ts)
		var leaves []navNode
		for _, t := range ts {
			leaves = append(leaves, navNode{title: t.Title, path: "guide/" + t.relPath})
		}
		guide = append(guide, navNode{title: help.CategoryName(cat), children: leaves})
	}
	nav = append(nav, navNode{title: "Game Instructions", children: guide})

	nav = append(nav, navNode{title: "Door Setup", path: "door-setup/index.md"})
	nav = append(nav, navNode{title: "Character Set", path: "charset/index.md"})
	nav = append(nav, navNode{title: "Translating", path: "translating/index.md"})

	// Developers: one leaf per English dev doc, titled from its first heading.
	dev, err := devNav(repoRoot)
	if err != nil {
		return nil, err
	}
	if len(dev) > 0 {
		nav = append(nav, navNode{title: "Developers", children: dev})
	}
	return nav, nil
}

// orderedCategories returns the categories present, known ones first in the
// help's fixed order, then any extras alphabetically.
func orderedCategories(topics []topic) []string {
	present := map[string]bool{}
	for _, t := range topics {
		present[t.Category] = true
	}
	rank := map[string]int{}
	order := help.CategoryOrder()
	for i, c := range order {
		rank[c] = i
	}
	var known, extra []string
	for c := range present {
		if _, ok := rank[c]; ok {
			known = append(known, c)
		} else {
			extra = append(extra, c)
		}
	}
	sortByRank(known, rank)
	sortStrings(extra)
	return append(known, extra...)
}

// devNav lists the English developer docs as nav leaves, titled from the first
// Markdown heading (falling back to the filename).
func devNav(repoRoot string) ([]navNode, error) {
	srcDir := filepath.Join(repoRoot, "docs", "dev")
	if _, err := os.Stat(srcDir); err != nil {
		return nil, nil
	}
	var out []navNode
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		out = append(out, navNode{
			title: firstHeading(string(raw), rel),
			path:  "developers/" + filepath.ToSlash(rel),
		})
		return nil
	})
	sortNav(out)
	return out, err
}

// firstHeading returns the text of the first "# " heading, or a title derived
// from the filename if there is none.
func firstHeading(md, relPath string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return titleize(strings.TrimSuffix(filepath.Base(relPath), ".md"))
}
