// Package help is the categorized, embedded help system for Immortal Barons.
//
// Player-facing help lives as Markdown files under
// content/<category>/<topic>.md, each with a small frontmatter block
// (title/category/order/in_game). That Markdown is the SINGLE SOURCE OF TRUTH:
// the game embeds and renders it to ANSI here (see render.go), and the
// user-facing web docs are assembled from the same files and translated with
// po4a. Design: docs/superpowers/specs/2026-07-03-docs-help-localization-design.md
//
// Why the content lives under this package instead of docs/help/: Go's
// //go:embed can only reach files at or below the embedding package's
// directory, so the source tree lives here and the web/po4a tooling reads it
// from internal/help/content/.
package help

import (
	"embed"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// content embeds the English source (content/) plus the po4a-generated
// per-language trees (content.<lang>/). All three are committed; the language
// dirs are regenerated from po/help/*.po by scripts/gen-help-translations.sh.
//
//go:embed content content.de content.ru
var content embed.FS

// Topic is one help article, parsed from a content/<category>/<file>.md file.
type Topic struct {
	Title    string // human title, from frontmatter
	Category string // category slug, from frontmatter
	Order    int    // sort order within the category
	InGame   bool   // show in the in-game browser (docs-only topics set false)
	Body     string // the Markdown body, frontmatter stripped

	path string // relative path under the content dir (e.g. "controls/interface.md")
}

// categoryOrder fixes the display order of the known categories; any category
// not listed here sorts after these, alphabetically. Matches the spec's set.
var categoryOrder = []string{
	"controls", "military", "economy", "warfare", "diplomacy", "interbbs",
}

// CategoryOrder returns the fixed display order of the known category slugs, so
// the docs-site assembler orders its Guide sidebar the same way the in-game help
// does.
func CategoryOrder() []string { return append([]string(nil), categoryOrder...) }

// categoryNames maps a category slug to its display name (a few need casing
// the slug can't carry, like inter-bbs).
var categoryNames = map[string]string{
	"controls":  "Controls",
	"military":  "Military",
	"economy":   "Economy",
	"warfare":   "Warfare",
	"diplomacy": "Diplomacy",
	"interbbs":  "Inter-BBS",
}

// all is the English topic set (the canonical structure), loaded once at init.
// translated indexes each language's topics by relative path, so a topic can be
// localized in place. Loading at init keeps callers simple (no error path) and
// surfaces a malformed committed file immediately as a panic during startup.
var (
	all        = loadDir("content")
	translated = map[string]map[string]Topic{
		"de": indexByPath(loadDir("content.de")),
		"ru": indexByPath(loadDir("content.ru")),
	}
)

// loadDir walks one embedded content root and parses every .md file, recording
// each topic's path relative to that root. A walk/read failure is a programming
// error (content is embedded at build time) — hence the panic.
func loadDir(root string) []Topic {
	var topics []Topic
	err := fs.WalkDir(content, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := content.ReadFile(path)
		if err != nil {
			return err
		}
		t := parseTopic(string(raw))
		t.path = strings.TrimPrefix(path, root+"/")
		topics = append(topics, t)
		return nil
	})
	if err != nil {
		panic("help: loading embedded content: " + err.Error())
	}
	return topics
}

func indexByPath(topics []Topic) map[string]Topic {
	idx := make(map[string]Topic, len(topics))
	for _, t := range topics {
		idx[t.path] = t
	}
	return idx
}

// localize returns t with its Title and Body swapped for the given language's
// translation when one exists; the structure (category, order, in_game) always
// comes from the English canonical topic. An empty or unknown lang yields
// English, as does a topic with no translation on file.
func localize(t Topic, lang string) Topic {
	if idx, ok := translated[lang]; ok {
		if tr, ok := idx[t.path]; ok {
			t.Title = tr.Title
			t.Body = tr.Body
		}
	}
	return t
}

// ParseTopic parses a topic file's raw text (frontmatter + body) into a Topic.
// Exported for the docs-site assembler (cmd/barons-docs) so "what a topic is"
// has one definition shared with the in-game help.
func ParseTopic(raw string) Topic { return parseTopic(raw) }

// parseTopic splits a "---\nkey: value\n...\n---\nbody" file into a Topic. The
// frontmatter is a tiny flat key:value subset (NOT full YAML), so we need no
// dependency. Recognized keys: title, category, order, in_game. A file with no
// frontmatter is treated as an all-body, in-game topic with an empty title.
func parseTopic(raw string) Topic {
	t := Topic{InGame: true}
	body := raw
	if strings.HasPrefix(raw, "---") {
		rest := strings.TrimPrefix(raw, "---")
		// The frontmatter ends at the next line that is exactly "---".
		if i := strings.Index(rest, "\n---"); i >= 0 {
			front := rest[:i]
			body = strings.TrimPrefix(rest[i+len("\n---"):], "\n")
			for _, line := range strings.Split(front, "\n") {
				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				switch strings.TrimSpace(key) {
				case "title":
					t.Title = unquote(strings.TrimSpace(val))
				case "category":
					t.Category = unquote(strings.TrimSpace(val))
				case "order":
					t.Order, _ = strconv.Atoi(strings.TrimSpace(val))
				case "in_game":
					t.InGame = strings.TrimSpace(val) == "true"
				}
			}
		}
	}
	t.Body = strings.TrimSpace(body)
	return t
}

// unquote strips a single pair of matching surrounding quotes, which po4a adds
// around YAML frontmatter values when it regenerates a translated file.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// CategoryName returns a category slug's display name, or the slug unchanged
// if it has no mapping.
func CategoryName(slug string) string {
	if n, ok := categoryNames[slug]; ok {
		return n
	}
	return slug
}

// Categories returns the category slugs that have at least one in-game topic,
// in display order: known categories first (per categoryOrder), then any
// others alphabetically.
func Categories() []string {
	seen := map[string]bool{}
	for _, t := range all {
		if t.InGame {
			seen[t.Category] = true
		}
	}
	rank := map[string]int{}
	for i, c := range categoryOrder {
		rank[c] = i
	}
	var known, extra []string
	for c := range seen {
		if _, ok := rank[c]; ok {
			known = append(known, c)
		} else {
			extra = append(extra, c)
		}
	}
	sort.Slice(known, func(i, j int) bool { return rank[known[i]] < rank[known[j]] })
	sort.Strings(extra)
	return append(known, extra...)
}

// Topics returns the in-game topics in a category, localized to lang (English
// for "" or an unknown lang), sorted by Order then Title. Order comes from the
// English canonical frontmatter, so the sort is stable across languages.
func Topics(category, lang string) []Topic {
	var out []Topic
	for _, t := range all {
		if t.InGame && t.Category == category {
			out = append(out, localize(t, lang))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].path < out[j].path // language-independent tie-break
	})
	return out
}
