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
//
// Ctrl+plus and Ctrl+minus (Cmd on macOS) make everything larger or smaller,
// as the A+ and A− buttons do, and Ctrl+0 goes back to the default size.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
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
	saved := loadSession()
	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = saved.Dirs
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Immortal Barons Sysop Panel"), app.Size(unit.Dp(1100), unit.Dp(720)))
		u := newUI(w, dirs, saved.Zoom)
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

	openBtn widget.Clickable
	// zoom scales the whole panel, text and spacing alike, so a larger size
	// never crowds a button's label against its edge.
	zoom            float32
	zoomIn, zoomOut widget.Clickable
	browser         *browser
	browsing        bool
	notice          string // a message shown above the tabs, e.g. an Open refusal
	// missing is the saved directories that would not open at startup. They
	// stay in the session, since the usual cause is a drive not mounted yet,
	// until one opens again or Forget is clicked.
	missing   []string
	forgetBtn widget.Clickable
}

func newUI(w *app.Window, dirs []string, zoom float32) *ui {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	u := &ui{win: w, th: th, browser: newBrowser(), zoom: nearestZoom(zoom)}
	// Each open clears the notice on success, so collect the failures and show
	// them together once every saved directory has been tried.
	var failed []string
	for _, d := range dirs {
		if !u.open(d) {
			failed = append(failed, u.notice)
			u.missing = append(u.missing, d)
		}
	}
	u.notice = strings.Join(failed, "; ")
	if len(u.missing) > 0 {
		u.notice += " (kept for next time)"
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
	u.missing = slices.DeleteFunc(u.missing, func(m string) bool { return m == dir || m == abs })
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
	if i < u.active {
		u.active-- // keep showing the same board, now one place to the left
	}
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

	u.zoomKeys(gtx)
	if u.zoomIn.Clicked(gtx) {
		u.setZoom(+1)
	}
	if u.zoomOut.Clicked(gtx) {
		u.setZoom(-1)
	}
	gtx.Metric.PxPerDp *= u.zoom
	gtx.Metric.PxPerSp *= u.zoom

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
	if u.forgetBtn.Clicked(gtx) {
		u.missing, u.notice = nil, ""
		u.saveSession()
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
				if len(u.missing) == 0 {
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, l.Layout)
				}
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, l.Layout),
						button(u.th, &u.forgetBtn, "Forget"),
					)
				})
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
	items = append(items,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.zoom <= zoomSteps[0] {
				gtx = gtx.Disabled()
			}
			return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, material.Button(u.th, &u.zoomOut, "A−").Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(u.th, fmt.Sprintf("%d%%", int(u.zoom*100+0.5)))
			return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(8)}.Layout(gtx, l.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.zoom >= zoomSteps[len(zoomSteps)-1] {
				gtx = gtx.Disabled()
			}
			return material.Button(u.th, &u.zoomIn, "A+").Layout(gtx)
		}),
	)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
}

// sessionFile remembers which directories were open, and the size, so the
// panel comes back as it was left. It lives in the user's config directory, never a data
// directory.
func sessionFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "immortal-barons", "ib-sysop.json")
}

// zoomSteps are the sizes A+ and A− step through. defaultZoom is larger than
// Gio's own size, which reads small on a desktop monitor.
var zoomSteps = []float32{0.75, 0.9, 1, 1.1, 1.25, 1.4, 1.6, 1.8, 2, 2.5, 3}

const defaultZoom = 1.25

// nearestZoom is the step closest to z, and the default for an unset z.
func nearestZoom(z float32) float32 {
	if z <= 0 {
		return defaultZoom
	}
	best := zoomSteps[0]
	for _, s := range zoomSteps {
		if abs(s-z) < abs(best-z) {
			best = s
		}
	}
	return best
}

func abs(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// setZoom moves one step larger (+1) or smaller (-1); 0 goes back to the
// default. Called with ui.mu held.
func (u *ui) setZoom(dir int) {
	z := u.zoom
	if dir == 0 {
		z = defaultZoom
	} else {
		for i, s := range zoomSteps {
			if s == u.zoom {
				z = zoomSteps[min(max(i+dir, 0), len(zoomSteps)-1)]
			}
		}
	}
	if z != u.zoom {
		u.zoom = z
		u.saveSession()
	}
}

// zoomKeys takes the shortcut keys wherever the focus is. Plus is matched
// with and without Shift, since on most layouts it shares a key with "=".
func (u *ui) zoomKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Required: key.ModShortcut, Optional: key.ModShift, Name: "+"},
			key.Filter{Required: key.ModShortcut, Optional: key.ModShift, Name: "="},
			key.Filter{Required: key.ModShortcut, Optional: key.ModShift, Name: "-"},
			key.Filter{Required: key.ModShortcut, Name: "0"},
		)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case "+", "=":
			u.setZoom(+1)
		case "-":
			u.setZoom(-1)
		case "0":
			u.setZoom(0)
		}
	}
}

// session is what the panel remembers between runs.
type session struct {
	Dirs []string `json:"dirs"`
	Zoom float32  `json:"zoom,omitempty"`
}

func loadSession() session {
	var s session
	if p := sessionFile(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			json.Unmarshal(data, &s)
		}
	}
	return s
}

// saveSession is best effort: losing it costs reopening a tab by hand.
func (u *ui) saveSession() {
	p := sessionFile()
	if p == "" {
		return
	}
	s := session{Dirs: make([]string, 0, len(u.tabs)), Zoom: u.zoom}
	for _, t := range u.tabs {
		s.Dirs = append(s.Dirs, t.dir)
	}
	s.Dirs = append(s.Dirs, u.missing...)
	data, _ := json.Marshal(s)
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		os.WriteFile(p, data, 0o644)
	}
}
