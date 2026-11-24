package menu

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
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

// optionRow is one choice a mod in c.Mods offers. The manifest is read with
// the game's choices so its label, bounds and diff all come from the same mod
// copy the editors will save.
type optionRow struct {
	modName     string
	id          string
	label       string
	line        string // type, default and range, as -check-mod prints it
	description string
	def         *mod.Option
}

// optionRows lists the options of the enabled mods, in list order, each mod's
// ids in manifest order. An unreadable mod is skipped here: modsError and the
// mod list row already say why it cannot be part of the next reset.
func optionRows(dataDir string, enabled []string, choices map[string]map[string]int64) []optionRow {
	var out []optionRow
	for _, name := range enabled {
		m, err := mod.ReadWithOptions(filepath.Join(dataDir, mod.Dir, name), choices[name])
		if err != nil {
			continue
		}
		for _, id := range m.SortedOptionIDs() {
			o := m.Manifest.Options[id]
			out = append(out, optionRow{
				modName: name, id: id, label: o.Label, line: m.OptionLine(id),
				description: o.Description, def: o,
			})
		}
	}
	return out
}

// optionChosen is the value a game plays: its explicit choice, or the default.
func optionChosen(row optionRow, choices map[string]map[string]int64) int64 {
	if v, ok := choices[row.modName][row.id]; ok {
		return v
	}
	return row.def.Value()
}

// setOptionChoice records one choice, making the maps only when there is one.
func setOptionChoice(c *game.Config, name, id string, v int64) {
	if c.ModOptions == nil {
		c.ModOptions = map[string]map[string]int64{}
	}
	if c.ModOptions[name] == nil {
		c.ModOptions[name] = map[string]int64{}
	}
	c.ModOptions[name][id] = v
}

// clearOptionChoice puts a new choice back to its default.
func clearOptionChoice(c *game.Config, name, id string) {
	if choices := c.ModOptions[name]; choices != nil {
		delete(choices, id)
	}
}

// optionDiffs spells out the choices that differ from a mod's defaults, for
// the editor's one-line summary and the help shown while a field has focus.
func optionDiffs(c game.Config) []string {
	var out []string
	for _, row := range optionRows(c.DataDir, c.Mods, c.ModOptions) {
		v := optionChosen(row, c.ModOptions)
		if v == row.def.Value() {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s: %s -> %s", row.modName, row.label,
			mod.OptionValueWord(row.def, row.def.Value()), mod.OptionValueWord(row.def, v)))
	}
	return out
}

// modsError is why c's mod list would not load at a reset, or nil. Both
// editors refuse to save while it is not nil, since the reset that follows a
// save would refuse the list after config.json had already been written.
func modsError(c game.Config) error {
	_, _, err := mod.Resolve(c.DataDir, c.Mods, c.ModOptions)
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
func toggleMod(dataDir string, options map[string]map[string]int64, list []string, name string) ([]string, error) {
	if i := slices.Index(list, name); i >= 0 {
		return slices.Delete(slices.Clone(list), i, i+1), nil
	}
	next := append(slices.Clone(list), name)
	if _, _, err := mod.Resolve(dataDir, next, options); err != nil {
		return list, err
	}
	return next, nil
}

// modsValue is the field's one-line summary. It shares a line with the field's
// number and label, so a long list is summed up rather than spelled out.
func modsValue(c *game.Config) string {
	list := strings.Join(c.Mods, ", ")
	var value string
	switch {
	case len(c.Mods) == 0:
		value = "none"
	case len(list) > 40:
		value = fmt.Sprintf("%d mods", len(c.Mods))
	default:
		value = list
	}
	if n := len(optionDiffs(*c)); n > 0 {
		value += fmt.Sprintf(" (%d option%s changed)", n, pluralS(n))
	}
	return value
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
		for _, r := range mod.Replaced(c.DataDir, c.Mods) {
			line := fmt.Sprintf(i18n.T(sessionLang(s), "%s's %s replaces %s's, which does not run."), r.By, r.File, r.Over)
			fmt.Fprintln(s, WrapIndented(line, "  "))
		}
		for i, r := range rows {
			mark := "[ ]"
			if r.on {
				mark = "[x]"
			}
			fmt.Fprintln(s, wrapHanging(fmt.Sprintf("%2d) %s %s", i+1, mark, r.label), "  ", "          "))
		}
		optionsRow := len(rows) + 1
		fmt.Fprintf(s, "%2d) %s\n", optionsRow, i18n.T(sessionLang(s), "Options for the mods in play"))
		fmt.Fprintf(s, "   0) %s\n", i18n.T(sessionLang(s), "Quit"))
		n := pickMod(s, optionsRow)
		if n == 0 {
			return
		}
		if n == optionsRow {
			editModOptions(s, c)
			continue
		}
		r := rows[n-1]
		if r.err != nil && !r.on {
			fail(s, r.err)
			continue
		}
		next, err := toggleMod(c.DataDir, c.ModOptions, c.Mods, r.name)
		if err != nil {
			fail(s, err)
			continue
		}
		c.Mods = next
	}
}

