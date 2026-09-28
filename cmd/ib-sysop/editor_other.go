//go:build !windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openInEditor opens path in the desktop's default application for it. $EDITOR
// is not used: it usually names a terminal editor, and the panel has no
// terminal to run one in. The opener returns once the editor has started, so
// its exit status says whether one could be found.
func openInEditor(path string) error {
	name, args := "xdg-open", []string{path}
	if runtime.GOOS == "darwin" {
		// -t is the default TEXT editor: a .cfg file has no other association.
		name, args = "open", []string{"-t", path}
	}
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("no default editor: %s is not installed, so the desktop's default application cannot be found; open %s in any text editor", name, path)
	}
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		var exit *exec.ExitError
		if errors.As(err, &exit) && msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("no default editor could open it (%s: %s); open %s in any text editor", name, msg, path)
	}
	return nil
}
