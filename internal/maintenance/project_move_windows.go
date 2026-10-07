//go:build windows

package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validateTrashPlatform(source string) error {
	volume := filepath.VolumeName(source)
	if len(volume) != 2 || volume[1] != ':' || strings.HasPrefix(source, `\\`) {
		return fmt.Errorf("Windows Recycle Bin requires a local fixed drive; UNC/device paths are unsupported; folder preserved")
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return err
	}
	if windows.GetDriveType(root) != windows.DRIVE_FIXED {
		return fmt.Errorf("Windows Recycle Bin does not support this removable/network drive; folder preserved")
	}
	return nil
}

// Hold all ancestor directories without FILE_SHARE_DELETE so their names
// cannot be replaced while we rename the leaf by its open identity.
func lockCleanupParents(path string) ([]windows.Handle, error) {
	volume := filepath.VolumeName(path)
	current := volume + `\`
	paths := []string{current}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(path), current), `\`) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		paths = append(paths, current)
	}
	var handles []windows.Handle
	for _, item := range paths {
		name, err := windows.UTF16PtrFromString(item)
		if err != nil {
			closeCleanupHandles(handles)
			return nil, err
		}
		h, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			closeCleanupHandles(handles)
			return nil, err
		}
		var info windows.ByHandleFileInformation
		if err = windows.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			windows.CloseHandle(h)
			closeCleanupHandles(handles)
			return nil, fmt.Errorf("reparse-point/changed ancestor rejected; folder preserved")
		}
		handles = append(handles, h)
	}
	return handles, nil
}

func closeCleanupHandles(handles []windows.Handle) {
	for _, h := range handles {
		_ = windows.CloseHandle(h)
	}
}

func moveProjectFolder(source, destination string, expected os.FileInfo) error {
	return moveProjectFolderBeforeRename(source, destination, expected, nil)
}

func moveProjectFolderBeforeRename(source, destination string, expected os.FileInfo, before func()) error {
	sourceParents, err := lockCleanupParents(filepath.Dir(source))
	if err != nil {
		return fmt.Errorf("source parent protected; folder preserved: %w", err)
	}
	defer closeCleanupHandles(sourceParents)
	destinationParents, err := lockCleanupParents(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("staging parent protected; folder preserved: %w", err)
	}
	defer closeCleanupHandles(destinationParents)
	name, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.DELETE|windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Errorf("folder locked or inaccessible; preserved: %w", err)
	}
	f := os.NewFile(uintptr(h), source)
	defer f.Close()
	actual, err := f.Stat()
	var identity windows.ByHandleFileInformation
	if err != nil || windows.GetFileInformationByHandle(h, &identity) != nil || identity.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || !actual.IsDir() || !os.SameFile(expected, actual) {
		return fmt.Errorf("folder identity/reparse point changed; preserved")
	}
	if before != nil {
		before()
	}
	current, err := os.Lstat(source)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(actual, current) {
		return fmt.Errorf("folder path changed before move; preserved")
	}
	filename, err := windows.UTF16FromString(filepath.Base(destination))
	if err != nil {
		return err
	}
	filename = filename[:len(filename)-1]
	type renameInformation struct {
		Flags  uint32
		Root   windows.Handle
		Length uint32
		Name   [1]uint16
	}
	var header renameInformation
	size := int(unsafe.Offsetof(header.Name)) + len(filename)*2
	buffer := make([]byte, size)
	info := (*renameInformation)(unsafe.Pointer(&buffer[0]))
	info.Root = destinationParents[len(destinationParents)-1]
	info.Length = uint32(len(filename) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[unsafe.Offsetof(header.Name)])), len(filename)), filename)
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(h, &status, &buffer[0], uint32(size), windows.FileRenameInformation); err != nil {
		return fmt.Errorf("no-replace identity-bound move refused; folder preserved: %w", err)
	}
	return nil
}
