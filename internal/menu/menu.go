// Package menu is the presentation-abstracted menu engine plus the BRE
// menu tree. menu.go is the generic framework; tree.go is the BRE-specific
// content. The framework knows nothing about which menus exist.
package menu

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// ctx is the per-session menu context: one shared *game.World plus the caller's
// BBS handle, which identifies the active empire for THIS session. Storing the
// handle (not a cached *Empire pointer) is what keeps the active empire correct
// after the door's FileStore reloads the world inside a transaction — the old
// pointer would go stale, but the handle re-resolves against fresh state. It
// also lets the web server run concurrent sessions against one world. It embeds
// *game.World so callback bodies keep using w.Config, w.Prices, w.NetWorth(...),
// etc. unchanged.
type ctx struct {
	*game.World
	handle string
	// cached memoizes the active empire so unlocked reads (the macro expander on
	// each keypress, output-time language, action-body gathers) don't scan
	// w.Empires and race a concurrent AddHuman on the web front-end. It is
	// re-resolved by handle whenever the world's reload generation advances (a
	// door FileStore reload) — the web never reloads, so after the first resolve
	// this is the same stable pointer the pre-handle code cached. ctx is
	// per-session, driven by one goroutine, so these fields need no locking.
	cached    *game.Empire
	cachedGen uint64
	cachedSet bool
	// UTF8 reports whether this session can display UTF-8. When false (a CP437
	// door/terminal) all output is forced to English, since non-English text
	// cannot be represented in CP437.
	UTF8 bool
}

// Player is the active empire for this session (nil before onboarding / after
// abdication). It re-resolves by handle after a world reload (so a stale
// post-reload pointer is never returned) but otherwise returns the memoized
// empire, keeping unlocked reads off the shared w.Empires slice.
func (c *ctx) Player() *game.Empire {
	if gen := c.World.ReloadGen(); !c.cachedSet || c.cachedGen != gen {
		c.cached = c.World.FindByOwner(c.handle)
		c.cachedGen = gen
		c.cachedSet = true
	}
	return c.cached
}

// mutatePlayer runs fn inside a world transaction with the re-resolved active
// player, returning errRealmChanged if the realm has vanished (abdicated by
// another node between the prompt and the write). It centralizes the
// re-resolve-then-nil-check that every mutating menu action shares; callers
// capture any extra values (gold, land, a report) through the closure.
func (c *ctx) mutatePlayer(fn func(p *game.Empire) error) error {
	var err error
	c.With(func() {
		p := c.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		err = fn(p)
	})
	return err
}

// cp437SafeLangs records, per language code, whether that whole catalog maps to
// CP437 — computed once at startup (catalogs are static). A CP437 door can then
// render the catalogs that fit the code page (e.g. German) and fall back to
// English only for the ones that don't (Cyrillic, CJK, Central-European Latin).
// English ("") is trivially safe.
var cp437SafeLangs = func() map[string]bool {
	m := map[string]bool{"": true}
	for _, l := range i18n.Languages {
		// The endonym is vetted along with the catalog: the picker prints it, so
		// a language whose own name can't be shown in CP437 isn't safe either.
		m[l.Code] = session.CP437Encodable(l.Name) &&
			session.AllCP437Encodable(i18n.Strings(l.Code))
	}
	return m
}()

// cp437SafeLang reports whether lang's catalog can be shown on a CP437 terminal.
func cp437SafeLang(lang string) bool { return cp437SafeLangs[lang] }

