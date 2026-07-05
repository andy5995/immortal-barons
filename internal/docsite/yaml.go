package docsite

import (
	"sort"
	"strings"
)

func sortTopics(ts []topic) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Order != ts[j].Order {
			return ts[i].Order < ts[j].Order
		}
		return ts[i].relPath < ts[j].relPath
	})
}

func sortByRank(s []string, rank map[string]int) {
	sort.Slice(s, func(i, j int) bool { return rank[s[i]] < rank[s[j]] })
}

func sortStrings(s []string) { sort.Strings(s) }

func sortNav(n []navNode) {
	sort.Slice(n, func(i, j int) bool { return n[i].title < n[j].title })
}

// titleize turns a "some-file-name" slug into "Some File Name" (fallback nav
// label when a doc has no heading).
func titleize(s string) string {
	words := strings.Fields(strings.ReplaceAll(s, "-", " "))
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// mkdocsYAML renders the full mkdocs.yml: site/theme/plugins config plus the
// generated nav. The i18n plugin uses the "folder" layout (site-src/<lang>/)
// with English as the default and fallback.
func mkdocsYAML(nav []navNode) string {
	var b strings.Builder
	b.WriteString(`site_name: Immortal Barons
site_description: A from-scratch Go clone of the BBS door game Barren Realms Elite.
site_url: https://andy5995.github.io/immortal-barons/
repo_url: https://github.com/andy5995/immortal-barons
docs_dir: site-src
use_directory_urls: true

theme:
  name: material
  features:
    - navigation.instant
    - navigation.sections
    - navigation.top
    - search.suggest
    - content.code.copy
  palette:
    - scheme: default
      toggle:
        icon: material/weather-night
        name: Switch to dark mode
    - scheme: slate
      toggle:
        icon: material/weather-sunny
        name: Switch to light mode

plugins:
  - search
  - i18n:
      docs_structure: folder
      fallback_to_default: true
      languages:
`)
	for _, lang := range languages {
		b.WriteString("        - locale: " + lang.code + "\n")
		b.WriteString("          name: " + lang.name + "\n")
		if lang.isFirst {
			b.WriteString("          default: true\n")
		}
	}
	b.WriteString(`
markdown_extensions:
  - admonition
  - tables
  - toc:
      permalink: true
  - pymdownx.superfences

nav:
`)
	for _, n := range nav {
		emitNav(&b, n, 1)
	}
	return b.String()
}

// emitNav writes one nav node (and its subtree) as YAML at the given depth
// (each level = two spaces). Titles are quoted so a ':' in a title is safe.
func emitNav(b *strings.Builder, n navNode, depth int) {
	indent := strings.Repeat("  ", depth)
	if len(n.children) == 0 {
		b.WriteString(indent + "- " + quote(n.title) + ": " + n.path + "\n")
		return
	}
	b.WriteString(indent + "- " + quote(n.title) + ":\n")
	for _, c := range n.children {
		emitNav(b, c, depth+1)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
