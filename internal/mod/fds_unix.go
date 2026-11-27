//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package mod

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
)

// protectInheritedFDs marks every open descriptor from 3 up close-on-exec in
// the game, so the worker does not receive it. Go opens its own with the flag
// set, but one the BBS passed to the game (the caller's socket, say) may lack
// it, and a child started with exec.Cmd would inherit it. It changes nothing
// for the game: the descriptors stay open here. The worker cannot do this
// for itself, since the Go runtime owns some descriptors there.
func protectInheritedFDs() {
	// /proc/self/fd and macOS's /dev/fd list every open descriptor. FreeBSD's
	// /dev/fd lists only 0 to 2 unless fdescfs is mounted, so it is not used.
	dirs := []string{"/proc/self/fd"}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs, "/dev/fd")
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if n, err := strconv.Atoi(e.Name()); err == nil && n >= 3 {
				syscall.CloseOnExec(n)
			}
		}
		return
	}
	// No listing: try each number up to the limit, a bad one is just refused.
	top := fdTryMax
	var r syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &r) == nil && uint64(r.Cur) < fdTryMax {
		top = int(r.Cur)
	}
	for fd := 3; fd < top; fd++ {
		syscall.CloseOnExec(fd)
	}
}

// fdTryMax is the most descriptors tried without a listing. A BBS hands a door
// a low-numbered socket.
const fdTryMax = 65536
