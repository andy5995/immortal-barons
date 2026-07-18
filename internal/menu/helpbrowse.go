package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
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
		// About lives here (keyed 'A') rather than on the Game/System menus, so 'I'
		// can stay BRE's InterBBS Scores key (#17 menu audit). Categories stay
		// numbered; only About is lettered. A 0) Quit line closes the list like the
		// other menus.
		fmt.Fprintf(s, "  A) %s\n", tr(s, "About"))
		fmt.Fprintf(s, "  0) %s\n", tr(s, "Quit"))
		fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Choice?"), ansi.Reset)
		// Single keypress, no Enter (like the other menus). Categories are numbered
		// 1..N (7 today, always <10), plus 'A' About and '0' Quit; ignore any other
		// key and keep waiting.
		var sel rune
		for {
			r, err := readKey(s)
			if err != nil {
				return Stay
			}
			if r == '0' || r == '\r' || r == '\n' || r == 'A' || r == 'a' || (r >= '1' && int(r-'0') <= len(cats)) {
				sel = r
				break
			}
		}
		if sel == '0' || sel == '\r' || sel == '\n' { // Enter leaves, like '0'
			fmt.Fprint(s, "\n")
			return Stay
		}
		fmt.Fprintf(s, "%c\n", sel)
		if sel == 'A' || sel == 'a' {
			about(s, w)
			continue
		}
		// playerLang, not the raw stored language: it applies the CP437 guard, so
		// a CP437 door reads help in English (or a CP437-safe language) instead
		// of substitution glyphs when the stored language can't be shown.
		browseCategory(s, cats[int(sel-'1')], playerLang(w))
	}
}

// showInstructions is the linear "read the manual" screen (BRE's Instructions
// item, separate from the ? help browser). It reads the overview plus every
// help topic in sequence from the single-source Markdown (help.Instructions),
// printing a section bar when the category changes and paging between topics.
// Q leaves early. Because it assembles the same topics the browser shows, it
// stays complete as the help content grows — no parallel document to maintain.
func showInstructions(s session.Session, w *ctx) Result {
	lang := playerLang(w)
	// Page the whole manual a screen at a time: emit one line, and after every
	// instructionsPerPage lines pause for Enter (continue) or Q (quit), so a long
	// topic can't scroll off before it's read. count carries across topics.
	const instructionsPerPage = 20
	count := 0
	emit := func(line string) bool {
		fmt.Fprintf(s, "%s\n", line)
		count++
		if count < instructionsPerPage {
			return true
		}
		count = 0
		fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightCyan, tr(s, "─»>Enter to continue, Q to quit<«─"), ansi.Reset)
		k, err := readKey(s)
		if err != nil || k == 'q' || k == 'Q' {
			return false
		}
		fmt.Fprint(s, "\n")
		return true
	}

	lastCat := ""
	for _, t := range help.Instructions(lang) {
		// The overview carries its own title heading, so it needs no section bar;
		// the real categories get one, mirroring BRE's section dividers.
		if t.Category != lastCat && t.Category != "introduction" {
			if !emit("") || !emit(fmt.Sprintf("%s── %s ──%s", ansi.FgBrightCyan, help.CategoryName(t.Category), ansi.Reset)) {
				return Stay
			}
		}
		lastCat = t.Category
		if !emit("") {
			return Stay
		}
		for _, line := range strings.Split(t.RenderANSI(78), "\n") {
			if !emit(line) {
				return Stay
			}
		}
	}
	if count > 0 { // un-paged lines remain since the last break: final "press a key"
		pause(s)
	}
	return Stay
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
	// A CP437 session can only render a catalog that maps to CP437, so offer just
	// those languages there (English, German, …); a UTF-8 session offers all.
	var langs []struct{ code, name string }
	for _, l := range helpLanguages {
		if w.UTF8 || cp437SafeLang(l.code) {
			langs = append(langs, l)
		}
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Language:"), ansi.Reset)
	for i, l := range langs {
		fmt.Fprintf(s, "  %d) %s\n", i+1, l.name)
	}
	i := promptInt(s, "Choose (0 to keep current)?")
	if i < 1 || i > len(langs) {
		return Stay
	}
	// Persist through a transaction — setting it on the w.Player() snapshot was
	// discarded, so the change never took effect and the menu kept the old language.
	code := langs[i-1].code
	if err := w.mutatePlayer(func(p *game.Empire) error {
		p.Language = code
		return nil
	}); err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Language set to %s.", languageName(code))
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
		// A 0) Back line + the same "Choice?" prompt as the category list. Topics
		// need Enter (some categories have >9, so a single key can't address them).
		fmt.Fprintf(s, "  0) %s\n", tr(s, "Back"))
		i := promptInt(s, "Choice?")
		if i < 1 || i > len(topics) {
			return
		}
		fmt.Fprintf(s, "\n%s\n", topics[i-1].RenderANSI(78))
		pause(s)
	}
}
