package main

import (
	"testing"

	"gioui.org/widget/material"
)

// An unset or unknown saved theme means System, and System draws what the
// desktop said, light when it said nothing.
func TestThemeChoice(t *testing.T) {
	for in, want := range map[string]themeMode{"": themeSystem, "bogus": themeSystem,
		"light": themeLight, "dark": themeDark, "system": themeSystem} {
		if got := parseThemeMode(in); got != want {
			t.Errorf("parseThemeMode(%q) = %q, want %q", in, got, want)
		}
	}
	for _, c := range []struct {
		mode        themeMode
		desktopDark bool
		want        bool
	}{
		{themeLight, true, false},
		{themeDark, false, true},
		{themeSystem, true, true},
		{themeSystem, false, false},
	} {
		u := &ui{theme: c.mode, desktopDark: c.desktopDark, th: material.NewTheme()}
		u.applyTheme()
		if u.dark() != c.want || (pal == darkPalette) != c.want || u.th.Bg != pal.bg {
			t.Errorf("%s with the desktop dark=%v: dark=%v, want %v", c.mode, c.desktopDark, u.dark(), c.want)
		}
	}
	pal = lightPalette
}

// The choice survives a restart.
func TestThemeIsSaved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	u := &ui{zoom: defaultZoom, theme: themeDark}
	u.saveSession()
	if got := parseThemeMode(loadSession().Theme); got != themeDark {
		t.Errorf("saved theme read back as %q", got)
	}
}
