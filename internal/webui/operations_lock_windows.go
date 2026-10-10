//go:build windows

package webui

import (
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func lockOperationJournal(project string) (func(), error) {
	releaseGuard,guardErr:=projectguard.AcquireWrite(project)
	if guardErr!=nil{return nil,guardErr}
	defer releaseGuard()
	path := operationJournalPath(project) + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	overlapped := &windows.Overlapped{}
	handle := windows.Handle(file.Fd())
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		file.Close()
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped); _ = file.Close() }, nil
}
