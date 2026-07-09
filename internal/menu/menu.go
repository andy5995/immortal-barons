// Package menu is the presentation-abstracted menu engine plus the BRE
// menu tree. menu.go is the generic framework; tree.go is the BRE-specific
// content. The framework knows nothing about which menus exist.
package menu

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// ctx is the per-session menu context: one shared *game.World plus the active
// empire for THIS session. Threading the active empire here (instead of a
// field on the shared World) is what lets the web server run concurrent
// sessions against one world. It embeds *game.World so callback bodies keep
// using w.Config, w.Prices, w.NetWorth(...), etc. unchanged.
type ctx struct {
	*game.World
	active *game.Empire
	// UTF8 reports whether this session can display UTF-8. When false (a CP437
	// door/terminal) all output is forced to English, since non-English text
	// cannot be represented in CP437.
	UTF8 bool
}

// Player is the active empire for this session (nil before onboarding / after
// abdication).
func (c *ctx) Player() *game.Empire { return c.active }

// playerLang is the active caller's UI language ("" = English), used to
// localize menu chrome at render time.
func playerLang(c *ctx) string {
	// A CP437 session can only render English, so ignore any stored language —
	// this is the single render-time guard that keeps a language set via the
	// UTF-8 web front-end from mojibaking when reached through a CP437 door.
	if c != nil && c.UTF8 && c.active != nil {
		return c.active.Language
	}
	return ""
}

// langSession wraps a Session so downstream output helpers can learn the
// caller's language from the Session alone (they receive s but not the World).
// The language is read live from the active empire, so a mid-session change in
// Preferences takes effect immediately. This is per-session state — safe for
// the web front-end, which runs concurrent sessions in one process.
type langSession struct {
	session.Session
	c *ctx
}

func (l *langSession) Lang() string { return playerLang(l.c) }

// SetInputLine forwards the prompt-restore hook down to the inner session so an
// idle/time warning can reprint the caller's current input line.
func (l *langSession) SetInputLine(line string) {
	if s, ok := l.Session.(session.InputLineSetter); ok {
		s.SetInputLine(line)
	}
}

// sessionLang extracts the caller's language from a wrapped Session, or "" for
// a plain Session (e.g. tests) — which renders English.
func sessionLang(s session.Session) string {
	if lp, ok := s.(interface{ Lang() string }); ok {
		return lp.Lang()
	}
	return ""
}

// tr translates msgid to the session's language, for direct-print call sites
// that don't go through a prompt/ok/fail helper. Keep color codes and layout
// whitespace outside the msgid so the catalog holds clean, translatable text.
func tr(s session.Session, msgid string) string {
	return i18n.T(sessionLang(s), msgid)
}

// Result tells the Run loop what to do after an action fires.
type Result struct {
	kind   resultKind
	target *Menu
}

type resultKind int

const (
	kindStay resultKind = iota
	kindBack
	kindQuit
	kindGoto
)

var (
	Stay = Result{kind: kindStay} // redraw the current menu
	Back = Result{kind: kindBack} // pop to the parent menu
	Quit = Result{kind: kindQuit} // exit the session
)

// Goto pushes a submenu onto the stack.
func Goto(m *Menu) Result { return Result{kind: kindGoto, target: m} }

// Action performs a menu item and reports what the loop should do next.
type Action func(s session.Session, g *ctx) Result

type Item struct {
	Key     rune
	Label   string
	LabelFn func(*ctx) string // dynamic label (e.g. toggle state); wins over Label
	Do      Action            // nil => heading/separator, not selectable
	Hidden  func(*ctx) bool   // nil => always shown

	// Price and Owned drive the BRE-style Price / # Owned columns on the
	// Spending and Sell menus. When any item in a menu sets either, the menu
	// draws the column header and every item's values (blank where nil).
	Price func(*ctx) int
	Owned func(*ctx) int
}

func (it *Item) hidden(g *ctx) bool {
	return it.Hidden != nil && it.Hidden(g)
}

func (it *Item) label(g *ctx) string {
	if it.LabelFn != nil {
		return it.LabelFn(g)
	}
	return it.Label
}

// displayLabel is the label as shown: a static Label is translated to lang; a
// dynamic LabelFn (toggle state, "Language: X") is left as its function
// produces it, since those are formatted, not catalog strings.
func (it *Item) displayLabel(g *ctx, lang string) string {
	if it.LabelFn != nil {
		return it.LabelFn(g)
	}
	return i18n.T(lang, it.Label)
}