// playerLang is the active caller's UI language ("" = English), used to
// localize menu chrome at render time.
func playerLang(c *ctx) string {
	if c == nil {
		return ""
	}
	p := c.Player()
	if p == nil || p.Language == "" {
		return ""
	}
	// A UTF-8 session renders any language. A CP437 session renders only a
	// catalog that maps entirely to CP437 (e.g. German); anything else (Cyrillic,
	// CJK) falls back to English rather than mojibake. This one render-time guard
	// also keeps a language set via the UTF-8 web front-end from breaking when the
	// same empire is later reached through a CP437 door.
	if c.UTF8 || cp437SafeLang(p.Language) {
		return p.Language
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

// DrainInput forwards the single-key trailing-terminator drain to the inner
// session; the embedded Session promotes ReadKey but not this optional method.
func (l *langSession) DrainInput() { session.Drain(l.Session) }

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
	Color   string            // overrides the menu color for this item's key+label; e.g. to tint an entry the color of the submenu it opens

	// Price and Owned drive the BRE-style Price / # Owned columns on the
	// Spending and Sell menus. When any item in a menu sets either, the menu
	// draws the column header and every item's values (blank where nil).
	Price func(*ctx) int
	Owned func(*ctx) int
}

func (it *Item) hidden(g *ctx) bool {
	return it.Hidden != nil && it.Hidden(g)
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
	// Columns opts a menu into a multi-column item block (BRE draws its menus
	// in columns). 0 or 1 keeps the default one-item-per-line layout; 2 or 3
	// lay the selectable items out row-major across that many columns. Only
	// used for menus without Price/# Owned columns.
	Columns int
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

// readChoice prints the "> " prompt, reads keypresses, and returns the matching menu
// item immediately (no Enter). It echoes the chosen item's label. Keys that match
// no visible selectable item are ignored in place — the menu is NOT redrawn — so
// an unbound keypress doesn't rescroll the screen.
func (m *Menu) readChoice(s session.Session, g *ctx) (*Item, error) {
	// With EnterExitsBuy on, a pipeline menu offers its '0' exit as the
	// default: show it after the prompt, and let Enter select it. Resolve the
	// default item and its label under the world lock (byKey/label read shared
	// state), before any I/O.
	var def *Item
	var defLabel string
	g.With(func() {
		if m.DefaultOnEnter != nil {
			def = m.DefaultOnEnter(g)
		} else if g.EnterExitsBuy && m.ExitOnEnter {
			def = m.byKey('0', g)
		}
		if def != nil {
			defLabel = def.displayLabel(g, playerLang(g))
		}
	})
	// A bare ">" is the prompt — language-neutral, so it needs no translation
	// (replaces BRE's "Choice>"). The default action's label follows it.
	fmt.Fprintf(s, "%s>%s ", ansi.FgBrightWhite, ansi.Reset)
	if def != nil {
		fmt.Fprint(s, defLabel)
	}
	// Wait for a key that actually does something: Enter (the default) or a key
	// bound to a visible item. Keys bound to nothing are ignored in place — no
	// newline, no menu redraw — so mashing an unbound key doesn't rescroll the menu.
	for {
		r, err := readKey(s)
		if err != nil {
			return nil, err
		}
		drainInput(s) // drop a trailing Enter sent in one burst with the selection key
		if def != nil && (r == '\r' || r == '\n') {
			fmt.Fprint(s, "\n")
			return def, nil
		}
		// Match the keypress and read the chosen item's label under the lock.
		var it *Item
		var itLabel string
		g.With(func() {
			it = m.byKey(r, g)
			if it != nil {
				itLabel = it.displayLabel(g, playerLang(g))
			}
		})
		if it == nil {
			continue // no item for this key: ignore it, keep waiting
		}
		if def != nil { // a real choice follows; erase the shown default first
			for range []rune(defLabel) {
				fmt.Fprint(s, "\b \b")
			}
		}
		fmt.Fprintf(s, "%s\n", itLabel)
		return it, nil
	}
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
		// Eliminated mid-session: another node's attack (or a WMD strike) can kill
		// this empire while the caller is playing. Once dead — or gone entirely —
		// the caller may not keep playing the corpse. End the whole session via
		// session.End so the unwind reaches GameLoop even from a nested Run (e.g.
		// the pre-turn Diplomacy menu); the collapse notice prints once.
		if eliminated(g) {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed,
				tr(s, "Your empire has collapsed — you have been eliminated. Return on a later day to build a new realm."),
				ansi.Reset)
			session.End(io.EOF)
		}
	}
	return nil
}

// eliminated reports whether the active empire is gone or dead, read under the
// world lock so a concurrent maintenance tick (web front-end) can't race the
// Alive check. A nil Player (removed) counts as eliminated too.
func eliminated(g *ctx) bool {
	dead := false
	g.With(func() {
		p := g.Player()
		dead = p == nil || !p.Alive
	})
	return dead
}

// 62 columns wide, matching BRE's main menu rules (from a live capture).
const rule = "──────────────────────────────────────────────────────────────"

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
		if m.Columns >= 2 && !cols {
			drawItemsColumns(&b, g, m, col, lang, m.Columns)
		} else {
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
						price = formatGold(it.Price(g), lang)
					}
					if it.Owned != nil {
						owned = formatGold(it.Owned(g), lang)
					}
					// BRE renders the Price column bright-white and the Owned column
					// white (#17 menu audit); the key is the menu accent color.
					fmt.Fprintf(&b, "  %s(%c)%s %s%-18s%s %s%8s%s %s%9s%s\n",
						col, it.Key, ansi.Reset, ansi.FgWhite, it.displayLabel(g, lang), ansi.Reset,
						ansi.FgBrightWhite, price, ansi.Reset, ansi.FgWhite, owned, ansi.Reset)
					continue
				}
				kcol, lcol := col, ansi.FgWhite
				if it.Color != "" {
					kcol, lcol = it.Color, it.Color
				}
				fmt.Fprintf(&b, "  %s(%c)%s %s%s%s\n",
					kcol, it.Key, ansi.Reset, lcol, it.displayLabel(g, lang), ansi.Reset)
			}
		}
		if m.Status != nil {
			fmt.Fprintf(&b, "%s\n%s%s%s\n", rule, ansi.FgBrightYellow, m.Status(g), ansi.Reset)
		}
		b.WriteString("\n")
	})
	fmt.Fprint(s, b.String())
}

