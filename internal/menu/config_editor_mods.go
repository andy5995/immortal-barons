package menu

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/mod"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The Mods page of both settings editors (#202): which mod folders the next
// reset applies, and in what order. It is offered on a stand-alone board only,
// since a league board refuses mods.

// modsTitle names the page in both editors.
const modsTitle = "Mods"

// modRow is one mod the page offers.
type modRow struct {
	name        string
	label       string // label and version, as the row shows it
	description string
	err         error // why the mod cannot be turned on; nil if it can
	on          bool
}

// modRows lists the mods in dataDir: the ones enabled first, in the order they
// apply, then the rest by name. An enabled mod whose folder is gone is still
// listed, so the sysop can turn it off. A name config.json lists twice is
// listed once.
func modRows(dataDir string, enabled []string) ([]modRow, error) {
	avail, err := mod.Available(dataDir)
	if err != nil {
		return nil, err
	}
	var rows []modRow
	byName := map[string]mod.Entry{}
	for _, e := range avail {
		byName[e.Name] = e
	}
	seen := map[string]bool{}
	add := func(name string, on bool) {
		if seen[name] {
			return
		}
		seen[name] = true
		r := modRow{name: name, label: name, on: on}
		e, found := byName[name]
		switch {
		case !found:
			r.err = fmt.Errorf("there is no folder %s", filepath.Join(dataDir, mod.Dir, name))
		case e.Err != nil:
			r.err = e.Err
		default:
			r.label = e.Mod.Ref().DisplayName() + " " + e.Mod.Version
			r.description = e.Mod.Description
		}
		if r.err != nil {
			r.label += " (cannot be loaded)"
		}
		rows = append(rows, r)
	}
	for _, name := range enabled {
		add(name, true)
	}
	for _, e := range avail {
		add(e.Name, false)
	}
	return rows, nil
}

// modsError is why c's mod list would not load at a reset, or nil. Both
// editors refuse to save while it is not nil, since the reset that follows a
// save would refuse the list after config.json had already been written.
func modsError(c game.Config) error {
	_, _, err := mod.Resolve(c.DataDir, c.Mods)
	return err
}

// noMods is what both editors say when the mods folder holds nothing.
func noMods(dataDir string) string {
	return fmt.Sprintf("There are no mods in %s. See docs/mods.md to make one.", filepath.Join(dataDir, mod.Dir))
}

// toggleMod turns name off if it is in list, or on at the end of it. Turning a
// mod on loads the whole new list as a reset would, so a mod that will not load,
// or that breaks a limit together with the others, is refused here rather than
// when the sysop saves.
func toggleMod(dataDir string, list []string, name string) ([]string, error) {
	if i := slices.Index(list, name); i >= 0 {
		return slices.Delete(slices.Clone(list), i, i+1), nil
	}
	next := append(slices.Clone(list), name)
	if _, _, err := mod.Resolve(dataDir, next); err != nil {
		return list, err
	}
	return next, nil
}

// modsValue is the field's one-line summary. It shares a line with the field's
// number and label, so a long list is summed up rather than spelled out.
func modsValue(c *game.Config) string {
	list := strings.Join(c.Mods, ", ")
	switch {
	case len(c.Mods) == 0:
		return "none"
	case len(list) > 40:
		return fmt.Sprintf("%d mods", len(c.Mods))
	}
	return list
}

// modsField is the line editor's Mods page: one field, which opens the list.
// 53 is the next free field number.
func modsField() cfgField {
	return cfgField{n: 53, label: "Mods in play", value: modsValue, edit: editMods}
}

