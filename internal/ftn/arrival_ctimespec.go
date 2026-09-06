//go:build darwin || freebsd || netbsd || dragonfly

package ftn

import (
	"os"
	"syscall"
	"time"
)

func arrivedAt(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Ctimespec.Sec, st.Ctimespec.Nsec)
	}
	return info.ModTime()
}
