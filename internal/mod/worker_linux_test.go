package mod

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A descriptor the BBS passed to the game, without close-on-exec, must not
// reach the worker.
func TestTheWorkerDoesNotInheritOtherDescriptors(t *testing.T) {
	lowerLimits(t)
	for range 4 {
		f, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
	}
	fd, err := syscall.Open(os.DevNull, syscall.O_RDONLY, 0) // inheritable
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)

	out := filepath.Join(t.TempDir(), "fds")
	script := filepath.Join(t.TempDir(), "list.sh")
	os.WriteFile(script, []byte("#!/bin/sh\nexec /bin/ls /proc/self/fd > "+out+"\n"), 0o755)
	list := func(start func() error) string {
		os.Remove(out)
		if err := start(); err != nil {
			t.Skip(err)
		}
		for range 100 {
			if b, err := os.ReadFile(out); err == nil && len(b) > 0 {
				return " " + strings.ReplaceAll(string(b), "\n", " ")
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("the listing never appeared")
		return ""
	}
	mark := " " + strconv.Itoa(fd) + " "
	// The control: started the plain way, the child holds it.
	if got := list(func() error { return exec.Command(script).Run() }); !strings.Contains(got, mark) {
		t.Skipf("the control child did not inherit descriptor %d: %q", fd, got)
	}
	old := executable
	executable = func() (string, error) { return script, nil }
	defer func() { executable = old }()
	var w *worker
	got := list(func() error {
		var err error
		w, err = startWorker()
		return err
	})
	w.close()
	if strings.Contains(got, mark) {
		t.Errorf("the worker holds descriptor %d: %q", fd, got)
	}
}
