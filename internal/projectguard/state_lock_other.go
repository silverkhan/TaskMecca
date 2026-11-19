//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package projectguard

import (
	"errors"
	"os"
)

func lockStateFile(*os.File) (func(), error) {
	return nil, errors.New("project state locking unsupported on this OS")
}

// The portable path performs opened inode/path checks after acquisition.
func openStateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
}