type Menu struct {
	Title string
	Color string // ansi color escape for the title and item hotkeys; "" uses a default
	Items []Item
	// ExitOnEnter marks a turn-pipeline menu whose '0' item is a safe exit
	// (back, not session quit). When the player's EnterExitsBuy preference is
	// on, that item is offered as the default and Enter selects it.
	ExitOnEnter bool
	// DefaultOnEnter picks a per-context default item (e.g. "Play" while
	// turns remain, "Quit" once they're gone): it's offered after the prompt
	// the same way the ExitOnEnter default is, and Enter selects it. nil
	// return means no default this time. Takes priority over ExitOnEnter.
	DefaultOnEnter func(*ctx) *Item
	Status         func(*ctx) string // optional status bar under the menu
	// NoClear suppresses the screen clear normally done before drawing this
	// menu, so an action's output (e.g. a purchase confirmation) stays
	// visible above the redrawn menu instead of being wiped (BRE-style).
	NoClear bool
}

// selectable reports whether it is a visible, choosable item (not a
// heading/separator, not hidden).
func (it *Item) selectable(g *ctx) bool {
	return it.Do != nil && !it.hidden(g)
}

// byKey returns the selectable item whose Key case-insensitively equals r.
func (m *Menu) byKey(r rune, g *ctx) *Item {
	key := unicode.ToUpper(r)
	for i := range m.Items {
		it := &m.Items[i]
		if it.selectable(g) && it.Key != 0 && unicode.ToUpper(it.Key) == key {
			return it
		}
	}
	return nil
}

// readChoice prints "Choice> ", reads ONE keypress, and returns the matching
// menu item immediately (no Enter). It echoes the chosen item's label. Keys
// that match no visible selectable item are ignored (return nil -> redraw).
func (m *Menu) readChoice(s session.Session, g *ctx) (*Item, error) {
	// With EnterExitsBuy on, a pipeline menu offers its '0' exit as the
	// default: show it after the prompt, and let Enter select it. Resolve the
	// default item and its label under the world lock (byKey/label read shared
	// state), before any I/O.
	var def *Item
	var defLabel, prompt string
	g.With(func() {
		if m.DefaultOnEnter != nil {
			def = m.DefaultOnEnter(g)
		} else if g.EnterExitsBuy && m.ExitOnEnter {
			def = m.byKey('0', g)
		}
		if def != nil {
			defLabel = def.label(g)
		}
		prompt = i18n.T(playerLang(g), "Choice>")
	})
	fmt.Fprintf(s, "%s%s%s ", ansi.FgBrightWhite, prompt, ansi.Reset)
	if def != nil {
		fmt.Fprint(s, defLabel)
	}
	r, err := s.ReadKey()
	if err != nil {
		if errors.Is(err, session.ErrSessionEnded) {
			// Boot at a menu: unwind everything — nested submenus and the
			// turn flow (runTurn calls Run for Spending/Attack/etc.) — instead
			// of returning up one level, which drops the caller back to the
			// game menu rather than ending the session.
			session.End(err)
		}
		return nil, err
	}
	if def != nil && (r == '\r' || r == '\n') {
		fmt.Fprint(s, "\n")
		return def, nil
	}
	if def != nil { // a real choice follows; erase the shown default first
		for range []rune(defLabel) {
			fmt.Fprint(s, "\b \b")
		}
	}
	// Match the keypress and read the chosen item's label under the lock.
	var it *Item
	var itLabel string
	g.With(func() {
		it = m.byKey(r, g)
		if it != nil {
			itLabel = it.label(g)
		}
	})
	if it == nil {
		fmt.Fprint(s, "\n")
		return nil, nil
	}
	fmt.Fprintf(s, "%s\n", itLabel)
	return it, nil
}

// Run drives the menu loop against a Session. It keeps a stack of menus:
// draw the top, read a single keypress hotkey, dispatch, then
// push/pop/quit/redraw. The loop is flat (no recursion), which keeps it
// easy to test with a scripted Session. Returns nil on a clean Quit, or the
// ReadKey error otherwise.
func Run(s session.Session, g *ctx, root *Menu) error {
	s = &langSession{Session: s, c: g}
	stack := []*Menu{root}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		draw(s, g, cur)

		item, err := cur.readChoice(s, g)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		switch res := item.Do(s, g); res.kind {
		case kindStay:
			// redraw on next iteration
		case kindBack:
			stack = stack[:len(stack)-1]
		case kindQuit:
			return nil
		case kindGoto:
			stack = append(stack, res.target)
		}
	}
	return nil
}

