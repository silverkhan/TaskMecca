//go:build windows

package rolepolicy

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
)

func openSnapshotPath(root, path string) (*os.File, error) {
	if e := validateSnapshotRelative(path); e != nil {
		return nil, e
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	expected, e := filepath.EvalSymlinks(full)
	if e != nil {
		return nil, e
	}
	if !strings.EqualFold(expected, full) {
		return nil, errors.New("source_symlink_or_nonregular")
	}
	p, e := windows.UTF16PtrFromString(full)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(h), full)
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	var data windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &data); e != nil || data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
		f.Close()
		return nil, errors.New("source_symlink_or_nonregular")
	}
	buf := make([]uint16, 32768)
	n, e := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	if e != nil || n >= uint32(len(buf)) {
		f.Close()
		return nil, errors.New("source_handle_path_unverified")
	}
	actual := windows.UTF16ToString(buf[:n])
	actual = strings.TrimPrefix(actual, `\\?\`)
	if strings.HasPrefix(actual, `UNC\`) {
		actual = `\\` + strings.TrimPrefix(actual, `UNC\`)
	}
	if !strings.EqualFold(filepath.Clean(actual), filepath.Clean(full)) {
		f.Close()
		return nil, errors.New("source_handle_path_changed")
	}
	return f, nil
}
