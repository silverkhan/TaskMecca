//go:build darwin

package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Darwin's renameatx_np(RENAME_EXCL) provides atomic no-replacement semantics
// for both the move and a rollback. SDK sys/stdio.h defines this flag as 0x4.
func renameCleanupNoReplace(fromFD int, from string, toFD int, to string) error {
	fromName, err := syscall.BytePtrFromString(from)
	if err != nil {
		return err
	}
	toName, err := syscall.BytePtrFromString(to)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(unix.SYS_RENAMEATX_NP, uintptr(fromFD), uintptr(unsafe.Pointer(fromName)), uintptr(toFD), uintptr(unsafe.Pointer(toName)), 0x4, 0)
	runtime.KeepAlive(fromName)
	runtime.KeepAlive(toName)
	if errno != 0 {
		return errno
	}
	return nil
}

// Open every parent component without following symlinks. Renameat stays bound
// to the inspected directories even if an ancestor path is replaced later.
func openCleanupParent(path string) (int, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	return fd, nil
}

func moveProjectFolder(source, destination string, expected os.FileInfo) error {
	return moveProjectFolderBeforeRename(source, destination, expected, nil)
}

func moveProjectFolderBeforeRename(source, destination string, expected os.FileInfo, before func()) error {
	sourceParent, err := openCleanupParent(filepath.Dir(source))
	if err != nil {
		return fmt.Errorf("source parent changed; folder preserved: %w", err)
	}
	defer unix.Close(sourceParent)
	trashParent, err := openCleanupParent(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("Trash parent changed; folder preserved: %w", err)
	}
	defer unix.Close(trashParent)
	sourceName, destName := filepath.Base(source), filepath.Base(destination)
	original, ok := expected.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("folder identity unavailable; folder preserved")
	}
	matches := func(stat *unix.Stat_t) bool {
		return uint64(stat.Dev) == uint64(original.Dev) && stat.Ino == original.Ino && stat.Mode&unix.S_IFMT == unix.S_IFDIR
	}
	var current unix.Stat_t
	if err := unix.Fstatat(sourceParent, sourceName, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil || !matches(&current) {
		return fmt.Errorf("project path changed; folder preserved")
	}
	// The destination is inside our newly-created private directory. Refuse an
	// unexpected item rather than replace it, including a dangling symlink.
	if err := unix.Fstatat(trashParent, destName, &current, unix.AT_SYMLINK_NOFOLLOW); err != unix.ENOENT {
		return fmt.Errorf("Trash destination changed; folder preserved")
	}
	if before != nil {
		before()
	}
	if err := renameCleanupNoReplace(sourceParent, sourceName, trashParent, destName); err != nil {
		return fmt.Errorf("recoverable folder cleanup unavailable: %w", err)
	}
	if err := unix.Fstatat(trashParent, destName, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil || !matches(&current) {
		if err := unix.Fstatat(sourceParent, sourceName, &current, unix.AT_SYMLINK_NOFOLLOW); err != unix.ENOENT {
			return fmt.Errorf("project changed during move; recovery required at %s", destination)
		}
		if err := renameCleanupNoReplace(trashParent, destName, sourceParent, sourceName); err != nil {
			return fmt.Errorf("project changed during move; recovery required at %s: %w", destination, err)
		}
		return fmt.Errorf("project changed during move; moved item restored")
	}
	return nil
}
