//go:build windows

package ftn

import (
	"os"
	"syscall"
	"time"
)

// arrivedAt uses the creation time, which for a file a mailer wrote into the
// inbound is when it landed. os.Chtimes does not move it, so a delivered file
// carrying the sender's modification time is still seen as new.
func arrivedAt(info os.FileInfo) time.Time {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, d.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}
