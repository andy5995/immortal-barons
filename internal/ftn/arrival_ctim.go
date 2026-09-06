//go:build linux || openbsd

package ftn

import (
	"os"
	"syscall"
	"time"
)

func arrivedAt(info os.FileInfo) time.Time {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
	}
	return info.ModTime()
}
