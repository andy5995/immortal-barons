//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package mod

import "os"

func openModFile(path string) (*os.File, error) { return os.Open(path) }

// linkCount: these systems do not report a link count in a form this reads.
func linkCount(*os.File, os.FileInfo) uint64 { return 1 }
