// Command ib-sysop is a desktop window for a sysop or League Coordinator: it
// shows each board's league state and runs the game's maintenance commands.
// It is not for playing the game.
//
// Each open data directory is a tab. The panel only ever reads a world
// (internal/sysop); every action runs the immortal-barons binary.
//
//	ib-sysop [DATADIR ...]
//
// With no arguments it reopens the directories that were open last time.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// mono is the face the tables and the log use, so columns line up.
const mono = font.Typeface("Go Mono")

// autoRefreshEvery is how often a tab with auto-refresh on re-reads its world.
// Every read takes the exclusive world lock the door nodes queue on, so this is
// deliberately slow.
const autoRefreshEvery = 60 * time.Second

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: ib-sysop [DATADIR ...]\n\n"+
			"Shows each game data directory's league state and runs its maintenance\n"+
			"commands. With no directory, reopens the ones open last time.\n")
	}
	flag.Parse()
	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = loadSession()
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Immortal Barons Sysop Panel"), app.Size(unit.Dp(1100), unit.Dp(720)))
		u := newUI(w, dirs)
		if err := u.loop(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

type ui struct {
	win *app.Window
	th  *material.Theme

	mu     sync.Mutex // guards what background work writes into the tabs
	tabs   []*boardTab
	active int

	openBtn  widget.Clickable
	browser  *browser
	browsing bool
	notice   string // a message shown above the tabs, e.g. an Open refusal
}

func newUI(w *app.Window, dirs []string) *ui {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	u := &ui{win: w, th: th, browser: newBrowser()}
	for _, d := range dirs {
		u.open(d)
	}
	if len(u.tabs) == 0 {
		u.browsing = true
	}
	go u.tick()
	return u
}

// tick wakes the window now and then so auto-refresh can come due without the
// mouse moving.
func (u *ui) tick() {
	for range time.Tick(5 * time.Second) {
		u.win.Invalidate()
	}
}

// open adds a tab for dir, or switches to the one already showing it.
func (u *ui) open(dir string) bool {
	abs, err := sysop.Open(dir)
	if err != nil {
		u.notice = fmt.Sprintf("%s: %v", dir, err)
		return false
	}
	u.notice = ""
	for i, t := range u.tabs {
		if t.dir == abs {
			u.active = i
			return true
		}
	}
	t := newBoardTab(u, abs)
	u.tabs = append(u.tabs, t)
	u.active = len(u.tabs) - 1
	t.refresh()
	u.saveSession()
	return true
}

func (u *ui) close(i int) {
	t := u.tabs[i]
	if t.running() {
		u.notice = filepath.Base(t.dir) + ": a command is still running; stop it before closing the tab"
		return
	}
	u.tabs = append(u.tabs[:i], u.tabs[i+1:]...)
	if u.active >= len(u.tabs) {
		u.active = len(u.tabs) - 1
	}
	if u.active < 0 {
		u.active = 0
	}
	u.saveSession()
}

func (u *ui) loop() error {
	var ops op.Ops
	for {
		switch e := u.win.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (u *ui) layout(gtx layout.Context) layout.Dimensions {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.openBtn.Clicked(gtx) {
		u.browsing = !u.browsing
	}
	for i := range u.tabs {
		if u.tabs[i].closeBtn.Clicked(gtx) {
			u.close(i)
			break
		}
		if u.tabs[i].tabBtn.Clicked(gtx) {
			u.active, u.browsing = i, false
		}
	}
	for _, t := range u.tabs {
		t.maybeAutoRefresh()
	}

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(u.tabBar),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.notice == "" {
					return layout.Dimensions{}
				}
				l := material.Body2(u.th, u.notice)
				l.Color = errorColor
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
			}),
			layout.Rigid(rule(u.th)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if u.browsing || len(u.tabs) == 0 {
					return u.browser.layout(gtx, u)
				}
				return u.tabs[u.active].layout(gtx)
			}),
		)
	})
}

// tabBar is one button per open data directory, each with its own close
// button, and Open… at the end.
func (u *ui) tabBar(gtx layout.Context) layout.Dimensions {
	var items []layout.FlexChild
	for i, t := range u.tabs {
		i, t := i, t
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			selected := i == u.active && !u.browsing
			return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(tabButton(u.th, &t.tabBtn, t.title(), selected)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						b := material.Button(u.th, &t.closeBtn, "×")
						b.Background = color.NRGBA{A: 0}
						b.Color = u.th.Fg
						b.Inset = layout.UniformInset(unit.Dp(6))
						return b.Layout(gtx)
					}),
				)
			})
		}))
	}
	items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return tabButton(u.th, &u.openBtn, "Open…", u.browsing || len(u.tabs) == 0)(gtx)
	}))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
}

// sessionFile remembers which directories were open, so the panel comes back
// as it was left. It lives in the user's config directory, never a data
// directory.
func sessionFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "immortal-barons", "ib-sysop.json")
}

func loadSession() []string {
	var dirs []string
	if p := sessionFile(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			json.Unmarshal(data, &dirs)
		}
	}
	return dirs
}

// saveSession is best effort: losing it costs reopening a tab by hand.
func (u *ui) saveSession() {
	p := sessionFile()
	if p == "" {
		return
	}
	dirs := make([]string, 0, len(u.tabs))
	for _, t := range u.tabs {
		dirs = append(dirs, t.dir)
	}
	data, _ := json.Marshal(dirs)
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		os.WriteFile(p, data, 0o644)
	}
}
