package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	errorColor = color.NRGBA{R: 0xb0, G: 0x10, B: 0x10, A: 0xff}
	dimColor   = color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xff}
	ruleColor  = color.NRGBA{R: 0xc8, G: 0xc8, B: 0xc8, A: 0xff}
)

// tabButton is a tab: filled when selected, flat when not. The selected tab is
// also the only one drawn in bold, so the choice never rests on color alone.
func tabButton(th *material.Theme, c *widget.Clickable, label string, selected bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := material.Button(th, c, label)
		b.Inset = layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}
		if !selected {
			b.Background = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
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
			paint.ColorOp{Color: ruleColor}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			return layout.Dimensions{Size: size}
		})
	}
}

// button is a plain material button with a little space after it.
func button(th *material.Theme, c *widget.Clickable, label string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, material.Button(th, c, label).Layout)
	})
}

// label is a body label as a flex child.
func label(th *material.Theme, s string) layout.FlexChild {
	return layout.Rigid(material.Body2(th, s).Layout)
}
