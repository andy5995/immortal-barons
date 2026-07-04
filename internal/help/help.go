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

//go:embed content
var content embed.FS

// Topic is one help article, parsed from a content/<category>/<file>.md file.
type Topic struct {
	Title    string // human title, from frontmatter
	Category string // category slug, from frontmatter
	Order    int    // sort order within the category
	InGame   bool   // show in the in-game browser (docs-only topics set false)
	Body     string // the Markdown body, frontmatter stripped
}

// categoryOrder fixes the display order of the known categories; any category
// not listed here sorts after these, alphabetically. Matches the spec's set.
var categoryOrder = []string{
	"controls", "military", "economy", "warfare", "diplomacy", "interbbs",
}

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

// all is the parsed topic set, loaded once at package init from the embedded
// files. Loading at init keeps callers simple (no error path) and surfaces a
// malformed committed file immediately as a panic during startup/tests.
var all = mustLoad()

// mustLoad walks the embedded content tree and parses every .md file. The
// content is embedded at build time, so a walk/read failure is a programming
// error, not a runtime condition — hence the panic.
func mustLoad() []Topic {
	var topics []Topic
	err := fs.WalkDir(content, "content", func(path string, d fs.DirEntry, err error) error {
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
		topics = append(topics, parseTopic(string(raw)))
		return nil
	})
	if err != nil {
		panic("help: loading embedded content: " + err.Error())
	}
	return topics
}

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
					t.Title = strings.TrimSpace(val)
				case "category":
					t.Category = strings.TrimSpace(val)
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

// Topics returns the in-game topics in a category, sorted by Order then Title.
func Topics(category string) []Topic {
	var out []Topic
	for _, t := range all {
		if t.InGame && t.Category == category {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Title < out[j].Title
	})
	return out
}
