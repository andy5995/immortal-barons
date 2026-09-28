//go:build !windows && !darwin

package main

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var portalValue = regexp.MustCompile(`uint32 (\d)`)

// desktopPrefersDark asks, in order, the places a Linux or BSD desktop records
// a dark preference, and returns the first answer. ok is false when none says.
//
//   - the freedesktop settings portal (GNOME, KDE and most others): 1 dark,
//     2 light, 0 no preference, which falls through to the rest
//   - GNOME's color-scheme setting
//   - the GTK theme's name, then XFCE's, which is how XFCE and older desktops
//     mark a dark theme ("Adwaita-dark", "Greybird-dark")
func desktopPrefersDark() (dark, ok bool) {
	if out, err := run("gdbus", "call", "--session",
		"--dest", "org.freedesktop.portal.Desktop",
		"--object-path", "/org/freedesktop/portal/desktop",
		"--method", "org.freedesktop.portal.Settings.ReadOne",
		"org.freedesktop.appearance", "color-scheme"); err == nil {
		if m := portalValue.FindStringSubmatch(out); m != nil {
			switch m[1] {
			case "1":
				return true, true
			case "2":
				return false, true
			}
		}
	}
	if out, err := run("gsettings", "get", "org.gnome.desktop.interface", "color-scheme"); err == nil {
		switch strings.Trim(strings.TrimSpace(out), "'") {
		case "prefer-dark":
			return true, true
		case "prefer-light":
			return false, true
		}
	}
	for _, cmd := range [][]string{
		{"gsettings", "get", "org.gnome.desktop.interface", "gtk-theme"},
		{"xfconf-query", "-c", "xsettings", "-p", "/Net/ThemeName"},
	} {
		if out, err := run(cmd[0], cmd[1:]...); err == nil && strings.TrimSpace(out) != "" {
			if strings.Contains(strings.ToLower(out), "dark") {
				return true, true
			}
		}
	}
	return false, false
}

// run is one short query, abandoned after two seconds: a session bus that is
// not answering must not hold the theme up.
func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}
