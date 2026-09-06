//go:build !linux && !openbsd && !darwin && !freebsd && !netbsd && !dragonfly

package ftn

import (
	"os"
	"time"
)

// arrivedAt falls back to the modification time where the change time is not
// reachable. That is the pre-#236 behaviour and it can report a freshly
// delivered file as old, so a board on such a platform reads the threshold as
// the sender's file age.
func arrivedAt(info os.FileInfo) time.Time {
	return info.ModTime()
}
