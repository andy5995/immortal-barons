//go:build linux || openbsd

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
		return time.Unix(int64(st.Ctim.Sec), int64(st.Ctim.Nsec))
	}
	return info.ModTime()
}
