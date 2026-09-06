//go:build darwin || freebsd || netbsd || dragonfly

package ftn

import (
	"os"
	"syscall"
	"time"
)

func arrivedAt(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		// The fields are int32 on a 32-bit build, so convert rather
		// than assume the width.
		return time.Unix(int64(st.Ctimespec.Sec), int64(st.Ctimespec.Nsec))
	}
	return info.ModTime()
}
