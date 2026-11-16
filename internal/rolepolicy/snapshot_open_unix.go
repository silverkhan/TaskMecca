//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package rolepolicy

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

// Every component is opened relative to the previous pinned directory handle.
// NOFOLLOW prevents parent/final symlink escape; NONBLOCK prevents FIFO hangs.
func openSnapshotPath(root, path string) (*os.File, error) {
	if e := validateSnapshotRelative(path); e != nil {
		return nil, e
	}
	parts := strings.Split(filepath.ToSlash(filepath.Join(root, filepath.FromSlash(path))), "/")
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			unix.Close(fd)
			return nil, errors.New("source_path_invalid")
		}
		var st unix.Stat_t
		if err := unix.Fstatat(fd, part, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			unix.Close(fd)
			return nil, &os.PathError{Op: "stat source without links", Path: path, Err: err}
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG && st.Mode&unix.S_IFMT != unix.S_IFDIR {
			unix.Close(fd)
			return nil, errors.New("source_symlink_or_nonregular")
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, err := unix.Openat(fd, part, flags, 0)
		unix.Close(fd)
		if err != nil {
			return nil, &os.PathError{Op: "open source without links", Path: path, Err: err}
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), filepath.Join(root, path))
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("source_symlink_or_nonregular")
	}
	return f, nil
}
