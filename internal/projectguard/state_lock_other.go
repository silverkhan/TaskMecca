//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package projectguard

import (
	"errors"
	"os"
)

func lockStateFile(*os.File) (func(), error) {
	return nil, errors.New("project state locking unsupported on this OS")
}
