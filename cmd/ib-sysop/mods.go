package main

import (
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/mod"
	"github.com/andy5995/immortal-barons/internal/sysop"
)

// modsView shows the mods a board's game plays and the folders in its mods
// folder, each with a button that runs -check-mod on it. It changes nothing:
// mods are chosen in the settings editor -reset opens.
type modsView struct {
	t     *boardTab
	check map[string]*widget.Clickable
	list  widget.List
}

func (m *modsView) init(t *boardTab) {
	m.t = t
	m.check = map[string]*widget.Clickable{}
	m.list.Axis = layout.Vertical
}

// playing is the line naming the mods the game plays.
func playing(mods []sysop.Mod) string {
	if len(mods) == 0 {
		return "This game plays: none (the base game)."
	}
	s := "This game plays: "
	for i, p := range mods {
		if i > 0 {
			s += ", "
		}
		s += p.Display
		if p.Version != "" {
			s += " " + p.Version
		}
	}
	return s + "."
}

// foundLine is one folder in the mods folder, in words.
func foundLine(f sysop.Mod) string {
	if f.Err != nil {
		return f.Name + ": does not load: " + f.Err.Error()
	}
	s := f.Name + "  version " + f.Version
	if f.Display != f.Name {
		s += "  (" + f.Display + ")"
	}
	return s
}

func (m *modsView) layout(gtx layout.Context) layout.Dimensions {
	th := m.t.u.th
	snap := m.t.snap
	if snap == nil {
		return material.Body2(th, m.t.emptyText("")).Layout(gtx)
	}
	busy := m.t.running()
	for _, f := range snap.Found {
		if c := m.check[f.Name]; c != nil && c.Clicked(gtx) && !busy {
			m.t.view = viewRun
			// A full path: -check-mod tries a bare name against its working
			// directory first, where a same-named folder could stand in for it.
			m.t.run.start(sysop.CheckMod, filepath.Join(m.t.dir, mod.Dir, f.Name))
		}
	}

	rows := []layout.FlexChild{
		label(th, playing(snap.Playing)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(th, "Mods change only at a reset, in the settings editor that -reset opens.")
			l.Color = pal.dim
			return l.Layout(gtx)
		}),
		layout.Rigid(rule(th)),
		label(th, "Found in "+filepath.Join(snap.DataDir, mod.Dir)+":"),
	}
	if snap.FoundErr != nil {
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, "Could not list the mods folder: "+snap.FoundErr.Error())
			l.Color = pal.err
			return l.Layout(gtx)
		}))
	}
	if len(snap.Found) == 0 && snap.FoundErr == nil {
		rows = append(rows, label(th, "No mod folders."))
	}
	rows = append(rows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &m.list).Layout(gtx, len(snap.Found), func(gtx layout.Context, i int) layout.Dimensions {
			f := snap.Found[i]
			c := m.check[f.Name]
			if c == nil {
				c = new(widget.Clickable)
				m.check[f.Name] = c
			}
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if busy {
							gtx = gtx.Disabled()
						}
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, newButton(th, c, "Check").Layout)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						l := material.Body2(th, foundLine(f))
						if f.Err != nil {
							l.Color = pal.err
						}
						return l.Layout(gtx)
					}),
				)
			})
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
