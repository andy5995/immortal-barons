package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// newButton is a material button with half of Gio's default padding, which
// left the toolbar and the tab row taller than their labels need.
func newButton(th *material.Theme, c *widget.Clickable, label string) material.ButtonStyle {
	b := material.Button(th, c, label)
	b.Inset = layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(6), Right: unit.Dp(6)}
	return b
}

// tabButton is a tab: filled when selected, flat when not. The selected tab is
// also the only one drawn in bold, so the choice never rests on color alone.
func tabButton(th *material.Theme, c *widget.Clickable, label string, selected bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := newButton(th, c, label)
		b.Inset.Top, b.Inset.Bottom = unit.Dp(3), unit.Dp(3) // a tab sits lower than a button
		if !selected {
			b.Background = pal.tabBg
			b.Color = th.Fg
		} else {
			b.Font.Weight = 700
		}
		return b.Layout(gtx)
	}
}

// rule is a thin horizontal line with a little space either side.
func rule(th *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(1)))
			defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
			paint.ColorOp{Color: pal.rule}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: size}
		})
	}
}

// button is a newButton with a little space after it.
func button(th *material.Theme, c *widget.Clickable, label string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, newButton(th, c, label).Layout)
	})
}

// label is a body label as a flex child.
func label(th *material.Theme, s string) layout.FlexChild {
	return layout.Rigid(material.Body2(th, s).Layout)
}
