package main

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows"
)

// openInEditor opens path with the application Windows associates with
// editing it. A .cfg file usually has no such association, so Notepad, which
// every Windows install carries, stands in for one.
func openInEditor(path string) error {
	verb, _ := windows.UTF16PtrFromString("edit")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err == nil {
		return nil
	}
	if err := exec.Command("notepad.exe", path).Start(); err != nil {
		return fmt.Errorf("no default editor is set for .cfg files and Notepad could not be started (%v); open %s in any text editor", err, path)
	}
	return nil
}
