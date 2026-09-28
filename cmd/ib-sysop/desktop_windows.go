package main

import "golang.org/x/sys/windows/registry"

// desktopPrefersDark reads the "Choose your app mode" setting: AppsUseLightTheme
// is 0 when apps should be dark. ok is false on a Windows too old to have it.
func desktopPrefersDark() (dark, ok bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false, false
	}
	return v == 0, true
}
