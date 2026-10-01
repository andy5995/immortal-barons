package main

import (
	"sort"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// table is a sortable, fixed-width text table. Clicking a heading sorts by that
// column; clicking it again reverses the order.
type table struct {
	heads   []string
	click   []widget.Clickable
	copyBtn widget.Clickable
	sort    int
	desc    bool
	list    widget.List
	wide    widget.List // scrolls a table wider than the window sideways
}

func newTable(heads ...string) *table {
	t := &table{heads: heads, click: make([]widget.Clickable, len(heads)), sort: -1}
	t.list.Axis = layout.Vertical
	t.wide.Axis = layout.Horizontal
	return t
}

// sortKey makes numbers compare as numbers ("10" after "9") and leaves the
// rest to compare as text.
func sortKey(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.Fields(s + " x")[0], 64)
	return f, err == nil
}

func less(a, b string) bool {
	fa, oka := sortKey(a)
	fb, okb := sortKey(b)
	if oka && okb && fa != fb {
		return fa < fb
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

func (t *table) layout(gtx layout.Context, th *material.Theme, rows [][]string, empty string) layout.Dimensions {
	for i := range t.click {
		if t.click[i].Clicked(gtx) {
			if t.sort == i {
				t.desc = !t.desc
			} else {
				t.sort, t.desc = i, false
			}
		}
	}
	if t.sort >= 0 {
		rows = append([][]string(nil), rows...)
		sort.SliceStable(rows, func(i, j int) bool {
			if t.desc {
				return less(rows[j][t.sort], rows[i][t.sort])
			}
			return less(rows[i][t.sort], rows[j][t.sort])
		})
	}

	// Copy takes the rows as shown, sorted the same way, as tab-separated text
	// with the headings first: it pastes into a message or a spreadsheet.
	if t.copyBtn.Clicked(gtx) {
		copyText(gtx, tsv(t.heads, rows))
	}

	// Each column is as wide as its longest cell, in characters of the mono face.
	widths := make([]int, len(t.heads))
	for i, h := range t.heads {
		widths[i] = len([]rune(h)) + 2
	}
	for _, r := range rows {
		for i, c := range r {
			widths[i] = max(widths[i], len([]rune(c)))
		}
	}
	pad := func(s string, w int) string {
		if n := w - len([]rune(s)); n > 0 {
			return s + strings.Repeat(" ", n)
		}
		return s
	}

	head := func(gtx layout.Context) layout.Dimensions {
		var cells []layout.FlexChild
		for i, h := range t.heads {
			i, h := i, h
			if i == t.sort {
				h += map[bool]string{false: " ▲", true: " ▼"}[t.desc]
			}
			cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &t.click[i], func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, pad(h, widths[i])+"  ")
					l.Font.Typeface, l.Font.Weight = mono, 700
					l.MaxLines = 1
					return l.Layout(gtx)
				})
			}))
		}
		return layout.Flex{}.Layout(gtx, cells...)
	}

	body := func(gtx layout.Context) layout.Dimensions {
		return t.body(gtx, th, head, rows, widths, pad, empty)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(rows) == 0 {
				gtx = gtx.Disabled()
			}
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx, button(th, &t.copyBtn, "Copy"))
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &t.wide).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return body(gtx)
			})
		}),
	)
}

// tsv is a table as tab-separated text, the headings as the first line.
func tsv(heads []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(strings.Join(heads, "\t") + "\n")
	for _, r := range rows {
		b.WriteString(strings.Join(r, "\t") + "\n")
	}
	return b.String()
}

func (t *table) body(gtx layout.Context, th *material.Theme, head layout.Widget, rows [][]string, widths []int,
	pad func(string, int) string, empty string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(head),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(rows) == 0 {
				return material.Body2(th, empty).Layout(gtx)
			}
			return material.List(th, &t.list).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
				var b strings.Builder
				for j, c := range rows[i] {
					b.WriteString(pad(c, widths[j]))
					b.WriteString("  ")
				}
				l := material.Body2(th, b.String())
				l.Font.Typeface = mono
				l.MaxLines = 1
				return l.Layout(gtx)
			})
		}),
	)
}
