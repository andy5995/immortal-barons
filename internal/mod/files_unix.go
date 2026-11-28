//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package mod

import (
	"os"
	"syscall"
)

// openModFile opens a mod's file without following a link at the last
// element, and without waiting on a pipe someone swapped in after the check.
func openModFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// linkCount is how many names the file has. A mod's file with more than one is
// refused: a hard link to a file elsewhere reads like a file in the folder, and
// a league's reset order would carry its contents to every board.
func linkCount(_ *os.File, fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}
