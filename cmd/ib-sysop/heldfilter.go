package main

import (
	"fmt"
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/andy5995/immortal-barons/internal/store"
)

// heldFilter narrows the held-packets table by reason: pick a reason, then
// hide it or show it alone. It only changes what the table shows; nothing on
// disk is touched.
type heldFilter struct {
	selected store.HeldReason
	hidden   map[store.HeldReason]bool
	chips    map[store.HeldReason]*widget.Clickable

	hideBtn, onlyBtn, allBtn widget.Clickable
}

// heldReasonCounts is each reason present in held, with how many packets carry
// it, in the order the reasons are first met.
func heldReasonCounts(held []store.HeldPacket) ([]store.HeldReason, map[store.HeldReason]int) {
	var order []store.HeldReason
	counts := map[store.HeldReason]int{}
	for _, h := range held {
		if counts[h.Reason] == 0 {
			order = append(order, h.Reason)
		}
		counts[h.Reason]++
	}
	return order, counts
}

// shown is held without the hidden reasons.
func (f *heldFilter) shown(held []store.HeldPacket) []store.HeldPacket {
	var out []store.HeldPacket
	for _, h := range held {
		if !f.hidden[h.Reason] {
			out = append(out, h)
		}
	}
	return out
}

// hide hides the selected reason.
func (f *heldFilter) hide() {
	if f.selected == "" {
		return
	}
	if f.hidden == nil {
		f.hidden = map[store.HeldReason]bool{}
	}
	f.hidden[f.selected] = true
}

// only hides every reason in present but the selected one.
func (f *heldFilter) only(present []store.HeldReason) {
	if f.selected == "" {
		return
	}
	f.hidden = map[store.HeldReason]bool{}
	for _, r := range present {
		if r != f.selected {
			f.hidden[r] = true
		}
	}
}

// layout draws the reason chips and the three actions. A chip names its reason
// and count, and says "hidden" in words, so the state never rests on color.
func (f *heldFilter) layout(gtx layout.Context, th *material.Theme, held []store.HeldPacket) layout.Dimensions {
	order, counts := heldReasonCounts(held)
	if len(order) == 0 {
		return layout.Dimensions{}
	}
	if f.chips == nil {
		f.chips = map[store.HeldReason]*widget.Clickable{}
	}
	for _, r := range order {
		if f.chips[r] == nil {
			f.chips[r] = new(widget.Clickable)
		}
		if f.chips[r].Clicked(gtx) {
			f.selected = r
		}
	}
	if !slices.Contains(order, f.selected) {
		f.selected = ""
	}
	if f.hideBtn.Clicked(gtx) {
		f.hide()
	}
	if f.onlyBtn.Clicked(gtx) {
		f.only(order)
	}
	if f.allBtn.Clicked(gtx) {
		f.hidden = nil
	}

	items := []layout.FlexChild{label(th, "Reason: ")}
	for _, r := range order {
		text := fmt.Sprintf("%s (%d)", r, counts[r])
		if f.hidden[r] {
			text += " hidden"
		}
		c, sel := f.chips[r], r == f.selected
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, tabButton(th, c, text, sel))
		}))
	}
	items = append(items, layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout))
	if f.selected != "" {
		items = append(items, button(th, &f.hideBtn, "Hide"), button(th, &f.onlyBtn, "Show only"))
	}
	if len(f.hidden) > 0 {
		items = append(items, button(th, &f.allBtn, "Show all"))
	}
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
	})
}
