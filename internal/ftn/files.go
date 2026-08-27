package ftn

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFile(path, data, mode, false)
}

func replaceFileAtomic(path string, data []byte, mode os.FileMode) error {
	return writeFile(path, data, mode, true)
}

func writeFile(path string, data []byte, mode os.FileMode, replace bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		if !replace {
			return fmt.Errorf("refusing to replace different file %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if !replace {
		return os.Rename(tmpPath, path)
	}
	return replaceRenameAtomic(tmpPath, path)
}