// editModOptions is the line editor's choices for the mods in play. A number
// opens one option; a bool flips, a number asks for a value in the mod's range.
func editModOptions(s session.Session, c *game.Config) {
	rows := optionRows(c.DataDir, c.Mods, c.ModOptions)
	if len(rows) == 0 {
		ok(s, "%s", i18n.T(sessionLang(s), "None of the mods in play has options."))
		return
	}
	for {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan,
			i18n.T(sessionLang(s), "Options for the mods in play"), ansi.Reset)
		for i, row := range rows {
			v := optionChosen(row, c.ModOptions)
			fmt.Fprintln(s, wrapHanging(fmt.Sprintf("%2d) %s", i+1,
				fmt.Sprintf("%s: %s", row.modName, mod.OptionValueLine(row.def, v))), "  ", "      "))
			if row.description != "" {
				fmt.Fprintln(s, WrapIndented(row.description, "      "))
			}
		}
		n := pickOption(s, len(rows))
		if n == 0 {
			return
		}
		row := rows[n-1]
		old, existed := c.ModOptions[row.modName][row.id]
		want := optionChosen(row, c.ModOptions)
		if row.def.Type == mod.OptionBool {
			want = 1 - want
		} else {
			msg := fmt.Sprintf(i18n.T(sessionLang(s), "%s (now %s; from %d to %d)?"),
				row.label, mod.OptionValueWord(row.def, want), *row.def.Min, *row.def.Max)
			for {
				line := strings.TrimSpace(prompt(s, msg))
				if line == "" {
					return
				}
				v, err := strconv.ParseInt(line, 10, 64)
				if err != nil || v < *row.def.Min || v > *row.def.Max {
					failNoPause(s, fmt.Errorf(i18n.T(sessionLang(s), "Enter a whole number from %d to %d."),
						*row.def.Min, *row.def.Max))
					continue
				}
				want = v
				break
			}
		}
		setOptionChoice(c, row.modName, row.id, want)
		if err := modsError(*c); err != nil {
			if existed {
				setOptionChoice(c, row.modName, row.id, old)
			} else {
				clearOptionChoice(c, row.modName, row.id)
			}
			fail(s, err)
		}
	}
}

// pickOption reads a row number, 0 to quit. Option rows are usually shorter
// than the mod list, but they take a typed number throughout: the next step
// reads a whole line too, so a one-key ChoiceQuit here would leave its Enter
// behind for the value prompt to swallow.
func pickOption(s session.Session, rows int) int {
	n, err := strconv.Atoi(strings.TrimSpace(prompt(s, i18n.T(sessionLang(s), "Change which option (0 = quit)?"))))
	if err != nil || n < 0 || n > rows {
		return 0
	}
	return n
}

// modsOrderNote heads the list in both editors. A literal passed through a
// constant is invisible to scripts/gen-ui-pot.py, so the line editor names it
// again in its i18n.T call.
const modsOrderNote = "Where two mods change the same value, the one turned on later wins."

// modsReplacedNote words a mod.Replacement. Like modsOrderNote, the line editor
// names it again in its i18n.T call, where scripts/gen-ui-pot.py can see it.
const modsReplacedNote = "%s's %s replaces %s's, which does not run."

// modsNote is the TUI's line above the list: modsOrderNote, then any script a
// later mod in list replaces.
func modsNote(dataDir string, list []string) string {
	lines := []string{modsOrderNote}
	for _, r := range mod.Replaced(dataDir, list) {
		lines = append(lines, fmt.Sprintf(modsReplacedNote, r.By, r.File, r.Over))
	}
	return strings.Join(lines, " ")
}

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
	note := modsNote(c.DataDir, c.Mods)
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
			msg, top := help, modsNote(c.DataDir, order)
			if _, _, err := mod.Resolve(c.DataDir, order, c.ModOptions); err != nil {
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
	t.addModOptions(form, &c)
	return form
}

// addModOptions puts each enabled mod's choices after its checkbox. The mod's
// label and description name what it changes; the field's help carries the
// type, default and range, which is the same line -check-mod prints.
func (t *configTUI) addModOptions(form *tview.Form, c *game.Config) {
	rows := optionRows(c.DataDir, c.Mods, c.ModOptions)
	if len(rows) == 0 {
		return
	}
	form.AddFormItem(tview.NewTextView().SetText(tview.Escape("Options for the mods in play")).SetSize(1, 0))
	for _, row := range rows {
		label := noStar + row.modName + ": " + row.label
		help := row.line
		if row.description != "" {
			help = row.description + "\n" + help
		}
		opening := optionChosen(row, c.ModOptions)
		if row.def.Type == mod.OptionBool {
			b := tview.NewCheckbox().SetLabel(label).SetChecked(opening != 0)
			b.SetFocusFunc(func() { t.showHelp(help) })
			form.AddFormItem(zebra(form, b))
			t.resetters = append(t.resetters, func() { b.SetChecked(opening != 0) })
			t.binders = append(t.binders, func(c *game.Config) {
				var v int64
				if b.IsChecked() {
					v = 1
				}
				setOptionChoice(c, row.modName, row.id, v)
			})
			continue
		}
		f := tview.NewInputField().SetLabel(label).SetText(strconv.FormatInt(opening, 10)).
			SetFieldWidth(12).SetAcceptanceFunc(tview.InputFieldInteger)
		f.SetFocusFunc(func() { t.showHelp(help) })
		valid := func(text string) (int64, bool) {
			v, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
			return v, err == nil && v >= *row.def.Min && v <= *row.def.Max
		}
		f.SetDoneFunc(func(tcell.Key) {
			if v, ok := valid(f.GetText()); ok {
				f.SetText(strconv.FormatInt(v, 10))
				return
			}
			f.SetText(strconv.FormatInt(opening, 10))
			t.showOptionError(fmt.Sprintf("%s must be from %d to %d.", row.label, *row.def.Min, *row.def.Max))
		})
		form.AddFormItem(zebra(form, f))
		t.resetters = append(t.resetters, func() { f.SetText(strconv.FormatInt(opening, 10)) })
		t.binders = append(t.binders, func(c *game.Config) {
			if v, ok := valid(f.GetText()); ok {
				setOptionChoice(c, row.modName, row.id, v)
			}
		})
	}
}

func (t *configTUI) showOptionError(msg string) {
	t.showModal(tview.NewModal().SetText(msg).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { t.dismissModal() }))
}
