package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// maxLogLines bounds the log pane, so a long -detailed run cannot grow it
// without limit.
const maxLogLines = 5000

// runView runs one game command for a board and shows its output.
type runView struct {
	t *boardTab

	choice   widget.Enum
	detailed widget.Bool
	arg      widget.Editor
	program  widget.Editor

	runBtn, stopBtn, copyBtn, clearBtn widget.Clickable
	yesBtn, noBtn                      widget.Clickable
	confirming                         *sysop.Command

	proc  *sysop.Run
	log   []string
	list  widget.List
	cmdsL widget.List
}

func (r *runView) init(t *boardTab) {
	r.t = t
	r.choice.Value = sysop.Commands[0].Flag
	r.arg.SingleLine = true
	r.program.SingleLine = true
	r.program.SetText(sysop.GameProgram())
	r.list.Axis = layout.Vertical
	r.list.ScrollToEnd = true
	r.cmdsL.Axis = layout.Vertical
}

// offered is the commands this board may run: the Coordinator's only on node #1.
func (r *runView) offered() []sysop.Command {
	coord := r.t.snap != nil && r.t.snap.Coordinator
	var out []sysop.Command
	for _, c := range sysop.Commands {
		if !c.Coordinator || coord {
			out = append(out, c)
		}
	}
	return out
}

func (r *runView) selected() *sysop.Command {
	for _, c := range r.offered() {
		if c.Flag == r.choice.Value {
			return &c
		}
	}
	return nil
}

func (r *runView) say(format string, a ...any) {
	r.log = append(r.log, fmt.Sprintf(format, a...))
	if n := len(r.log) - maxLogLines; n > 0 {
		r.log = r.log[n:]
	}
}

// start runs c. Called with ui.mu held.
func (r *runView) start(c sysop.Command) {
	args, err := c.Args(r.t.dir, r.detailed.Value, r.arg.Text())
	if err != nil {
		r.say("! %v", err)
		return
	}
	prog := strings.TrimSpace(r.program.Text())
	if prog == "" {
		r.say("! No immortal-barons program: put its path in the Program box.")
		return
	}
	r.say("$ %s %s", prog, strings.Join(args, " "))
	u := r.t.u
	proc, err := sysop.Start(prog, r.t.dir, args, func(line string) {
		u.mu.Lock()
		r.say("%s", line)
		u.mu.Unlock()
		u.win.Invalidate()
	})
	if err != nil {
		r.say("! %v", err)
		return
	}
	r.proc = proc
	began := time.Now()
	go func() {
		err := proc.Err()
		u.mu.Lock()
		if err != nil {
			r.say("! %s ended: %v (%s)", c.Flag, err, time.Since(began).Round(time.Second))
		} else {
			r.say("  %s finished (%s)", c.Flag, time.Since(began).Round(time.Second))
		}
		r.proc = nil
		// What the command changed is what the sysop wants to see next.
		r.t.refresh()
		u.mu.Unlock()
		u.win.Invalidate()
	}()
}

func (r *runView) layout(gtx layout.Context) layout.Dimensions {
	th := r.t.u.th
	if r.choice.Update(gtx) {
		r.confirming = nil // a confirmation is for the command it named
	}
	sel := r.selected()
	if sel == nil {
		// A Coordinator command stays chosen only while the board is node #1.
		r.choice.Value = sysop.Commands[0].Flag
		sel = r.selected()
	}
	if r.runBtn.Clicked(gtx) && r.proc == nil {
		// A missing or malformed value is refused before the confirmation, so
		// the sysop is not asked to confirm a run that cannot happen.
		if _, err := sel.Args(r.t.dir, false, r.arg.Text()); err != nil {
			r.say("! %v", err)
		} else if sel.Confirm != "" {
			r.confirming = sel
		} else {
			r.start(*sel)
		}
	}
	if r.yesBtn.Clicked(gtx) && r.confirming != nil && r.proc == nil {
		r.start(*r.confirming)
		r.confirming = nil
	}
	if r.noBtn.Clicked(gtx) {
		r.confirming = nil
	}
	if r.stopBtn.Clicked(gtx) && r.proc != nil {
		r.say("! stopping…")
		r.proc.Stop()
	}
	if r.clearBtn.Clicked(gtx) {
		r.log = nil
	}
	if r.copyBtn.Clicked(gtx) {
		copyText(gtx, strings.Join(r.log, "\n")+"\n")
	}

	cmds := r.offered()
	left := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(360)))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return material.List(th, &r.cmdsL).Layout(gtx, len(cmds), func(gtx layout.Context, i int) layout.Dimensions {
			c := cmds[i]
			label := c.Flag
			if c.Coordinator {
				label += "  (Coordinator)"
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.RadioButton(th, &r.choice, c.Flag, label).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(th, c.Help)
					l.Color = pal.dim
					return layout.Inset{Left: unit.Dp(32), Bottom: unit.Dp(4)}.Layout(gtx, l.Layout)
				}),
			)
		})
	}

	controls := func(gtx layout.Context) layout.Dimensions {
		var rows []layout.FlexChild
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				label(th, "Program: "),
				layout.Flexed(1, boxed(th, &r.program, "path to immortal-barons")),
			)
		}))
		if sel.Arg != "" {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					label(th, sel.Flag+" takes a "+sel.Arg+": "),
					layout.Flexed(1, boxed(th, &r.arg, sel.Arg)),
				)
			}))
		}
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !sel.Detailed {
				gtx = gtx.Disabled()
			}
			return material.CheckBox(th, &r.detailed, "Detailed: show each packet as it is read and written").Layout(gtx)
		}))
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if r.confirming != nil {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(th, r.confirming.Confirm+" Run "+r.confirming.Flag+"?")
						l.Color = pal.err
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx, button(th, &r.yesBtn, "Yes, run it"), button(th, &r.noBtn, "Cancel"))
					}),
				)
			}
			busy := r.proc != nil
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if busy {
						gtx = gtx.Disabled()
					}
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, newButton(th, &r.runBtn, "Run "+sel.Flag).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !busy {
						gtx = gtx.Disabled()
					}
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, newButton(th, &r.stopBtn, "Stop").Layout)
				}),
				button(th, &r.copyBtn, "Copy log"),
				button(th, &r.clearBtn, "Clear log"),
			)
		}))
		rows = append(rows, layout.Rigid(rule(th)))
		rows = append(rows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &r.list).Layout(gtx, len(r.log), func(gtx layout.Context, i int) layout.Dimensions {
				l := material.Body2(th, r.log[i])
				l.Font.Typeface = mono
				if strings.HasPrefix(r.log[i], "! ") {
					l.Color = pal.err
				}
				return l.Layout(gtx)
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	}

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(left),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Flexed(1, controls),
	)
}

// boxed is a bordered single-line editor.
func boxed(th *material.Theme, e *widget.Editor, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return widget.Border{Color: pal.border, Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, material.Editor(th, e, hint).Layout)
			})
		})
	}
}