// drawItemsColumns renders a menu's selectable items in ncol side-by-side
// columns, filling column-major (down the first column, then the next), so the
// on-screen key order descends the left edge as BRE's menus do — NOT row-major
// left-to-right. Every cell but the last in a row is padded to a fixed visible
// width so the columns align; ANSI escapes are excluded from that width
// (utf8.RuneCountInString). Hidden items are skipped; a heading/separator
// (Do == nil) closes the current column block and takes its own full-width
// line. The cell width narrows as ncol grows so a full row stays inside 80
// columns for the door standard (2 cols → 34, 3+ cols → 25: 25+25+~25 ≈ 75).
func drawItemsColumns(b *strings.Builder, g *ctx, m *Menu, col, lang string, ncol int) {
	cellWidth := 34 // visible width of a padded cell (incl. the "  (K) " prefix)
	if ncol >= 3 {
		cellWidth = 25
	}
	cell := func(it *Item) (string, int) {
		label := it.displayLabel(g, lang)
		kcol, lcol := col, ansi.FgWhite
		if it.Color != "" {
			kcol, lcol = it.Color, it.Color
		}
		s := fmt.Sprintf("  %s(%c)%s %s%s%s", kcol, it.Key, ansi.Reset, lcol, label, ansi.Reset)
		return s, 6 + utf8.RuneCountInString(label) // "  (K) " is 6 visible cols
	}
	// renderBlock lays a run of items out column-major. With nrows =
	// ceil(len/ncol), cell (row r, col c) is item[c*nrows+r]: item indices fill
	// down column 0 first, so keys read top-to-bottom then left-to-right.
	renderBlock := func(items []*Item) {
		if len(items) == 0 {
			return
		}
		nrows := (len(items) + ncol - 1) / ncol
		for r := 0; r < nrows; r++ {
			// last = the rightmost populated column on this row (may be short on the
			// final row); indices grow with c, so every column left of it is filled.
			last := 0
			for c := 0; c < ncol; c++ {
				if c*nrows+r < len(items) {
					last = c
				}
			}
			for c := 0; c <= last; c++ {
				s, n := cell(items[c*nrows+r])
				if c < last { // pad all but the trailing cell so columns align
					pad := cellWidth - n
					if pad < 1 {
						pad = 1
					}
					s += strings.Repeat(" ", pad)
				}
				b.WriteString(s)
			}
			b.WriteString("\n")
		}
	}
	var block []*Item
	for i := range m.Items {
		it := &m.Items[i]
		if it.hidden(g) {
			continue
		}
		if it.Do == nil { // heading/separator: close the block, then a full-width line
			renderBlock(block)
			block = block[:0]
			fmt.Fprintf(b, "  %s\n", it.displayLabel(g, lang))
			continue
		}
		block = append(block, it)
	}
	renderBlock(block)
}

func gotoMenu(m *Menu) Action {
	return func(session.Session, *ctx) Result { return Goto(m) }
}

func back(session.Session, *ctx) Result { return Back }
func quit(session.Session, *ctx) Result { return Quit }

// toggle flips a bool preference and redraws (its label shows the state). The
// flip runs inside a transaction: the preferences live on the World, so a door
// node must read-modify-write them against fresh state and persist the change —
// an unlocked flip would be lost on the next reload. (The pointer stays valid
// across reloads because the World is reloaded in place; only *Empire pointers
// rebind.)
func toggle(get func(*ctx) *bool) Action {
	return func(_ session.Session, g *ctx) Result {
		g.With(func() {
			p := get(g)
			*p = !*p
		})
		return Stay
	}
}

func onOff(name string, get func(*ctx) *bool) func(*ctx) string {
	return func(g *ctx) string {
		// BRE shows a bare, right-aligned Yes/No (no brackets), verified against a
		// live Preferences capture.
		state := "No"
		if *get(g) {
			state = "Yes"
		}
		return fmt.Sprintf("%-28s %3s", i18n.T(playerLang(g), name), state)
	}
}
