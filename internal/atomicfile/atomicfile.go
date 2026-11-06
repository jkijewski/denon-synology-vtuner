// Package atomicfile writes files so that a crash or power cut leaves either the old
// content or the new content, never a partial or empty file.
package atomicfile

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Write replaces path with data. The data reaches the disk before the rename, and the
// rename reaches the disk before Write returns.
func Write(path string, data []byte, perm fs.FileMode) error {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
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
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
