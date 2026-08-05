package menu

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// lightbarSelect renders a list with a moving highlight and returns the chosen
// 0-based index, or -1 to go back. It sidesteps the old single-key limit (a list
// over 9 items couldn't be addressed by one digit) — the highlight reaches any
// item, so there are no numbers and no digit hotkeys. Navigation:
//
//   - ↑ / ↓            move the highlight (wrapping, and Back is part of the cycle)
//   - Enter            choose the highlighted item (Enter on Back goes back)
//   - a letter         BRE-style prefix type-ahead: jump to the first item whose
//     title matches the growing typed prefix
//   - Backspace / Q    go back immediately
//
// It clears the screen once, then redraws the list in place (cursor-up + erase)
// on each move, so it needs a plain ANSI terminal — which the door front-ends
// target.
func lightbarSelect(s session.Session, title string, items []string, backLabel string) int {
	if len(items) == 0 {
		return -1
	}
	fmt.Fprint(s, ansi.Clear)
	// Back is the final selectable row, so the highlight walks onto it like any
	// other item and Enter there goes back. No number hotkeys: typing a prefix
	// already jumps to any item, so the numbers would be redundant.
	all := append(append([]string{}, items...), backLabel)
	back := len(items) // Back's index in all
	sel, prefix := 0, ""
	draw := func() {
		fmt.Fprint(s, ansi.Home) // redraw from top-left each frame — survives a resize
		fmt.Fprintf(s, "%s%s%s%s\n%s\n", ansi.EraseLine, ansi.FgBrightCyan, title, ansi.Reset, ansi.EraseLine)
		for i, it := range all {
			fmt.Fprint(s, ansi.EraseLine)
			// The ">" marker carries the selection on its own: reverse video alone
			// says nothing on a monochrome terminal or to a reader with a colour
			// vision deficiency. Both rows put the title in the same column.
			if i == sel {
				fmt.Fprintf(s, "  %s>%s %s%s%s\n", ansi.FgBrightWhite, ansi.Reset, ansi.Reverse, it, ansi.Reset)
			} else {
				fmt.Fprintf(s, "    %s\n", it)
			}
		}
		fmt.Fprintf(s, "%s   %s↑↓ move · Enter select · type to jump · Backspace back%s\n%s", ansi.EraseLine, ansi.Dim, ansi.Reset, ansi.EraseDown)
	}
	for {
		draw()
		kind, r, err := readNavKey(s)
		if err != nil {
			return -1
		}
		switch kind {
		case keyUp:
			sel, prefix = (sel-1+len(all))%len(all), ""
		case keyDown:
			sel, prefix = (sel+1)%len(all), ""
		case keyEnter:
			if sel == back {
				return -1
			}
			return sel
		case keyRune:
			switch {
			case r == 'q' || r == 'Q' || r == 0x08 || r == 0x7f: // Q, Backspace, Del: go back
				return -1
			case unicode.IsLetter(r):
				k := unicode.ToLower(r)
				if i := prefixMatch(all, prefix+string(k)); i >= 0 {
					sel, prefix = i, prefix+string(k)
				} else if i := prefixMatch(all, string(k)); i >= 0 {
					sel, prefix = i, string(k) // no match extending the prefix: restart it
				}
			}
		}
	}
}

// chooseFromList picks one of items, returning its 0-based index or -1 for
// back. It is the lightbar on an ANSI terminal and a numbered list where the
// escapes cannot be rendered (issue #99) — that session gets no highlight to
// move and no in-place repaint, so the lightbar would append a fresh copy of
// the whole list on every arrow key.
func chooseFromList(s session.Session, plain bool, title string, items []string, backLabel string) int {
	if plain {
		return numberedSelect(s, title, items, backLabel)
	}
	return lightbarSelect(s, title, items, backLabel)
}

// numberedSelect is the non-ANSI list: print it once, read a number. It follows
// the menu convention — a "0) <back>" line and the bare ">" prompt readChoice
// uses — but reads a whole line rather than one key, since the list runs past
// nine items.
func numberedSelect(s session.Session, title string, items []string, backLabel string) int {
	fmt.Fprintf(s, "\n%s\n\n", title)
	for i, it := range items {
		fmt.Fprintf(s, "  %2d) %s\n", i+1, it)
	}
	fmt.Fprintf(s, "   0) %s\n", backLabel)
	for {
		in := strings.TrimSpace(prompt(s, ">"))
		if in == "" {
			continue
		}
		if q := unicode.ToLower([]rune(in)[0]); q == 'q' { // Q leaves, as in the lightbar
			return -1
		}
		n, err := strconv.Atoi(in)
		if err != nil || n < 0 || n > len(items) {
			continue
		}
		if n == 0 {
			return -1
		}
		return n - 1
	}
}

// prefixMatch returns the index of the first item whose title, lowercased,
// starts with p, or -1 if none does.
func prefixMatch(items []string, p string) int {
	for i, it := range items {
		if strings.HasPrefix(strings.ToLower(it), p) {
			return i
		}
	}
	return -1
}

// helpBrowse is the categorized help browser: pick a category, then a topic,
// then read it. It replaces the old flat "pick 1-of-N topics" list. Content is
// the embedded single-source Markdown from internal/help.
func helpBrowse(s session.Session, w *ctx) Result {
	for {
		cats := help.Categories()
		// About is the last list item (not a reserved letter as before): with a
		// lightbar the arrows/type-ahead reach it, so it needs no dedicated key.
		items := make([]string, 0, len(cats)+1)
		for _, c := range cats {
			items = append(items, help.CategoryName(c))
		}
		items = append(items, tr(s, "About"))
		i := chooseFromList(s, w.Plain, tr(s, "Help — choose a category:"), items, tr(s, "Quit"))
		if i < 0 {
			return Stay
		}
		if i == len(cats) { // the About entry
			about(s, w)
			continue
		}
		// playerLang, not the raw stored language: it applies the CP437 guard, so
		// a CP437 door reads help in English (or a CP437-safe language) instead
		// of substitution glyphs when the stored language can't be shown.
		browseCategory(s, w.Plain, cats[i], playerLang(w))
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

// browseCategory lists a category's topics as a lightbar and renders the chosen
// one, paged. The lightbar reaches any number of topics (covert has 15), so no
// single-key-vs-Enter compromise is needed.
func browseCategory(s session.Session, plain bool, cat, lang string) {
	for {
		topics := help.Topics(cat, lang)
		titles := make([]string, len(topics))
		for i, t := range topics {
			titles[i] = t.Title
		}
		i := chooseFromList(s, plain, help.CategoryName(cat)+tr(s, " — choose a topic:"), titles, tr(s, "Back"))
		if i < 0 {
			return
		}
		fmt.Fprintf(s, "\n%s\n", topics[i].RenderANSI(78))
		pause(s)
	}
}
