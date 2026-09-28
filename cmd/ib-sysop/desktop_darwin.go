package main

import (
	"os/exec"
	"strings"
)

// desktopPrefersDark reads macOS's appearance: AppleInterfaceStyle is "Dark"
// in dark mode and absent in light mode, when defaults exits non-zero.
func desktopPrefersDark() (dark, ok bool) {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return false, true
	}
	return strings.TrimSpace(string(out)) == "Dark", true
}
