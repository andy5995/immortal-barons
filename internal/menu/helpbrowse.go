package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

// helpBrowse is the categorized help browser: pick a category, then a topic,
// then read it. It replaces the old flat "pick 1-of-N topics" list. Content is
// the embedded single-source Markdown from internal/help (see
// docs/superpowers/specs/2026-07-03-docs-help-localization-design.md).
func helpBrowse(s session.Session, w *game.World) Result {
	for {
		cats := help.Categories()
		fmt.Fprintf(s, "\n%sHelp — choose a category:%s\n", ansi.FgBrightCyan, ansi.Reset)
		for i, c := range cats {
			fmt.Fprintf(s, "  %d) %s\n", i+1, help.CategoryName(c))
		}
		i := promptInt(s, "Category (0 to leave)?")
		if i < 1 || i > len(cats) {
			return Stay
		}
		browseCategory(s, cats[i-1])
	}
}

// browseCategory lists a category's topics and renders the chosen one, paged.
func browseCategory(s session.Session, cat string) {
	for {
		topics := help.Topics(cat)
		fmt.Fprintf(s, "\n%s%s — choose a topic:%s\n", ansi.FgBrightCyan, help.CategoryName(cat), ansi.Reset)
		for i, t := range topics {
			fmt.Fprintf(s, "  %d) %s\n", i+1, t.Title)
		}
		i := promptInt(s, "Topic (0 to go back)?")
		if i < 1 || i > len(topics) {
			return
		}
		fmt.Fprintf(s, "\n%s\n", topics[i-1].RenderANSI(78))
		pause(s)
	}
}
