package main

import (
	"os"
	"path/filepath"
	"sort"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// browser picks a data directory. Gio has no native folder dialog, so this is
// a small one of its own: a path to type or paste, and the folders under it to
// click through. A folder Open would accept is marked as game data.
type browser struct {
	path   widget.Editor
	up     widget.Clickable
	open   widget.Clickable
	list   widget.List
	dirs   []dirEntry
	listed string // the path dirs was read from
}

type dirEntry struct {
	name  string
	game  bool
	click widget.Clickable
}

func newBrowser() *browser {
	b := &browser{}
	b.path.SingleLine = true
	b.path.Submit = true
	b.list.Axis = layout.Vertical
	start, err := os.Getwd()
	if err != nil {
		start, _ = os.UserHomeDir()
	}
	b.path.SetText(start)
	return b
}

func (b *browser) read(dir string) {
	b.listed = dir
	b.dirs = nil
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}
		_, err := sysop.Open(filepath.Join(dir, e.Name()))
		b.dirs = append(b.dirs, dirEntry{name: e.Name(), game: err == nil})
	}
	sort.Slice(b.dirs, func(i, j int) bool { return b.dirs[i].name < b.dirs[j].name })
}

func (b *browser) layout(gtx layout.Context, u *ui) layout.Dimensions {
	th := u.th
	for {
		ev, ok := b.path.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok && u.open(b.path.Text()) {
			u.browsing = false
		}
	}
	if b.up.Clicked(gtx) {
		b.path.SetText(filepath.Dir(filepath.Clean(b.path.Text())))
	}
	if b.open.Clicked(gtx) && u.open(b.path.Text()) {
		u.browsing = false
	}
	for i := range b.dirs {
		if b.dirs[i].click.Clicked(gtx) {
			b.path.SetText(filepath.Join(b.listed, b.dirs[i].name))
		}
	}
	if cur := filepath.Clean(b.path.Text()); cur != b.listed {
		if fi, err := os.Stat(cur); err == nil && fi.IsDir() {
			b.read(cur)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, "Open a game data directory").Layout),
		layout.Rigid(material.Body2(th, "The folder the game's -data flag names, or the door folder holding it. "+
			"Folders marked [game data] can be opened.").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				button(th, &b.up, "Up"),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return widget.Border{Color: pal.border, Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(6)).Layout(gtx, material.Editor(th, &b.path, "path").Layout)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				button(th, &b.open, "Open"),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(b.dirs) == 0 {
				return material.Body2(th, "(no folders here)").Layout(gtx)
			}
			return material.List(th, &b.list).Layout(gtx, len(b.dirs), func(gtx layout.Context, i int) layout.Dimensions {
				d := &b.dirs[i]
				label := d.name + string(filepath.Separator)
				if d.game {
					label += "   [game data]"
				}
				return material.Clickable(gtx, &d.click, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(4)).Layout(gtx, material.Body1(th, label).Layout)
				})
			})
		}),
	)
}
