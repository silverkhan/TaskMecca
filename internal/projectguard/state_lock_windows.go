//go:build windows

package projectguard

import (
	"golang.org/x/sys/windows"
	"os"
)

func lockStateFile(file *os.File) (func(), error) {
	overlapped := new(windows.Overlapped)
	handle := windows.Handle(file.Fd())
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped) }, nil
}

// The portable path performs opened inode/path checks after acquisition.
func openStateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
}
