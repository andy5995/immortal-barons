//go:build !windows

package ftn

import "os"

func replaceRenameAtomic(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
