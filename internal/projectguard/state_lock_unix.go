//go:build darwin || linux || freebsd || openbsd || netbsd

package projectguard

import (
	"os"
	"syscall"
)

func lockStateFile(file *os.File) (func(), error) {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, nil
}

// Reject a substituted terminal symlink atomically at open on supported Unix.
func openStateFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
