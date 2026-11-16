//go:build windows

package rolepolicy

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"time"
)

func decisionReadLease(path string) (func(), error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_ALWAYS, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(h), path)
	var attributes windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &attributes); e != nil || attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		f.Close()
		return nil, errors.New("decision_lock_nonregular")
	}
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("decision_lock_nonregular")
	}
	o := &windows.Overlapped{}
	deadline := time.Now().Add(3 * time.Second)
	for {
		e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, o)
		if e == nil {
			return func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, o); _ = f.Close() }, nil
		}
		if !errors.Is(e, windows.ERROR_LOCK_VIOLATION) || time.Now().After(deadline) {
			f.Close()
			return nil, e
		}
		time.Sleep(10 * time.Millisecond)
	}
}