const rule = "────────────────────────────────────────────────────────────"

// titleRule renders a BRE-style header: the bracketed menu title centered on a
// rule line, e.g. "──────[Goldie Luck's Bank]──────", all in the menu color.
func titleRule(color, title string) string {
	label := "[" + title + "]"
	width := len([]rune(rule))
	if len([]rune(label)) >= width {
		return color + label + ansi.Reset
	}
	left := (width - len([]rune(label))) / 2
	right := width - left - len([]rune(label))
	return color + strings.Repeat("─", left) + label + strings.Repeat("─", right) + ansi.Reset
}

// hasColumns reports whether any visible item carries a Price/Owned column, so
// the menu draws the BRE-style "Price / # Owned" table (Spending, Sell).
func (m *Menu) hasColumns(g *ctx) bool {
	for i := range m.Items {
		it := &m.Items[i]
		if it.hidden(g) {
			continue
		}
		if it.Price != nil || it.Owned != nil {
			return true
		}
	}
	return false
}

func draw(s session.Session, g *ctx, m *Menu) {
	// Render the whole menu into an in-memory buffer while holding the world
	// lock, then flush it to the session unlocked. Every item callback
	// (hidden/label/Price/Owned/Status) reads shared empire and world
	// state; running them all inside one g.With makes those reads race-free
	// against the daily-maintenance ticker. Building a strings.Builder is not
	// I/O, so the lock is never held across the actual write (the final
	// Fprint below).
	var b strings.Builder
	g.With(func() {
		if !m.NoClear {
			b.WriteString(ansi.Clear)
		}
		col := m.Color
		if col == "" {
			col = ansi.FgBrightCyan
		}
		lang := playerLang(g)
		fmt.Fprintf(&b, "%s\n", titleRule(col, i18n.T(lang, m.Title)))
		cols := m.hasColumns(g)
		if cols {
			fmt.Fprintf(&b, "%s  Key %-18s %8s %9s%s\n",
				col, i18n.T(lang, "Item"), i18n.T(lang, "Price"), i18n.T(lang, "# Owned"), ansi.Reset)
		}
		for i := range m.Items {
			it := &m.Items[i]
			if it.hidden(g) {
				continue
			}
			if it.Do == nil {
				fmt.Fprintf(&b, "  %s\n", it.displayLabel(g, lang))
				continue
			}
			if cols {
				price, owned := "", ""
				if it.Price != nil {
					price = comma(it.Price(g))
				}
				if it.Owned != nil {
					owned = comma(it.Owned(g))
				}
				fmt.Fprintf(&b, "  %s(%c)%s %s%-18s%s %8s %9s\n",
					col, it.Key, ansi.Reset, ansi.FgWhite, it.displayLabel(g, lang), ansi.Reset, price, owned)
				continue
			}
			fmt.Fprintf(&b, "  %s(%c)%s %s%s%s\n",
				col, it.Key, ansi.Reset, ansi.FgWhite, it.displayLabel(g, lang), ansi.Reset)
		}
		if m.Status != nil {
			fmt.Fprintf(&b, "%s\n%s%s%s\n", rule, ansi.FgBrightYellow, m.Status(g), ansi.Reset)
		}
		b.WriteString("\n")
	})
	fmt.Fprint(s, b.String())
}

func gotoMenu(m *Menu) Action {
	return func(session.Session, *ctx) Result { return Goto(m) }
}

func back(session.Session, *ctx) Result { return Back }
func quit(session.Session, *ctx) Result { return Quit }

// toggle flips a bool preference and redraws (its label shows the state).
func toggle(get func(*ctx) *bool) Action {
	return func(_ session.Session, g *ctx) Result {
		p := get(g)
		*p = !*p
		return Stay
	}
}

func onOff(name string, get func(*ctx) *bool) func(*ctx) string {
	return func(g *ctx) string {
		state := "OFF"
		if *get(g) {
			state = "ON"
		}
		return fmt.Sprintf("%-28s [%s]", name, state)
	}
}
