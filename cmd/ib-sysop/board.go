package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// The views inside a board's tab.
const (
	viewBoards = iota
	viewInFlight
	viewHeld
	viewRun
	viewConfig
)

var viewNames = [...]string{"Boards", "In flight", "Held packets", "Run", "bbs.cfg"}

// boardTab is one open data directory. Everything a background read or a
// running command writes into it is guarded by ui.mu, which the frame holds.
type boardTab struct {
	u   *ui
	dir string

	tabBtn, closeBtn widget.Clickable

	snap     *sysop.Snapshot
	readErr  error
	reading  bool
	lastRead time.Time

	refreshBtn widget.Clickable
	auto       widget.Bool
	view       int
	viewBtns   [len(viewNames)]widget.Clickable

	boards, inFlight, held *table

	run runView
	cfg cfgView
}

func newBoardTab(u *ui, dir string) *boardTab {
	t := &boardTab{u: u, dir: dir,
		boards:   newTable("#", "BBS Name", "Last heard", "Silent", "Version", "Round trip", "Probe back", "Status"),
		inFlight: newTable("ID", "Kind", "What", "Owner", "Target board", "Target realm", "Launch day", "Waiting", "Held days", "Lost-forces return"),
		held:     newTable("File", "From board", "Type", "Reason", "Arrived", "Expires", "Pauses lost forces"),
	}
	t.run.init(t)
	t.cfg.init(t)
	return t
}

// title is the tab's label: the board's name once known, else the folder.
func (t *boardTab) title() string {
	name := filepath.Base(filepath.Dir(t.dir)) + string(filepath.Separator) + filepath.Base(t.dir)
	if t.snap != nil && t.snap.BoardID != "" {
		name = t.snap.BoardID
	}
	if t.reading {
		name += " …"
	}
	return name
}

func (t *boardTab) running() bool { return t.run.proc != nil }

// refresh reads the world off the UI goroutine. The lock it takes is the one a
// running -maint holds, so a read can wait for as long as that run takes.
// Called with ui.mu held.
func (t *boardTab) refresh() {
	if t.reading {
		return
	}
	t.reading = true
	go func() {
		s, err := sysop.Read(t.dir, time.Now())
		t.u.mu.Lock()
		t.reading, t.lastRead = false, time.Now()
		if err == nil {
			t.snap = &s
		}
		t.readErr = err
		t.u.mu.Unlock()
		t.u.win.Invalidate()
	}()
}

func (t *boardTab) maybeAutoRefresh() {
	if t.auto.Value && !t.reading && !t.running() && time.Since(t.lastRead) >= autoRefreshEvery {
		t.refresh()
	}
}

func (t *boardTab) layout(gtx layout.Context) layout.Dimensions {
	th := t.u.th
	if t.refreshBtn.Clicked(gtx) {
		t.refresh()
	}
	for i := range t.viewBtns {
		if t.viewBtns[i].Clicked(gtx) {
			t.view = i
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, t.dir)
					l.MaxLines = 1
					return l.Layout(gtx)
				}),
				button(th, &t.refreshBtn, "Refresh"),
				layout.Rigid(material.CheckBox(th, &t.auto, fmt.Sprintf("Auto-refresh every %d s", int(autoRefreshEvery.Seconds()))).Layout),
			)
		}),
		layout.Rigid(t.summary),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var items []layout.FlexChild
			for i, n := range viewNames {
				i, n := i, n
				items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, tabButton(th, &t.viewBtns[i], n, i == t.view))
				}))
			}
			return layout.Flex{}.Layout(gtx, items...)
		}),
		layout.Rigid(rule(th)),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			switch t.view {
			case viewBoards:
				return t.boards.layout(gtx, th, t.boardRows(), t.emptyText("No other boards are known yet."))
			case viewInFlight:
				return t.inFlight.layout(gtx, th, t.inFlightRows(), t.emptyText("Nothing is in flight from this board."))
			case viewHeld:
				return t.held.layout(gtx, th, t.heldRows(), t.emptyText("No packets are held."))
			case viewConfig:
				return t.cfg.layout(gtx)
			default:
				return t.run.layout(gtx)
			}
		}),
	)
}

func (t *boardTab) emptyText(s string) string {
	if t.snap == nil {
		return "Not read yet."
	}
	return s
}

// summary is the line under the path: the board, its role and game day, and
// when it was read. A failed read says so here and keeps the last good data.
func (t *boardTab) summary(gtx layout.Context) layout.Dimensions {
	th := t.u.th
	var s string
	switch {
	case t.snap == nil && t.reading:
		s = "Reading… (waiting for the world lock if a run holds it)"
	case t.snap == nil:
		s = ""
	default:
		p := t.snap
		role := "stand-alone board"
		switch {
		case p.Coordinator:
			role = "League Coordinator (node #1)"
		case p.League:
			role = "league member"
		}
		s = fmt.Sprintf("%s: %s, game day %d. Read %s.", p.BoardID, role, p.GameDay, p.Taken.Format("2006-01-02 15:04:05"))
		if p.MinVersion != "" {
			s += " League minimum v" + p.MinVersion + "."
		}
		if p.OwnRulesDiffer {
			s += " THIS BOARD IS PLAYING RULES THE COORDINATOR HAS NOT SENT."
		}
	}
	children := []layout.FlexChild{label(th, s)}
	if t.readErr != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(th, "Read failed: "+t.readErr.Error())
			l.Color = pal.err
			return l.Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func days(n int) string {
	if n == 1 {
		return "1 day"
	}
	return strconv.Itoa(n) + " days"
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func (t *boardTab) boardRows() [][]string {
	if t.snap == nil {
		return nil
	}
	var rows [][]string
	for _, b := range t.snap.Boards {
		silent := "-"
		if b.SilentDays >= 0 {
			silent = days(b.SilentDays)
		}
		ver := "unknown"
		if b.Version != "" {
			ver = "v" + b.Version
		}
		trip := "-"
		if b.RoundTrip > 0 {
			trip = fmt.Sprintf("%.1f days", b.RoundTrip)
		}
		rows = append(rows, []string{strconv.Itoa(b.Number), b.Name, stamp(b.LastHeard), silent, ver,
			trip, stamp(b.ProbeBack), b.Status()})
	}
	return rows
}

func (t *boardTab) inFlightRows() [][]string {
	if t.snap == nil {
		return nil
	}
	var rows [][]string
	for _, f := range t.snap.InFlight {
		ret := "never (recovery off)"
		if f.Recovers {
			switch {
			case f.DaysLeft <= 0:
				ret = "next run"
			default:
				ret = "in " + days(f.DaysLeft)
			}
			if f.Held {
				ret += " (paused: board held)"
			}
		}
		rows = append(rows, []string{strconv.Itoa(f.ID), f.Kind, f.What, f.Owner, f.TargetBoard, f.TargetRealm,
			strconv.Itoa(f.LaunchedDay), days(f.Waiting), strconv.Itoa(f.HeldDays), ret})
	}
	return rows
}

func (t *boardTab) heldRows() [][]string {
	if t.snap == nil {
		return nil
	}
	var rows [][]string
	for _, h := range t.snap.Held {
		from, typ := h.FromBoard, h.Type
		if from == "" {
			from, typ = "?", "?"
		}
		pauses := "no"
		if h.PausesLostForces {
			pauses = "yes"
		}
		rows = append(rows, []string{h.File, from, typ, string(h.Reason), stamp(h.Arrived), stamp(h.Expires), pauses})
	}
	return rows
}
