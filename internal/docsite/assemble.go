// Package docsite assembles the documentation website's MkDocs source tree from
// the repo's committed Markdown, so the site and the in-game help share one
// source of truth. It is build-time tooling (run by cmd/barons-docs in CI); the
// game does not import it.
//
// Output: <outDir>/site-src/<lang>/** (the mkdocs-static-i18n "folder" layout,
// English default + de/ru overlays) and <outDir>/mkdocs.yml. Missing
// translations fall back to English via the i18n plugin, so a language folder
// need only hold the pages that are actually translated.
package docsite

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/i18n"
)

// extraCSS makes in-content links obviously links. Material for MkDocs only
// colors body links by default (no underline), so a link in a sentence is easy
// to miss; underline them and give them a clearer color.
const extraCSS = `/* Underline in-content links so they read as links, not just tinted text. */
.md-typeset a {
  text-decoration: underline;
  text-underline-offset: 0.15em;
}
.md-typeset a:hover {
  text-decoration: underline;
}
`

// siteLang is one language column of the site: its code, its endonym (shown in
// the language switcher), and whether it is the default/fallback.
type siteLang struct {
	code    string
	name    string
	isFirst bool
}

// languages are the site's languages in menu order; English is the default and
// the fallback for untranslated pages, followed by every translated language
// from the single source of truth in i18n.Languages.
var languages = func() []siteLang {
	ls := []siteLang{{"en", "English", true}}
	for _, l := range i18n.Languages {
		ls = append(ls, siteLang{l.Code, l.Name, false})
	}
	return ls
}()

// Assemble reads the doc sources under repoRoot and writes the MkDocs source
// tree + mkdocs.yml under outDir (which is cleared first).
func Assemble(repoRoot, outDir string) error {
	siteSrc := filepath.Join(outDir, "site-src")
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(siteSrc, 0o755); err != nil {
		return err
	}

	// English help topics define the Guide's structure (categories/order); all
	// languages reuse it, with each language's translated body where present.
	enTopics, err := loadTopics(repoRoot, "en")
	if err != nil {
		return err
	}
	lk := newLinker(repoRoot, enTopics)
	srcRel := func(abs string) string { r, _ := filepath.Rel(repoRoot, abs); return filepath.ToSlash(r) }

	for _, lang := range languages {
		langDir := filepath.Join(siteSrc, lang.code)
		// Home (README) and the two doc pages: only if this language has them.
		for _, pg := range []struct{ src, dst, siteRel string }{
			{homeSource(repoRoot, lang.code), filepath.Join(langDir, "index.md"), "index.md"},
			{guideIntroSource(repoRoot, lang.code), filepath.Join(langDir, "guide", "index.md"), "guide/index.md"},
			{doorSetupSource(repoRoot, lang.code), filepath.Join(langDir, "door-setup", "index.md"), "door-setup/index.md"},
			{webserverSource(repoRoot, lang.code), filepath.Join(langDir, "web-server", "index.md"), "web-server/index.md"},
			{charsetSource(repoRoot, lang.code), filepath.Join(langDir, "charset", "index.md"), "charset/index.md"},
			{faqSource(repoRoot, lang.code), filepath.Join(langDir, "faq", "index.md"), "faq/index.md"},
			{translatingSource(repoRoot, lang.code), filepath.Join(langDir, "translating", "index.md"), "translating/index.md"},
		} {
			if err := copyIfExists(pg.src, pg.dst, srcRel(pg.src), pg.siteRel, lk); err != nil {
				return err
			}
		}

		// Help topics for this language (may be a partial set; the rest fall
		// back to English).
		topics, err := loadTopics(repoRoot, lang.code)
		if err != nil {
			return err
		}
		for _, t := range topics {
			dst := filepath.Join(langDir, "guide", filepath.FromSlash(t.relPath))
			if err := copyMarkdown(t.absPath, dst, srcRel(t.absPath), "guide/"+t.relPath, lk); err != nil {
				return err
			}
		}
	}

	// Developer docs are English-only.
	if err := copyTree(filepath.Join(repoRoot, "docs", "dev"), filepath.Join(siteSrc, "en", "developers"), repoRoot, "developers", lk); err != nil {
		return err
	}

	// Custom CSS lives in the default-language folder (served at the site root,
	// so all languages pick it up).
	cssPath := filepath.Join(siteSrc, "en", "stylesheets", "extra.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cssPath, []byte(extraCSS), 0o644); err != nil {
		return err
	}

	nav, err := buildNav(repoRoot, enTopics)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "mkdocs.yml"), []byte(mkdocsYAML(nav)), 0o644)
}

// topic is a help article located on disk, with its path relative to the
// language's content root (e.g. "economy/regions.md").
type topic struct {
	help.Topic
	relPath string
	absPath string
}

// contentDir is the on-disk help content root for a language.
func contentDir(repoRoot, lang string) string {
	base := filepath.Join(repoRoot, "internal", "help", "content")
	if lang == "en" {
		return base
	}
	return base + "." + lang
}

// loadTopics walks a language's help content, parsing each topic's frontmatter
// with the same parser the game uses.
func loadTopics(repoRoot, lang string) ([]topic, error) {
	root := contentDir(repoRoot, lang)
	var topics []topic
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		topics = append(topics, topic{
			Topic:   help.ParseTopic(string(raw)),
			relPath: filepath.ToSlash(rel),
			absPath: path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].relPath < topics[j].relPath })
	return topics, nil
}

// homeSource / guideIntroSource / doorSetupSource / webserverSource resolve a
// doc's file for a language: <base>.md for English, <base>.<lang>.md otherwise.
func homeSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "README.md"), lang)
}
func guideIntroSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "playing.md"), lang)
}
func doorSetupSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "door-setup.md"), lang)
}
func webserverSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "webserver.md"), lang)
}
func charsetSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "charset.md"), lang)
}
func faqSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "faq.md"), lang)
}
func translatingSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "translating.md"), lang)
}

func langFile(enPath, lang string) string {
	if lang == "en" {
		return enPath
	}
	return strings.TrimSuffix(enPath, ".md") + "." + lang + ".md"
}
