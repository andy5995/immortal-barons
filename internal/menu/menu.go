// Package menu is the presentation-abstracted menu engine plus the BRE
// menu tree. menu.go is the generic framework; tree.go is the BRE-specific
// content. The framework knows nothing about which menus exist.
package menu

import (
	"fmt"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

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
type Action func(s session.Session, g *game.World) Result

type Item struct {
	Key     rune
	Label   string
	LabelFn func(*game.World) string // dynamic label (e.g. toggle state); wins over Label
	Do      Action                   // nil => heading/separator, not selectable
	Hidden  func(*game.World) bool   // nil => always shown
}

func (it *Item) hidden(g *game.World) bool {
	return it.Hidden != nil && it.Hidden(g)
}

func (it *Item) label(g *game.World) string {
	if it.LabelFn != nil {
		return it.LabelFn(g)
	}
	return it.Label
}

type Menu struct {
	Title  string
	Items  []Item
	Status func(*game.World) string // optional status bar under the menu
}

// selectable reports whether it is a visible, choosable item (not a
// heading/separator, not hidden).
func (it *Item) selectable(g *game.World) bool {
	return it.Do != nil && !it.hidden(g)
}

// byKey returns the selectable item whose Key case-insensitively equals r.
func (m *Menu) byKey(r rune, g *game.World) *Item {
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
func (m *Menu) readChoice(s session.Session, g *game.World) (*Item, error) {
	fmt.Fprintf(s, "%sChoice>%s ", ansi.FgBrightWhite, ansi.Reset)
	r, err := s.ReadKey()
	if err != nil {
		return nil, err
	}
	it := m.byKey(r, g)
	if it == nil {
		fmt.Fprint(s, "\n")
		return nil, nil
	}
	fmt.Fprintf(s, "%s\n", it.label(g))
	return it, nil
}

// Run drives the menu loop against a Session. It keeps a stack of menus:
// draw the top, read a single keypress hotkey, dispatch, then
// push/pop/quit/redraw. The loop is flat (no recursion), which keeps it
// easy to test with a scripted Session. Returns nil on a clean Quit, or the
// ReadKey error otherwise.
func Run(s session.Session, g *game.World, root *Menu) error {
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

func draw(s session.Session, g *game.World, m *Menu) {
	fmt.Fprint(s, ansi.Clear)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightCyan, m.Title, ansi.Reset)
	fmt.Fprintf(s, "%s\n", rule)
	for i := range m.Items {
		it := &m.Items[i]
		if it.hidden(g) {
			continue
		}
		if it.Do == nil {
			fmt.Fprintf(s, "  %s\n", it.label(g))
			continue
		}
		fmt.Fprintf(s, "  %s(%c)%s %s\n",
			ansi.FgBrightYellow, it.Key, ansi.Reset, it.label(g))
	}
	if m.Status != nil {
		fmt.Fprintf(s, "%s\n%s%s%s\n", rule, ansi.FgGreen, m.Status(g), ansi.Reset)
	}
	fmt.Fprint(s, "\n")
}

// stubbed prints a "not yet implemented" notice and waits for a keypress.
func stubbed(name string) Action {
	return func(s session.Session, g *game.World) Result {
		fmt.Fprintf(s, "\n  %s[%s — not yet implemented]%s\n  Press any key...",
			ansi.FgYellow, name, ansi.Reset)
		s.ReadKey()
		return Stay
	}
}

func gotoMenu(m *Menu) Action {
	return func(session.Session, *game.World) Result { return Goto(m) }
}

func back(session.Session, *game.World) Result { return Back }
func quit(session.Session, *game.World) Result { return Quit }

// toggle flips a bool preference and redraws (its label shows the state).
func toggle(get func(*game.World) *bool) Action {
	return func(_ session.Session, g *game.World) Result {
		p := get(g)
		*p = !*p
		return Stay
	}
}

func onOff(name string, get func(*game.World) *bool) func(*game.World) string {
	return func(g *game.World) string {
		state := "OFF"
		if *get(g) {
			state = "ON"
		}
		return fmt.Sprintf("%-28s [%s]", name, state)
	}
}
