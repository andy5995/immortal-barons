package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// helpBrowse is the categorized help browser: pick a category, then a topic,
// then read it. It replaces the old flat "pick 1-of-N topics" list. Content is
// the embedded single-source Markdown from internal/help (see
// docs/superpowers/specs/2026-07-03-docs-help-localization-design.md).
func helpBrowse(s session.Session, w *ctx) Result {
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
		browseCategory(s, cats[i-1], w.Player().Language)
	}
}

// helpLanguages are the languages the UI/help can render, in menu order: English
// (the canonical source, code "") followed by every translated language from the
// single source of truth in i18n.Languages. Endonyms (each language in its own
// tongue) read the same regardless of the current UI language, the standard for
// a language picker.
var helpLanguages = func() []struct{ code, name string } {
	ls := []struct{ code, name string }{{"", "English"}}
	for _, l := range i18n.Languages {
		ls = append(ls, struct{ code, name string }{l.Code, l.Name})
	}
	return ls
}()

// languageName is the endonym for a stored language code.
func languageName(code string) string {
	for _, l := range helpLanguages {
		if l.code == code {
			return l.name
		}
	}
	return "English"
}

// pickLanguage lets the caller choose the UI/help language, stored per-empire
// so each caller keeps their own. Partly-translated languages fall back to
// English per string.
func pickLanguage(s session.Session, w *ctx) Result {
	// Non-English text can't be shown over a CP437 session, so language
	// selection is only offered in UTF-8 mode.
	if !w.UTF8 {
		ok(s, "To enable language selection, this program must be run with -utf8")
		return Stay
	}
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Language:"), ansi.Reset)
	for i, l := range helpLanguages {
		fmt.Fprintf(s, "  %d) %s\n", i+1, l.name)
	}
	i := promptInt(s, "Choose (0 to keep current)?")
	if i < 1 || i > len(helpLanguages) {
		return Stay
	}
	p.Language = helpLanguages[i-1].code
	ok(s, "Language set to %s.", languageName(p.Language))
	return Stay
}

// browseCategory lists a category's topics and renders the chosen one, paged.
func browseCategory(s session.Session, cat, lang string) {
	for {
		topics := help.Topics(cat, lang)
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
