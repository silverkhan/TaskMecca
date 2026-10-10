//go:build windows

package notify

import (
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func acquireDeliveryLock(project string) (func(), error) {
	releaseGuard,guardErr:=projectguard.AcquireWrite(project)
	if guardErr!=nil{return nil,guardErr}
	defer releaseGuard()
	path := ledgerPath(project) + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	overlapped := new(windows.Overlapped)
	handle := windows.Handle(file.Fd())
	if err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		file.Close()
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped); _ = file.Close() }, nil
}
