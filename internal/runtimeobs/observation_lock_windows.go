//go:build windows

package runtimeobs

import (
	"os"
	"path/filepath"
	"golang.org/x/sys/windows"
)

func lockObservationControl() (func(), error) {
	path, err := observationControlPath()
	if err != nil { return nil, err }
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { return nil, err }
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil { return nil, err }
	overlapped := new(windows.Overlapped)
	handle := windows.Handle(file.Fd())
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil { file.Close(); return nil, err }
	return func() { _ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped); _ = file.Close() }, nil
}