// editMods is the line editor's mod list. A number turns that mod on or off.
func editMods(s session.Session, c *game.Config) {
	for {
		rows, err := modRows(c.DataDir, c.Mods)
		if err != nil {
			fail(s, err)
			return
		}
		if len(rows) == 0 {
			ok(s, "%s", noMods(c.DataDir))
			return
		}
		fmt.Fprintf(s, "\n%s%s%s\n%s\n", ansi.FgBrightCyan, modsTitle, ansi.Reset,
			WrapIndented(i18n.T(sessionLang(s), "Where two mods change the same value, the one turned on later wins."), "  "))
		if err := modsError(*c); err != nil {
			failNoPause(s, err)
		}
		for i, r := range rows {
			mark := "[ ]"
			if r.on {
				mark = "[x]"
			}
			fmt.Fprintln(s, wrapHanging(fmt.Sprintf("%2d) %s %s", i+1, mark, r.label), "  ", "          "))
		}
		fmt.Fprintf(s, "   0) %s\n", i18n.T(sessionLang(s), "Quit"))
		n := pickMod(s, len(rows))
		if n == 0 {
			return
		}
		r := rows[n-1]
		if r.err != nil && !r.on {
			fail(s, r.err)
			continue
		}
		next, err := toggleMod(c.DataDir, c.Mods, r.name)
		if err != nil {
			fail(s, err)
			continue
		}
		c.Mods = next
	}
}

// modsOrderNote heads the list in both editors. A literal passed through a
// constant is invisible to scripts/gen-ui-pot.py, so the line editor names it
// again in its i18n.T call.
const modsOrderNote = "Where two mods change the same value, the one turned on later wins."

// pickMod reads a row number, 0 to quit. ChoiceQuit reads one key, so it
// serves up to nine rows; a longer list takes a typed number.
func pickMod(s session.Session, rows int) int {
	if rows <= 9 {
		return ChoiceQuit(s, rows)
	}
	n, err := strconv.Atoi(strings.TrimSpace(prompt(s, "Turn which mod on or off (0 = quit)?")))
	if err != nil || n < 0 || n > rows {
		return 0
	}
	return n
}

// modsForm is the tview editor's Mods tab: a checkbox for each mod. It keeps
// the list in the order mods were turned on, as the line editor does, so the
// same choices give the same rules in either editor.
func (t *configTUI) modsForm() *tview.Form {
	form := tview.NewForm()
	c := t.base
	rows, err := modRows(c.DataDir, c.Mods)
	if err != nil || len(rows) == 0 {
		msg := noMods(c.DataDir)
		if err != nil {
			msg = err.Error()
		}
		form.AddTextView("", tview.Escape(msg), 0, 2, true, false)
		return form
	}
	note := modsOrderNote
	if err := modsError(c); err != nil {
		note = err.Error()
	}
	noteView := tview.NewTextView().SetDynamicColors(true).SetText(tview.Escape(note)).SetSize(2, 0)
	form.AddFormItem(noteView)

	var opening []string
	for _, r := range rows {
		if r.on {
			opening = append(opening, r.name)
		}
	}
	order := slices.Clone(opening)
	for _, r := range rows {
		help := r.description
		if r.err != nil {
			help = r.err.Error()
		}
		if help == "" {
			help = "This mod has no description."
		}
		b := tview.NewCheckbox().SetLabel(noStar + tview.Escape(r.label)).SetChecked(r.on)
		// A mod that does not load can be turned off but not on.
		b.SetDisabled(r.err != nil && !r.on)
		b.SetFocusFunc(func() { t.showHelp(tview.Escape(help)) })
		// tview calls this before it stores the new state, so the box cannot be
		// put back from here: it follows the list, and a list that will not load
		// is explained below and refused at save, like any other.
		b.SetChangedFunc(func(on bool) {
			order = slices.DeleteFunc(slices.Clone(order), func(n string) bool { return n == r.name })
			if on {
				order = append(order, r.name)
			}
			b.SetDisabled(r.err != nil && !on)
			msg, top := help, modsOrderNote
			if _, _, err := mod.Resolve(c.DataDir, order); err != nil {
				msg = "This list will not load, so it cannot be saved: " + err.Error()
				top = err.Error()
			}
			noteView.SetText(tview.Escape(top))
			t.showHelp(tview.Escape(msg))
		})
		form.AddFormItem(zebra(form, b))
		t.resetters = append(t.resetters, func() { b.SetChecked(r.on) })
	}
	t.resetters = append(t.resetters, func() { order = slices.Clone(opening) })
	t.binders = append(t.binders, func(c *game.Config) { c.Mods = slices.Clone(order) })
	return form
}
