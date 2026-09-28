package main

import (
	"image/color"

	"gioui.org/widget/material"
)

// themeMode is the Light / Dark / System choice, saved with the session.
type themeMode string

const (
	themeSystem themeMode = "system" // follow the desktop; light when it says nothing
	themeLight  themeMode = "light"
	themeDark   themeMode = "dark"
)

var themeModes = [...]struct {
	mode  themeMode
	label string
}{{themeLight, "Light"}, {themeDark, "Dark"}, {themeSystem, "System"}}

// palette is every color the panel draws with. Contrast, measured against the
// palette's own bg (WCAG 2.1):
//
//	         light   dark
//	fg       21.0    13.4
//	err       7.2     6.7
//	dim       6.3     7.0
//	border    3.5     3.9   input boxes (3:1 for a UI component)
//	fg/tabBg 17.1    10.1   an unselected tab's label on its fill
//	contrast  6.9     8.5   contrastFg on contrastBg: the selected tab, buttons
//	                         contrastBg on bg: 6.9 / 6.7, the checkbox mark
//
// rule is a decorative separator and is left faint on purpose.
type palette struct {
	bg, fg, contrastBg, contrastFg color.NRGBA
	err, dim, rule, border, tabBg  color.NRGBA
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

var (
	lightPalette = palette{
		bg: rgb(0xffffff), fg: rgb(0x000000),
		contrastBg: rgb(0x3f51b5), contrastFg: rgb(0xffffff),
		err: rgb(0xb01010), dim: rgb(0x606060), rule: rgb(0xc8c8c8),
		border: rgb(0x8a8a8a), tabBg: rgb(0xe8e8e8),
	}
	darkPalette = palette{
		bg: rgb(0x1e1e1e), fg: rgb(0xe6e6e6),
		contrastBg: rgb(0x8c9eff), contrastFg: rgb(0x000000),
		err: rgb(0xff7b7b), dim: rgb(0xa8a8a8), rule: rgb(0x4a4a4a),
		border: rgb(0x7a7a7a), tabBg: rgb(0x333333),
	}
)

// pal is the palette in use. Only the UI goroutine reads or sets it.
var pal = lightPalette

func parseThemeMode(s string) themeMode {
	switch m := themeMode(s); m {
	case themeLight, themeDark:
		return m
	}
	return themeSystem
}

// dark reports whether the panel should draw dark now.
func (u *ui) dark() bool {
	return u.theme == themeDark || (u.theme == themeSystem && u.desktopDark)
}

// applyTheme puts the palette the current choice calls for into use.
func (u *ui) applyTheme() {
	pal = lightPalette
	if u.dark() {
		pal = darkPalette
	}
	u.th.Palette = material.Palette{Bg: pal.bg, Fg: pal.fg, ContrastBg: pal.contrastBg, ContrastFg: pal.contrastFg}
}

// setTheme is a click on Light, Dark or System.
func (u *ui) setTheme(m themeMode) {
	if m == u.theme {
		return
	}
	u.theme = m
	u.applyTheme()
	u.saveSession()
	if m == themeSystem {
		go u.detectDesktopTheme()
	}
}

// detectDesktopTheme asks the desktop whether it is dark, off the UI goroutine
// since the answer can take a moment (a D-Bus call on Linux). It only matters
// while System is chosen, and is asked again now and then so a desktop that
// switches at dusk is followed.
func (u *ui) detectDesktopTheme() {
	dark, _ := desktopPrefersDark()
	u.mu.Lock()
	changed := dark != u.desktopDark
	u.desktopDark = dark
	if changed {
		u.applyTheme()
	}
	u.mu.Unlock()
	if changed {
		u.win.Invalidate()
	}
}
