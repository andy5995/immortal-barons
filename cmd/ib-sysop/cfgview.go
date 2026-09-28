package main

import (
	"os"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// cfgView shows the board's bbs.cfg and opens it in the desktop's default
// editor. The panel does not edit the file itself: the editor saves it, and the
// view notices the new modification time and reads it again.
type cfgView struct {
	t *boardTab

	editBtn widget.Clickable
	list    widget.List

	loaded  bool
	exists  bool
	mtime   time.Time
	lines   []string
	readErr error

	status    string
	statusErr bool
}

func (c *cfgView) init(t *boardTab) {
	c.t = t
	c.list.Axis = layout.Vertical
}

func (c *cfgView) path() string { return sysop.BoardConfigPath(c.t.dir) }

// check re-reads the file when it has appeared, gone or changed since the last
// look. A change after the first read re-reads the board too, since bbs.cfg
// names it. Called with ui.mu held.
func (c *cfgView) check() {
	fi, err := os.Stat(c.path())
	exists := err == nil
	var mtime time.Time
	if exists {
		mtime = fi.ModTime()
	}
	if c.loaded && exists == c.exists && mtime.Equal(c.mtime) {
		return
	}
	changed := c.loaded
	c.loaded, c.exists, c.mtime = true, exists, mtime
	c.lines, c.readErr = nil, nil
	if exists {
		data, err := os.ReadFile(c.path())
		if err != nil {
			c.readErr = err
		} else {
			c.lines = strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
		}
	} else if err != nil && !os.IsNotExist(err) {
		c.readErr = err
	}
	if changed {
		c.status, c.statusErr = "bbs.cfg changed on disk and was read again.", false
		c.t.refresh()
	}
}

// edit opens the file, writing it from the game's template first if the board
// has none. The opener can take a moment, so it runs off the UI goroutine.
// Called with ui.mu held.
func (c *cfgView) edit() {
	created, err := sysop.EnsureBoardConfig(c.t.dir)
	if err != nil {
		c.status, c.statusErr = "Could not create bbs.cfg: "+err.Error(), true
		return
	}
	c.status, c.statusErr = "Opening bbs.cfg in the default editor…", false
	if created {
		c.status = "This board had no bbs.cfg; it was written with the settings in force. Opening it…"
	}
	path, u := c.path(), c.t.u
	go func() {
		err := openInEditor(path)
		u.mu.Lock()
		if err != nil {
			c.status, c.statusErr = err.Error(), true
		} else {
			c.status, c.statusErr = "Opened in the default editor. Save there; this view reloads the file when it changes.", false
			if created {
				c.status = "This board had no bbs.cfg; it was written with the settings in force. " + c.status
			}
		}
		u.mu.Unlock()
		u.win.Invalidate()
	}()
}

func (c *cfgView) layout(gtx layout.Context) layout.Dimensions {
	th := c.t.u.th
	if c.editBtn.Clicked(gtx) {
		c.edit()
	}
	c.check()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				button(th, &c.editBtn, "Edit in default editor"),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, c.path())
					l.Color, l.MaxLines = pal.dim, 1
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if c.status == "" {
				return layout.Dimensions{}
			}
			l := material.Body2(th, c.status)
			if c.statusErr {
				l.Color = pal.err
			}
			return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, l.Layout)
		}),
		label(th, "The game reads bbs.cfg at the start of every run; a change takes effect on the next one."),
		layout.Rigid(rule(th)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch {
			case c.readErr != nil:
				l := material.Body2(th, "Could not read bbs.cfg: "+c.readErr.Error())
				l.Color = pal.err
				return l.Layout(gtx)
			case !c.exists:
				return material.Body2(th, "This board has no bbs.cfg. Edit writes one from the settings it is running with.").Layout(gtx)
			}
			return material.List(th, &c.list).Layout(gtx, len(c.lines), func(gtx layout.Context, i int) layout.Dimensions {
				l := material.Body2(th, c.lines[i])
				l.Font.Typeface = mono
				return l.Layout(gtx)
			})
		}),
	)
}
